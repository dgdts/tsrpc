package tsrpc

import (
	"context"
	"net"
	"strings"
	"testing"
	"time"
)

// TestClient_dialTimeout tests the dialTimeout function.
func TestClient_dialTimeout(t *testing.T) {
	t.Parallel()
	l, _ := net.Listen("tcp", ":0")

	// Dummy function for dialing with timeout.
	f := func(conn net.Conn, opt *Option) (client *Client, err error) {
		_ = conn.Close()
		time.Sleep(time.Second * 2)
		return nil, nil
	}

	// Test case for dialing with a connect timeout.
	t.Run("timeout", func(t *testing.T) {
		_, err := dialTimeout(f, "tcp", l.Addr().String(), &Option{ConnectTimeout: time.Second})
		_assert(err != nil && strings.Contains(err.Error(), "connect timeout"), "expect a timeout error")
	})

	// Test case for dialing without a connect timeout.
	t.Run("no timeout", func(t *testing.T) {
		_, err := dialTimeout(f, "tcp", l.Addr().String(), &Option{ConnectTimeout: 0})
		_assert(err == nil, "should never timeout")
	})
}

// Bar is a dummy type used for testing.
type Bar int

// Timeout is a dummy method of Bar.
func (b Bar) Timeout(argv int, reply *int) error {
	time.Sleep(time.Second * 2)
	return nil
}

// startServer starts a dummy server for testing.
func startServer(addr chan string) {
	var b Bar
	_ = Register(&b)
	l, _ := net.Listen("tcp", ":0")
	addr <- l.Addr().String()
	Accept(l)
}

// TestClient_Call tests the Call method of the Client.
func TestClient_Call(t *testing.T) {
	t.Parallel()
	addrCh := make(chan string)
	go startServer(addrCh)
	addr := <-addrCh
	time.Sleep(time.Second)
	// Test case for client timeout.
	t.Run("client timeout", func(t *testing.T) {
		client, _ := Dial("tcp", addr)
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		var reply int
		err := client.Call(ctx, "Bar.Timeout", 1, &reply)
		_assert(err != nil && strings.Contains(err.Error(), ctx.Err().Error()), "expect a timeout error")
	})
	// Test case for server handle timeout.
	t.Run("server handle timeout", func(t *testing.T) {
		client, _ := Dial("tcp", addr, &Option{
			HandleTimeout: time.Second,
		})
		var reply int
		err := client.Call(context.Background(), "Bar.Timeout", 1, &reply)
		_assert(err != nil && strings.Contains(err.Error(), "timeout"), "expect a timeout error")
	})
}
