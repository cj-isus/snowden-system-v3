package hysteria2

import (
	"context"
	"net"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/sagernet/quic-go"
	"github.com/sagernet/quic-go/congestion"
	"github.com/sagernet/quic-go/http3"
	qtls "github.com/sagernet/sing-quic"
	congestion_meta1 "github.com/sagernet/sing-quic/congestion_meta1"
	congestion_meta2 "github.com/sagernet/sing-quic/congestion_meta2"
	"github.com/sagernet/sing-quic/hysteria"
	hyCC "github.com/sagernet/sing-quic/hysteria/congestion"
	"github.com/sagernet/sing-quic/hysteria2/internal/protocol"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/common/ntp"
	aTLS "github.com/sagernet/sing/common/tls"
)

const handshakeTimeout = 15 * time.Second

type ClientOptions struct {
	Context            context.Context
	Dialer             N.Dialer
	Logger             logger.Logger
	BrutalDebug        bool
	ServerAddress      M.Socksaddr
	ServerPorts        []string
	HopInterval        time.Duration
	SendBPS            uint64
	ReceiveBPS         uint64
	SalamanderPassword string
	Password           string
	TLSConfig          aTLS.Config
	UDPDisabled        bool
}

type Client struct {
	ctx                context.Context
	dialer             N.Dialer
	logger             logger.Logger
	brutalDebug        bool
	serverAddr         M.Socksaddr
	serverPorts        []uint16
	hopInterval        time.Duration
	sendBPS            uint64
	receiveBPS         uint64
	salamanderPassword string
	password           string
	tlsConfig          aTLS.Config
	quicConfig         *quic.Config
	udpDisabled        bool

	connAccess sync.Mutex
	conn       *clientQUICConnection
	pending    *clientOffer
	retiring   bool // reset admission seals under connAccess before external cleanup
}

func NewClient(options ClientOptions) (*Client, error) {
	if options.TLSConfig == nil {
		return nil, E.New("missing HY2 TLS configuration")
	}
	if options.Context == nil {
		options.Context = context.Background()
	}
	quicConfig := &quic.Config{
		DisablePathMTUDiscovery:        !(runtime.GOOS == "windows" || runtime.GOOS == "linux" || runtime.GOOS == "android" || runtime.GOOS == "darwin"),
		EnableDatagrams:                !options.UDPDisabled,
		InitialStreamReceiveWindow:     hysteria.DefaultStreamReceiveWindow,
		MaxStreamReceiveWindow:         hysteria.DefaultStreamReceiveWindow,
		InitialConnectionReceiveWindow: hysteria.DefaultConnReceiveWindow,
		MaxConnectionReceiveWindow:     hysteria.DefaultConnReceiveWindow,
		MaxIdleTimeout:                 hysteria.DefaultMaxIdleTimeout,
		KeepAlivePeriod:                hysteria.DefaultKeepAlivePeriod,
	}
	if len(options.TLSConfig.NextProtos()) == 0 {
		options.TLSConfig.SetNextProtos([]string{http3.NextProtoH3})
	}
	var serverPorts []uint16
	if len(options.ServerPorts) > 0 {
		var err error
		serverPorts, err = hysteria.ParsePorts(options.ServerPorts)
		if err != nil {
			return nil, err
		}
	}
	return &Client{
		ctx:                options.Context,
		dialer:             options.Dialer,
		logger:             options.Logger,
		brutalDebug:        options.BrutalDebug,
		serverAddr:         options.ServerAddress,
		serverPorts:        serverPorts,
		hopInterval:        options.HopInterval,
		sendBPS:            options.SendBPS,
		receiveBPS:         options.ReceiveBPS,
		salamanderPassword: options.SalamanderPassword,
		password:           options.Password,
		tlsConfig:          options.TLSConfig,
		quicConfig:         quicConfig,
		udpDisabled:        options.UDPDisabled,
	}, nil
}

