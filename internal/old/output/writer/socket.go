package writer

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/benthosdev/benthos/v4/internal/codec"
	"github.com/benthosdev/benthos/v4/internal/component"
	"github.com/benthosdev/benthos/v4/internal/component/metrics"
	"github.com/benthosdev/benthos/v4/internal/interop"
	"github.com/benthosdev/benthos/v4/internal/log"
	"github.com/benthosdev/benthos/v4/internal/message"
)

//------------------------------------------------------------------------------

// SocketConfig contains configuration fields for the Socket output type.
type SocketConfig struct {
	Network string `json:"network" yaml:"network"`
	Address string `json:"address" yaml:"address"`
	Codec   string `json:"codec" yaml:"codec"`
}

// NewSocketConfig creates a new SocketConfig with default values.
func NewSocketConfig() SocketConfig {
	return SocketConfig{
		Network: "",
		Address: "",
		Codec:   "lines",
	}
}

//------------------------------------------------------------------------------

// Socket is an output type that sends messages as a continuous steam of line
// delimied messages over socket.
type Socket struct {
	network   string
	address   string
	codec     codec.WriterConstructor
	codecConf codec.WriterConfig

	stats metrics.Type
	log   log.Modular

	writer    codec.Writer
	writerMut sync.Mutex
}

// NewSocket creates a new Socket writer type.
func NewSocket(
	conf SocketConfig,
	mgr interop.Manager,
	log log.Modular,
	stats metrics.Type,
) (*Socket, error) {
	switch conf.Network {
	case "tcp", "udp", "unix":
	default:
		return nil, fmt.Errorf("socket network '%v' is not supported by this output", conf.Network)
	}
	codec, codecConf, err := codec.GetWriter(conf.Codec)
	if err != nil {
		return nil, err
	}
	t := Socket{
		network:   conf.Network,
		address:   conf.Address,
		codec:     codec,
		codecConf: codecConf,
		stats:     stats,
		log:       log,
	}
	return &t, nil
}

//------------------------------------------------------------------------------

// Connect establises a connection to the target socket server.
func (s *Socket) Connect() error {
	return s.ConnectWithContext(context.Background())
}

// ConnectWithContext establises a connection to the target socket server.
func (s *Socket) ConnectWithContext(ctx context.Context) error {
	s.writerMut.Lock()
	defer s.writerMut.Unlock()
	if s.writer != nil {
		return nil
	}

	conn, err := net.Dial(s.network, s.address)
	if err != nil {
		return err
	}

	s.writer, err = s.codec(conn)
	if err != nil {
		conn.Close()
		return err
	}

	s.log.Infof("Sending messages over %v socket to: %s\n", s.network, s.address)
	return nil
}

// Write attempts to write a message.
func (s *Socket) Write(msg *message.Batch) error {
	return s.WriteWithContext(context.Background(), msg)
}

// WriteWithContext attempts to write a message.
func (s *Socket) WriteWithContext(ctx context.Context, msg *message.Batch) error {
	s.writerMut.Lock()
	w := s.writer
	s.writerMut.Unlock()

	if w == nil {
		return component.ErrNotConnected
	}

	return msg.Iter(func(i int, part *message.Part) error {
		serr := w.Write(ctx, part)
		if serr != nil || s.codecConf.CloseAfter {
			s.writerMut.Lock()
			s.writer.Close(ctx)
			s.writer = nil
			s.writerMut.Unlock()
		}
		return serr
	})
}

// CloseAsync shuts down the socket output and stops processing messages.
func (s *Socket) CloseAsync() {
	s.writerMut.Lock()
	if s.writer != nil {
		s.writer.Close(context.Background())
		s.writer = nil
	}
	s.writerMut.Unlock()
}

// WaitForClose blocks until the socket output has closed down.
func (s *Socket) WaitForClose(timeout time.Duration) error {
	return nil
}

//------------------------------------------------------------------------------
