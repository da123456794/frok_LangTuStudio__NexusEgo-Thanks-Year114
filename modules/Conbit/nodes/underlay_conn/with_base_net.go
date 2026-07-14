package underlay_conn

import (
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"github.com/LangTuStudio/Conbit/internal/termlog"
	"github.com/LangTuStudio/Conbit/minecraft/protocol/packet"
	"github.com/LangTuStudio/Conbit/nodes/defines"
)

func NewBasicNetServer(addr string) (net.Listener, error) {
	frags := strings.Split(addr, "://")
	if len(frags) != 2 {
		return nil, fmt.Errorf("must be in format of network://address. e.g. tcp://0.0.0.0:2401")
	}
	if frags[0] == "unix" {
		os.Remove(frags[1])
	}
	return net.Listen(frags[0], frags[1])
}

func NewBasicNetClient(addr string, timeout time.Duration) (net.Conn, error) {
	frags := strings.Split(addr, "://")
	if len(frags) != 2 {
		return nil, fmt.Errorf("must be in format of network://address. e.g. tcp://127.0.0.1:2401")
	}
	return net.DialTimeout(frags[0], frags[1], timeout)
}

func NewClientFromBasicNet(addr string, timeout time.Duration) (defines.NewMasterNodeAPIClient, error) {
	conn, err := NewBasicNetClient(addr, timeout)
	if err != nil {
		return nil, err
	}
	frameConn := NewConnectionFromNet(conn)
	frameConn.EnableCompression(packet.SnappyCompression())
	client := NewFrameAPIClient(frameConn)
	go client.Run()
	return client, nil
}

func NewServerFromBasicNet(addr string) (defines.NewMasterNodeAPIServer, error) {
	listen, err := NewBasicNetServer(addr)
	if err != nil {
		return nil, err
	}
	server := NewFrameAPIServer(func() { listen.Close() })
	go func() {
		for {
			if server.Closed() {
				break
			}
			conn, err := listen.Accept()
			if err != nil {
				if errors.Is(err, net.ErrClosed) || server.Closed() {
					break
				}
				termlog.ErrorDetail(err, "接入点网络监听接收连接失败")
				continue
			}
			frameConn := NewConnectionFromNet(conn)
			frameConn.EnableCompression(packet.SnappyCompression())
			serveConn := server.NewFrameAPIServer(frameConn)
			go func() {
				serveConn.Run()
				if err := <-serveConn.WaitClosed(); err != nil {
					termlog.ErrorDetail(err, "访问点连接已断开")
				} else {
					termlog.Infof("访问点连接已断开")
				}
				conn.Close()
			}()
		}
	}()
	return server, nil
}
