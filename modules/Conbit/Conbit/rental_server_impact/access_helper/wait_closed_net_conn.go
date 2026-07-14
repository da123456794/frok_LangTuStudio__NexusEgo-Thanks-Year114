package access_helper

import (
	"net"
	"sync"
)

type waitClosedNetConn struct {
	net.Conn
	closeOnce  sync.Once
	waitClosed chan struct{}
}

func newWaitClosedNetConn(conn net.Conn) *waitClosedNetConn {
	return &waitClosedNetConn{
		Conn:       conn,
		waitClosed: make(chan struct{}),
	}
}

func (conn *waitClosedNetConn) Close() error {
	err := conn.Conn.Close()
	conn.closeOnce.Do(func() {
		close(conn.waitClosed)
	})
	return err
}

func (conn *waitClosedNetConn) WaitClosed() chan struct{} {
	return conn.waitClosed
}
