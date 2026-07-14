package base_net

import (
	"context"
	"github.com/sandertv/go-raknet"
	"net"
	"time"
)

// RakNet is an implementation of a RakNet v10 Network.
type rakNet struct{}

// DialContext creates a new connection using the context provided.
func (r rakNet) DialContext(ctx context.Context, address string) (interface {
	net.Conn
	WaitClosed() chan struct{}
}, error) {
	conn, err := raknet.DialContext(ctx, address)
	if err != nil {
		return nil, err
	}
	wrapper := &RaknetConnWrapper{
		conn:   conn,
		closed: make(chan struct{}),
	}
	return wrapper, nil
}

// PingContext pings the server at the given address and returns the response.
func (r rakNet) PingContext(ctx context.Context, address string) (response []byte, err error) {
	return raknet.PingContext(ctx, address)
}

// Listen starts listening for incoming connections on the specified address.
func (r rakNet) Listen(address string) (NetworkListener, error) {
	return raknet.Listen(address)
}

var RakNet Network

func init() {
	RakNet = rakNet{}
}

// RaknetConnWrapper wraps around *raknet.Conn to add WaitClosed method.
type RaknetConnWrapper struct {
	conn   *raknet.Conn
	closed chan struct{}
}

// WaitClosed returns a channel that will be closed when Close is called.
func (w *RaknetConnWrapper) WaitClosed() chan struct{} {
	return w.closed
}

// Implement net.Conn interface methods by delegating to the underlying *raknet.Conn.

func (w *RaknetConnWrapper) Read(b []byte) (n int, err error)  { return w.conn.Read(b) }
func (w *RaknetConnWrapper) Write(b []byte) (n int, err error) { return w.conn.Write(b) }
func (w *RaknetConnWrapper) Close() error {
	select {
	case <-w.closed:
	default:
		close(w.closed)
	}
	return w.conn.Close()
}
func (w *RaknetConnWrapper) LocalAddr() net.Addr               { return w.conn.LocalAddr() }
func (w *RaknetConnWrapper) RemoteAddr() net.Addr              { return w.conn.RemoteAddr() }
func (w *RaknetConnWrapper) SetDeadline(t time.Time) error     { return w.conn.SetDeadline(t) }
func (w *RaknetConnWrapper) SetReadDeadline(t time.Time) error { return w.conn.SetReadDeadline(t) }
func (w *RaknetConnWrapper) SetWriteDeadline(t time.Time) error {
	return w.conn.SetWriteDeadline(t)
}