func (c *Client) offer(ctx context.Context) (*clientQUICConnection, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := c.ctx.Err(); err != nil {
		return nil, err
	}
	c.connAccess.Lock()
	conn := c.conn
	if conn != nil {
		if !c.retiring && conn.active() {
			c.connAccess.Unlock()
			return conn, nil
		}
		select {
		case <-conn.retired:
			if conn.closeErr != nil {
				c.connAccess.Unlock()
				return nil, E.Errors(errClientUncertain, conn.closeErr)
			}
			c.conn = nil
			c.retiring = false
		default:
			c.connAccess.Unlock()
			conn.closeWithError(net.ErrClosed)
			return nil, errClientRetiring
		}
	}
	pending := c.pending
	if pending != nil {
		c.connAccess.Unlock()
		select {
		case <-pending.done:
			return pending.conn, pending.err
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	// A pending offer is shared by concurrent callers. Do not derive offerCtx
	// from the foreground request ctx: a timed-out request must stop waiting for
	// the shared result, but it must not tear down the background QUIC dial that
	// may still be reused by later requests. The connection attempt is owned by
	// the client lifetime context instead.
	owner := newClientQUICConnection(c.ctx)
	offerCtx, cancelCause := common.ContextWithCancelCause(owner.ctx)
	offerCtx, cancelTimeout := context.WithTimeout(offerCtx, handshakeTimeout)
	pending = &clientOffer{
		done: make(chan struct{}),
		cancel: func(cause error) {
			cancelCause(cause)
			cancelTimeout()
		},
		owner: owner,
	}
	c.pending = pending
	c.connAccess.Unlock()

	owner.runTask(func() {
		select {
		case <-owner.initialDone:
			return
		case <-offerCtx.Done():
			owner.requestClose(context.Cause(offerCtx), true)
		}
	})
	go c.completeOffer(pending, offerCtx)

	select {
	case <-pending.done:
		return pending.conn, pending.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (c *Client) offerNew(ctx context.Context, conn *clientQUICConnection) error {
	// Refuse an unsupported TLS provider before creating a native socket. Keep
	// each offer's owned clone/ALPN and the exact custom-vs-STD choice; no downgrade.
	tlsConfig := c.tlsConfig.Clone()
	if tlsConfig == nil {
		return E.New("missing HY2 TLS configuration clone")
	}
	tlsConfig.SetNextProtos([]string{http3.NextProtoH3})
	var dialQUIC func(net.PacketConn) (*quic.Conn, error)
	if custom, isCustom := tlsConfig.(qtls.Config); isCustom {
		dialQUIC = func(packetConn net.PacketConn) (*quic.Conn, error) {
			return custom.DialEarly(ctx, packetConn, c.serverAddr.UDPAddr(), c.quicConfig)
		}
	} else {
		stdConfig, configErr := tlsConfig.STDConfig()
		if configErr != nil {
			return configErr
		}
		if stdConfig == nil {
			return E.New("missing HY2 TLS configuration")
		}
		dialQUIC = func(packetConn net.PacketConn) (*quic.Conn, error) {
			transport := &quic.Transport{Conn: packetConn}
			conn.attachTransport(transport)
			return transport.DialEarly(ctx, c.serverAddr.UDPAddr(), stdConfig, c.quicConfig)
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	dialCtx := ctx
	hopCtx := conn.ctx
	firstDial := true
	dialFunc := func(serverAddr M.Socksaddr) (net.PacketConn, error) {
		currentCtx := hopCtx
		if firstDial {
			// The initial socket open belongs to the shared offer. Later port hops
			// belong to the live client connection and must outlive any one caller.
			currentCtx = dialCtx
			firstDial = false
		}
		return c.dialPacket(currentCtx, conn, serverAddr)
	}
	var (
		packetConn net.PacketConn
		err        error
	)
	if len(c.serverPorts) == 0 {
		packetConn, err = dialFunc(c.serverAddr)
	} else {
		packetConn, err = hysteria.NewHopPacketConn(dialFunc, c.serverAddr, c.serverPorts, c.hopInterval)
	}
	if packetConn != nil {
		conn.attachRaw(packetConn)
	}
	if err != nil {
		return err
	}
	if packetConn == nil {
		return errClientNoSocket
	}
	// One explicit dial, with the raw abort owner published before it starts.
	// HTTP/3 Transport.RoundTrip otherwise hides asynchronous dial / retry work
	// behind a pointer that may still change after RoundTrip cancellation.
	quicConn, err := dialQUIC(packetConn)
	if quicConn != nil {
		conn.attachQUIC(quicConn)
	}
	if err != nil {
		return qtls.WrapError(err)
	}
	if quicConn == nil {
		return errClientNoSocket
	}
	http3Client := (&http3.Transport{MaxResponseHeaderBytes: 64 * 1024, DisableCompression: true}).NewClientConn(quicConn)
	conn.attachHTTP3(http3Client)
	request := &http.Request{
		Method: http.MethodPost,
		URL: &url.URL{
			Scheme: "https",
			Host:   protocol.URLHost,
			Path:   protocol.URLPath,
		},
		Header: make(http.Header),
	}
	protocol.AuthRequestToHeader(request.Header, protocol.AuthRequest{Auth: c.password, Rx: c.receiveBPS})
	response, err := http3Client.RoundTrip(request.WithContext(ctx))
	if err != nil {
		if response != nil && response.Body != nil {
			_ = response.Body.Close()
		}
		return err
	}
	if err := response.Body.Close(); err != nil {
		return err
	}
	if response.StatusCode != protocol.StatusAuthOK {
		return E.New("authentication failed, status code: ", response.StatusCode)
	}
	authResponse := protocol.AuthResponseFromHeader(response.Header)
	actualTx := authResponse.Rx
	if actualTx == 0 || actualTx > c.sendBPS {
		actualTx = c.sendBPS
	}
	if !authResponse.RxAuto && actualTx > 0 {
		quicConn.SetCongestionControl(hyCC.NewBrutalSender(actualTx, c.brutalDebug, c.logger))
	} else {
		timeFunc := ntp.TimeFuncFromContext(c.ctx)
		if timeFunc == nil {
			timeFunc = time.Now
		}
		quicConn.SetCongestionControl(congestion_meta2.NewBbrSender(
			congestion_meta2.DefaultClock{TimeFunc: timeFunc},
			congestion.ByteCount(quicConn.Config().InitialPacketSize),
			congestion.ByteCount(congestion_meta1.InitialCongestionWindow),
		))
	}
	conn.udpDisabled = !authResponse.UDPEnabled
	if !c.udpDisabled {
		conn.runTask(func() { c.loopMessages(conn) })
	}
	return nil
}

func (c *Client) DialConn(ctx context.Context, destination M.Socksaddr) (net.Conn, error) {
	conn, err := c.offer(ctx)
	if err != nil {
		return nil, err
	}
	return conn.dialTCP(ctx, destination)
}

func (c *Client) ListenPacket(ctx context.Context) (net.PacketConn, error) {
	if c.udpDisabled {
		return nil, os.ErrInvalid
	}
	conn, err := c.offer(ctx)
	if err != nil {
		return nil, err
	}
	if conn.udpDisabled {
		return nil, E.New("UDP disabled by server")
	}
	conn.udpAccess.Lock()
	defer conn.udpAccess.Unlock()
	select {
	case <-conn.connDone:
		return nil, E.Errors(conn.connErr, os.ErrClosed)
	default:
	}
	if len(conn.udpConnMap) >= maxUDPReceiveSessions {
		conn.receiveBudget.drop()
		return nil, errUDPReceiveCapacity
	}
	// Find a free ID across uint32 wrap without replacing a live session.
	sessionID := conn.udpSessionID
	for attempts := 0; attempts <= maxUDPReceiveSessions; attempts++ {
		if _, present := conn.udpConnMap[sessionID]; !present {
			break
		}
		sessionID++
	}
	conn.udpSessionID = sessionID + 1
	var clientPacketConn *udpPacketConn
	clientPacketConn, err = newUDPPacketConn(conn.ctx, conn.quicConn, &conn.receiveBudget, &conn.sendBudget, func() {
		conn.udpAccess.Lock()
		if conn.udpConnMap[sessionID] == clientPacketConn {
			delete(conn.udpConnMap, sessionID)
		}
		conn.udpAccess.Unlock()
	})
	if err != nil {
		return nil, err
	}
	clientPacketConn.sessionID = sessionID // immutable before map publication
	conn.udpConnMap[sessionID] = clientPacketConn
	return clientPacketConn, nil
}

func (c *Client) CloseWithError(err error) error {
	if err == nil {
		err = net.ErrClosed
	}
	c.connAccess.Lock()
	conn := c.conn
	if conn != nil {
		c.retiring = true
	}
	pending := c.pending
	if pending != nil && !pending.discarded {
		pending.discarded = true
		pending.cause = err
	}
	c.connAccess.Unlock()

	if pending != nil {
		pending.owner.closeWithError(err)
		pending.cancel(err)
	}
	if conn != nil {
		conn.closeWithError(err)
	}
	if pending != nil {
		<-pending.done
		return pending.owner.retirementError()
	}
	if conn != nil {
		return conn.retirementError()
	}
	return nil
}

type clientOffer struct {
	done      chan struct{}
	cancel    func(error)
	owner     *clientQUICConnection
	conn      *clientQUICConnection
	err       error
	discarded bool
	cause     error
}

type clientQUICConnection struct {
	quicConn        *quic.Conn
	lifeAccess      sync.Mutex
	ctx             context.Context
	cancel          func(error)
	initialDone     chan struct{}
	retired         chan struct{}
	closing         bool
	initializing    bool
	failure         error
	closeErr        error
	rawClose        *clientCloseCall
	quicClose       *clientCloseCall
	transportClose  *clientCloseCall
	http3Wait       *clientCloseCall
	opening         *clientSocketDial
	tasks           sync.WaitGroup
	connDone        chan struct{}
	connErr         error
	udpDisabled     bool
	udpAccess       sync.RWMutex
	udpConnMap      map[uint32]*udpPacketConn
	udpSessionID    uint32
	receiveBudget   udpReceiveBudget
	sendBudget      udpSendBudget
	tcpStreams      map[*clientConn]struct{}
	tcpDone         chan struct{}
	tcpReported     bool
	tcpFailure      error
	tcpCalls        int
	tcpControls     int
	tcpStaging      int
	tcpStageBytes   int
	tcpVectors      [maxTCPVectorWrites]tcpVectorWrite
	tcpVectorBytes  int
	tcpVectorInputs int
}

func (c *clientQUICConnection) active() bool {
	if c.quicConn == nil {
		return false
	}
	select {
	case <-c.quicConn.Closing():
		return false
	default:
	}
	select {
	case <-c.quicConn.Context().Done():
		return false
	default:
	}
	select {
	case <-c.connDone:
		return false
	default:
	}
	return true
}
