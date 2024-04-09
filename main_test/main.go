package main

import (
	"context"
	"log"
	"net"
	"net/http"
	"sync"
	"time"
	"tsrpc"
	"tsrpc/registry"
	"tsrpc/xclient"
)

// Foo represents a sample type for testing.
type Foo int

// Args represents arguments for testing.
type Args struct{ Num1, Num2 int }

// Sum is a method of Foo that sums two integers.
func (f Foo) Sum(args Args, reply *int) error {
	*reply = args.Num1 + args.Num2
	return nil
}

// Sleep is a method of Foo that sleeps for a specified duration and then sums two integers.
func (f Foo) Sleep(args Args, reply *int) error {
	time.Sleep(time.Second * time.Duration(args.Num1))
	*reply = args.Num1 + args.Num2
	return nil
}

// startRegistry starts the registry server.
func startRegistry(wg *sync.WaitGroup) {
	l, _ := net.Listen("tcp", ":9999") // Listen on port 9999
	registry.HandleHTTP()              // Handle HTTP requests for registry
	wg.Done()
	_ = http.Serve(l, nil) // Serve requests using the HTTP server
}

// startServer starts an RPC server.
func startServer(registryAddr string, wg *sync.WaitGroup) {
	var foo Foo
	l, err := net.Listen("tcp", ":0") // Listen on a random port
	if err != nil {
		log.Fatal("network error:", err)
	}
	server := tsrpc.NewServer() // Create a new RPC server
	err = server.Register(&foo) // Register the Foo service
	if err != nil {
		log.Fatal("register error:", err)
	}
	registry.Heartbeat(registryAddr, "tcp@"+l.Addr().String(), 0) // Register server with the registry
	wg.Done()
	server.Accept(l) // Accept incoming connections
}

// foo is a helper function for making RPC calls or broadcasts.
func foo(xc *xclient.XClient, ctx context.Context, typ, serviceMethod string, args *Args) {
	var reply int
	var err error
	switch typ {
	case "call":
		err = xc.Call(ctx, serviceMethod, args, &reply) // Make an RPC call
	case "broadcast":
		err = xc.Broadcast(ctx, serviceMethod, args, &reply) // Broadcast an RPC call
	}
	if err != nil {
		log.Printf("%s %s error: %v", typ, serviceMethod, err)
	} else {
		log.Printf("%s %s success: %d + %d = %d", typ, serviceMethod, args.Num1, args.Num2, reply)
	}
}

// call performs RPC calls to the registry server.
func call(registry string) {
	d := xclient.NewTSRegistryDiscovery(registry, 0)           // Discover registry server
	xc := xclient.NewXClient(d, xclient.RoundRobinSelect, nil) // Create a new RPC client
	defer func() {
		_ = xc.Close()
	}()
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			foo(xc, context.Background(), "call", "Foo.Sum", &Args{Num1: i, Num2: i * i}) // Make RPC calls
		}(i)
	}
	wg.Wait()
}

// broadcast performs RPC broadcasts to the registry server.
func broadcast(registry string) {
	d := xclient.NewTSRegistryDiscovery(registry, 0)           // Discover registry server
	xc := xclient.NewXClient(d, xclient.RoundRobinSelect, nil) // Create a new RPC client
	defer func() {
		_ = xc.Close()
	}()
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			foo(xc, context.Background(), "broadcast", "Foo.Sum", &Args{Num1: i, Num2: i * i}) // Make RPC broadcasts
			ctx, cancel := context.WithTimeout(context.Background(), time.Second*2)            // Set a timeout for Sleep RPC
			defer cancel()
			foo(xc, ctx, "broadcast", "Foo.Sleep", &Args{Num1: i, Num2: i * i}) // Make Sleep RPC calls
		}(i)
	}
	wg.Wait()
}

func main() {
	log.SetFlags(0)
	registryAddr := "http://localhost:9999/_tsrpc_/registry" // Registry server address
	var wg sync.WaitGroup
	wg.Add(1)

	go startRegistry(&wg) // Start registry server
	wg.Wait()

	time.Sleep(time.Second) // Wait for registry to start
	wg.Add(2)
	go startServer(registryAddr, &wg) // Start two RPC servers
	go startServer(registryAddr, &wg)
	wg.Wait()

	time.Sleep(time.Second) // Wait for servers to start
	call(registryAddr)      // Make RPC calls
	broadcast(registryAddr) // Make RPC broadcasts
}
