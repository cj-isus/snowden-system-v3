package repository

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/netip"
	"os"
	"os/signal"
	"strconv"
	"time"
)

// servePublisher is reached only by an explicit repository serve command.
// It never binds a public address, discovers a port, changes a firewall or
// chooses TLS/Cloudflare/deployment policy on the owner's behalf.
func servePublisher(directory, address string) ([]byte, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, errors.New("repository: explicit literal loopback address:port required")
	}
	ip, err := netip.ParseAddr(host)
	number, portErr := strconv.Atoi(port)
	if err != nil || !ip.IsLoopback() || ip.Zone() != "" || portErr != nil || number < 1 || number > 65535 || strconv.Itoa(number) != port {
		return nil, errors.New("repository: literal loopback and explicit canonical port1..65535 required")
	}
	handler, err := NewPublisherHandler(directory)
	if err != nil {
		return nil, err
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	checkCtx, checkCancel := context.WithTimeout(ctx, 10*time.Second)
	_, _, err = handler.load(checkCtx)
	checkCancel()
	if err != nil {
		return nil, err // establish a valid local view before binding any socket
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return nil, err
	}
	defer listener.Close()
	server := &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 8 << 10}
	finished := make(chan error, 1)
	go func() { finished <- server.Serve(listener) }()
	select {
	case err = <-finished:
		_ = server.Close()
		if err == nil || errors.Is(err, http.ErrServerClosed) {
			return nil, errors.New("repository: local server stopped without owner cancellation")
		}
		return nil, err
	case <-ctx.Done():
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		shutdownErr := server.Shutdown(shutdownCtx)
		shutdownCancel()
		if shutdownErr != nil {
			shutdownErr = errors.Join(shutdownErr, server.Close())
		}
		serveErr := <-finished
		if errors.Is(serveErr, http.ErrServerClosed) {
			serveErr = nil
		}
		if err = errors.Join(shutdownErr, serveErr); err != nil {
			return nil, err
		}
	}
	return json.Marshal(struct {
		Schema               int    `json:"schema"`
		Operation            string `json:"operation"`
		ExternalReachability string `json:"external_reachability"`
	}{1, "local_repository_server_stopped", "not_observed"})
}
