package core

// Only the private service-bootstrap DNS role uses this transport. The SDK
// client/router/codecs and protected user-DNS transport keep their own roles.

import (
	"context"
	"errors"
	"reflect"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/dialer"
	sbdns "github.com/sagernet/sing-box/dns"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"

	"github.com/snowden-system/windows/backend/config"
)

const maxBootstrapExchanges = 2 // original A and AAAA calls, including uncertain retirement

type bootstrapDNSRegistry struct {
	adapter.DNSTransportRegistry
	worker *bootstrapWorker
}

// All mutable transport/query/retirement metadata is guarded by worker.mu.
// That value lock never covers native I/O or a join.
type bootstrapDNSTransport struct {
	sbdns.TransportAdapter
	worker     *bootstrapWorker
	dialer     N.Dialer
	server     M.Socksaddr
	started    bool
	closed     bool
	exchanges  map[*bootstrapExchange]struct{}
	retirement *bootstrapDNSRetirement
}

type bootstrapDNSRetirement struct {
	done chan struct{}
	err  error // immutable once done closes; error retains the original retirement
}

func (registry *bootstrapDNSRegistry) CreateDNSTransport(ctx context.Context, logger log.ContextLogger, tag, kind string, raw any) (adapter.DNSTransport, error) {
	if tag != config.BootstrapDNSTag {
		return registry.DNSTransportRegistry.CreateDNSTransport(ctx, logger, tag, kind, raw)
	}
	worker := registry.worker
	options, ok := raw.(*option.RemoteDNSServerOptions)
	if worker == nil || worker.dnsServer == nil || kind != "udp" || !ok || options == nil {
		return nil, ErrPreflightScope
	}
	expected := option.RemoteDNSServerOptions{}
	expected.Server, expected.ServerPort, expected.Detour = worker.dnsServer.Server, worker.dnsServer.ServerPort, worker.dnsDetour
	if !reflect.DeepEqual(*options, expected) {
		return nil, ErrPreflightScope // no legacy/local/default-outbound or extra dial options
	}
	transport := &bootstrapDNSTransport{TransportAdapter: sbdns.NewTransportAdapterWithRemoteOptions(kind, tag, expected),
		worker: worker, server: expected.Build(), exchanges: make(map[*bootstrapExchange]struct{})}
	worker.mu.Lock()
	if worker.transport != nil || worker.closed || worker.fault != nil {
		worker.mu.Unlock()
		return nil, ErrPreflightScope
	}
	worker.transport = transport // original owner before SDK dialer construction
	worker.mu.Unlock()
	instance, err := sbdns.NewRemoteDialer(ctx, expected)
	if err != nil || instance == nil {
		worker.poison(ErrPreflightCleanup)
		return nil, errors.Join(ErrPreflightCleanup, err)
	}
	worker.mu.Lock()
	transport.dialer = instance
	worker.mu.Unlock()
	return transport, nil
}

func (transport *bootstrapDNSTransport) Start(stage adapter.StartStage) error {
	if stage != adapter.StartStateStart {
		return nil
	}
	worker := transport.worker
	worker.mu.Lock()
	valid := worker.transport == transport && !worker.closed && worker.fault == nil && !transport.closed && transport.dialer != nil
	worker.mu.Unlock()
	if !valid {
		return ErrPreflightScope
	}
	if err := dialer.InitializeDetour(transport.dialer); err != nil {
		return err // same pinned SDK direct/custom detour initialization, never a fallback
	}
	worker.mu.Lock()
	defer worker.mu.Unlock()
	if worker.closed || worker.fault != nil || transport.closed {
		return ErrPreflightScope
	}
	transport.started = true
	return nil
}

func (transport *bootstrapDNSTransport) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), workerCloseTimeout)
	defer cancel()
	return transport.retire(ctx, true)
}

// No persistent socket or receive goroutine to adopt after Reset. Existing
// calls still have to return and confirm their own connection Close.
func (transport *bootstrapDNSTransport) Reset() {
	ctx, cancel := context.WithTimeout(context.Background(), workerCloseTimeout)
	defer cancel()
	if err := transport.retire(ctx, false); err != nil {
		transport.worker.poison(err) // the SDK's void Reset cannot carry a positive receipt
	}
}

func (worker *bootstrapWorker) retireDNSQueries(ctx context.Context, closing bool) error {
	worker.mu.Lock()
	transport := worker.transport
	worker.mu.Unlock()
	if transport == nil {
		return nil // IP-only config has no bootstrap transport
	}
	return transport.retire(ctx, closing)
}

func (transport *bootstrapDNSTransport) retire(ctx context.Context, closing bool) error {
	worker := transport.worker
	for {
		if err := contextError(ctx); err != nil {
			return errors.Join(ErrPreflightCleanup, err)
		}
		worker.mu.Lock()
		if closing {
			transport.closed = true // before joining another original reset/retirement
		}
		if previous := transport.retirement; previous != nil {
			worker.mu.Unlock()
			select {
			case <-previous.done:
				if previous.err != nil {
					return previous.err
				}
			case <-ctx.Done():
				return errors.Join(ErrPreflightCleanup, ctx.Err())
			}
			// A Close/pause can follow an earlier Reset's snapshot. Re-scan
			// under the already closed/paused gate rather than borrow its ACK.
			continue
		}
		retirement := &bootstrapDNSRetirement{done: make(chan struct{})}
		transport.retirement = retirement
		queries := make([]*bootstrapExchange, 0, len(transport.exchanges))
		for query := range transport.exchanges {
			queries = append(queries, query)
		}
		worker.mu.Unlock()
		return transport.finishRetirement(ctx, retirement, queries)
	}
}

// One retained retirement owner also covers panic/Goexit in a native Close.
// Timeout or an early returned error never erases original query ownership.
func (transport *bootstrapDNSTransport) finishRetirement(ctx context.Context, retirement *bootstrapDNSRetirement, queries []*bootstrapExchange) (result error) {
	worker := transport.worker
	completed := false
	defer func() {
		if !completed {
			_ = recover()
			result = errors.Join(ErrPreflightCleanup, result)
		}
		worker.mu.Lock()
		if transport.retirement != retirement {
			result = errors.Join(ErrPreflightCleanup, result)
		}
		retirement.err = result
		if result == nil {
			transport.retirement = nil
		} else {
			if worker.fault == nil {
				worker.fault = errors.Join(ErrPreflightCleanup, result)
			}
			worker.admission.open.Store(false)
		}
		close(retirement.done)
		worker.mu.Unlock()
		if result != nil {
			worker.poison(result) // original queries and failed retirement remain owned
		}
		result = errors.Join(result, worker.failure())
	}()
	for _, query := range queries {
		query.cancel()
	}
	for _, query := range queries {
		_ = query.owner.close() // acquisition may still be returning; inspect its final receipt below
	}
	for _, query := range queries {
		select {
		case <-query.done:
			result = errors.Join(result, query.cleanupErr)
		case <-ctx.Done():
			result = errors.Join(result, ErrPreflightCleanup, ctx.Err())
		}
	}
	completed = true
	return result
}

var (
	_ adapter.DNSTransportRegistry = (*bootstrapDNSRegistry)(nil)
	_ adapter.DNSTransport         = (*bootstrapDNSTransport)(nil)
)
