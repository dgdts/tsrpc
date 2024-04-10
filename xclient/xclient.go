package xclient

import (
	"context"
	"io"
	"reflect"
	"sync"

	"github.com/dgdts/tsrpc"
)

// XClient represents an RPC client that supports dynamic server discovery.
type XClient struct {
	d       Discovery                // d is the server discovery instance.
	mode    SelectMode               // mode is the server selection mode.
	opt     *tsrpc.Option            // opt is the RPC client options.
	mu      sync.Mutex               // mu is used to synchronize access to the clients map.
	clients map[string]*tsrpc.Client // clients maps server addresses to their corresponding RPC clients.
}

// Ensure XClient implements the io.Closer interface.
var _ io.Closer = (*XClient)(nil)

// NewXClient creates a new XClient instance with the specified server discovery, selection mode, and options.
func NewXClient(d Discovery, mode SelectMode, opt *tsrpc.Option) *XClient {
	return &XClient{
		d:       d,
		mode:    mode,
		opt:     opt,
		clients: make(map[string]*tsrpc.Client),
	}
}

// Close closes all RPC clients.
func (x *XClient) Close() error {
	x.mu.Lock()
	defer x.mu.Unlock()
	for key, client := range x.clients {
		_ = client.Close()
		delete(x.clients, key)
	}
	return nil
}

// dial retrieves or creates an RPC client for the specified server address.
func (x *XClient) dial(rpcAddr string) (*tsrpc.Client, error) {
	x.mu.Lock()
	defer x.mu.Unlock()
	client, ok := x.clients[rpcAddr]
	if ok && !client.IsAvaliable() {
		_ = client.IsAvaliable()
		delete(x.clients, rpcAddr)
		client = nil
	}
	if client == nil {
		var err error
		client, err = tsrpc.XDial(rpcAddr, x.opt)
		if err != nil {
			return nil, err
		}
		x.clients[rpcAddr] = client
	}
	return client, nil
}

// call invokes an RPC call to the specified server address with the given context, service method, and arguments.
func (x *XClient) call(rpcAddr string, ctx context.Context, serviceMethod string, args, reply interface{}) error {
	client, err := x.dial(rpcAddr)
	if err != nil {
		return err
	}
	return client.Call(ctx, serviceMethod, args, reply)
}

// Call invokes an RPC call to a selected server with the specified context, service method, and arguments.
func (x *XClient) Call(ctx context.Context, serviceMethod string, args, reply interface{}) error {
	rpcAddr, err := x.d.Get(x.mode)
	if err != nil {
		return err
	}
	return x.call(rpcAddr, ctx, serviceMethod, args, reply)
}

// Broadcast sends an RPC call to all available servers concurrently with the specified context, service method, and arguments.
func (x *XClient) Broadcast(ctx context.Context, serviceMethod string, args, reply interface{}) error {
	servers, err := x.d.GetAll()
	if err != nil {
		return err
	}
	var wg sync.WaitGroup
	var mu sync.Mutex
	var e error
	replyDone := reply == nil
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	for _, rpcAddr := range servers {
		wg.Add(1)
		go func(rpcAddr string) {
			defer wg.Done()
			var cloneReply interface{}
			if reply != nil {
				cloneReply = reflect.New(reflect.ValueOf(reply).Elem().Type()).Interface()
			}
			err := x.call(rpcAddr, ctx, serviceMethod, args, reply)
			mu.Lock()
			if err != nil && e == nil {
				e = err
				cancel()
			}
			if err == nil && !replyDone {
				reflect.ValueOf(reply).Elem().Set(reflect.ValueOf(cloneReply).Elem())
				replyDone = true
			}
			mu.Unlock()
		}(rpcAddr)
	}

	wg.Wait()
	return e
}
