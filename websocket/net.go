package websocket

import (
	"context"
	"fmt"
	"golang.org/x/net/websocket"
	"io"
	"net"
	"netshaper/conf"
	"sync"
	"time"
)

const (
	DefaultProtocol       = ""
	DefaultOrigin         = "http://localhost"
	DefaultSendTimeout    = time.Duration(0)
	DefaultReceiveTimeout = time.Duration(0)
	DefaultBuffSize       = uint(128)
)

func NewNet(opts ...conf.Option[NetConfig]) conf.Option[Config] {
	return conf.OptionFunc[Config](func(config Config) Config {
		if config != nil {
			panic(fmt.Errorf("net option received non-nil config %#v", config))
		}

		cfg := conf.ApplyOptions(opts)
		return &cfg
	})
}

func WithNetProtocol(protocol string) conf.Option[NetConfig] {
	return conf.OptionFunc[NetConfig](func(config NetConfig) NetConfig {
		config.Protocol = protocol
		return config
	})
}

func WithNetOrigin(origin string) conf.Option[NetConfig] {
	return conf.OptionFunc[NetConfig](func(config NetConfig) NetConfig {
		config.Origin = origin
		return config
	})
}

func WithNetSendTimeout(timeout time.Duration) conf.Option[NetConfig] {
	return conf.OptionFunc[NetConfig](func(config NetConfig) NetConfig {
		config.SendTimeout = timeout
		return config
	})
}

func WithNetReceiveTimeout(timeout time.Duration) conf.Option[NetConfig] {
	return conf.OptionFunc[NetConfig](func(config NetConfig) NetConfig {
		config.ReceiveTimeout = timeout
		return config
	})
}

func WithNetBufferSize(size uint) conf.Option[NetConfig] {
	return conf.OptionFunc[NetConfig](func(config NetConfig) NetConfig {
		config.BufferSize = size
		return config
	})
}

var _ Config = (*NetConfig)(nil)

type NetConfig struct {
	Protocol       string
	Origin         string
	SendTimeout    time.Duration
	ReceiveTimeout time.Duration
	BufferSize     uint
}

func (c *NetConfig) Create(ctx context.Context) (Client, error) {
	ctx, cancel := context.WithCancel(ctx)

	protocol := c.Protocol
	if protocol == "" {
		protocol = DefaultProtocol
	}
	origin := c.Origin
	if origin == "" {
		origin = DefaultOrigin
	}
	sendTimeout := c.SendTimeout
	if sendTimeout == 0 {
		sendTimeout = DefaultSendTimeout
	}
	receiveTimeout := c.ReceiveTimeout
	if receiveTimeout == 0 {
		receiveTimeout = DefaultReceiveTimeout
	}
	buffSize := c.BufferSize
	if buffSize == 0 {
		buffSize = DefaultBuffSize
	}

	return &netClient{
		ctx:            ctx,
		cancel:         cancel,
		protocol:       protocol,
		origin:         origin,
		sendTimeout:    sendTimeout,
		receiveTimeout: receiveTimeout,
		bufferSize:     buffSize,
	}, nil
}

var _ Client = (*netClient)(nil)

type netClient struct {
	ctx            context.Context
	cancel         context.CancelFunc
	responsesWg    sync.WaitGroup
	protocol       string
	origin         string
	sendTimeout    time.Duration
	receiveTimeout time.Duration
	bufferSize     uint
}

func (c *netClient) Request(req *Request) (res RawResponse, err error) {
	protocol := req.Protocol
	if protocol == "" {
		protocol = c.protocol
	}
	origin := req.Origin
	if origin == "" {
		origin = c.origin
	}
	receiveTimeout := req.ReceiveTimeout
	if receiveTimeout == 0 {
		receiveTimeout = c.receiveTimeout
	}
	buffSize := req.BufferSize
	if buffSize == 0 {
		buffSize = c.bufferSize
	}

	// TODO: call with context & timeouts
	conn, err := websocket.Dial(req.URL.String(), protocol, origin)
	if err != nil {
		return nil, err
	}
	select {
	case <-c.ctx.Done():
		err = c.ctx.Err()
		return
	case <-req.Context().Done():
		err = req.Context().Err()
		return
	default:
		break
	}

	wsResCtx, wsResCancel := context.WithCancel(req.Context())
	wsRes := &netResponse{
		clientCtx:      c.ctx,
		ctx:            wsResCtx,
		cancel:         wsResCancel,
		responseWg:     &c.responsesWg,
		conn:           conn,
		sendTimeout:    c.sendTimeout,
		receiveTimeout: c.receiveTimeout,
		messages:       make(chan Message, buffSize),
	}

	defer c.responsesWg.Add(1)
	defer wsRes.wg.Add(1)
	go wsRes.run()

	return wsRes, err
}

func (c *netClient) Close(_ context.Context) {
	defer c.responsesWg.Wait()
	c.cancel()
}

var _ RawResponse = (*netResponse)(nil)

type netResponse struct {
	clientCtx      context.Context
	ctx            context.Context
	cancel         context.CancelFunc
	responseWg     *sync.WaitGroup
	conn           *websocket.Conn
	sendTimeout    time.Duration
	receiveTimeout time.Duration
	messages       chan Message
	err            error
	wg             sync.WaitGroup
}

func (r *netResponse) Send(message Message) (err error) {
	if r.sendTimeout > 0 {
		err = r.conn.SetWriteDeadline(time.Now().Add(r.sendTimeout))
		if err != nil {
			return
		}
	}

	_, err = r.conn.Write(message.Buff())

	return
}

func (r *netResponse) Listen() <-chan Message {
	return r.messages
}

func (r *netResponse) Closed() <-chan struct{} {
	return r.ctx.Done()
}

func (r *netResponse) Err() error {
	return r.err
}

func (r *netResponse) Close(_ context.Context) {
	defer r.responseWg.Done()
	defer r.wg.Wait()
	r.cancel()
}

func (r *netResponse) run() {
	defer r.wg.Done()
	defer close(r.messages)
	defer func() { _ = r.conn.Close() }()
	defer r.cancel()

	for {
		select {
		case <-r.clientCtx.Done():
			return
		case <-r.ctx.Done():
			return
		default:
			msg, err, eof := r.receiveMessage()
			if eof {
				r.err = err
				return
			}

			r.messages <- msg
		}
	}
}

func (r *netResponse) receiveMessage() (msg Message, err error, eof bool) {
	if r.receiveTimeout > 0 {
		err = r.conn.SetReadDeadline(time.Now().Add(r.receiveTimeout))
		if err != nil {
			return
		}
	}

	buf := []byte{}
	err = websocket.Message.Receive(r.conn, &buf)

	switch err {
	case nil:
		msg = ByteMessage(buf)
	case io.EOF, io.ErrUnexpectedEOF:
		err = nil
		eof = true
	default:
		switch err.(type) {
		case *net.OpError:
			eof = true
		default:
			msg = &ErrorMessage{err}
		}
	}

	return
}
