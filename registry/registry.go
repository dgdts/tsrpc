package registry

import (
	"log"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

// TSRegistry represents a registry for managing RPC servers.
type TSRegistry struct {
	timeout time.Duration          // timeout is the duration after which a server is considered inactive.
	mu      sync.Mutex             // mu is used to synchronize access to the servers map.
	servers map[string]*ServerItem // servers maps server addresses to their corresponding ServerItem.
}

// ServerItem represents information about an RPC server.
type ServerItem struct {
	Addr  string    // Addr is the address of the RPC server.
	start time.Time // start is the time when the server was registered.
}

const (
	defaultPath    = "/_tsrpc_/registry" // defaultPath is the default path for the registry.
	defaultTimeout = time.Minute * 5     // defaultTimeout is the default timeout duration.
)

// DefaultTSRegister is the default TSRegistry instance.
var DefaultTSRegister = New(defaultTimeout)

// New creates a new TSRegistry instance with the specified timeout duration.
func New(timeout time.Duration) *TSRegistry {
	return &TSRegistry{
		servers: make(map[string]*ServerItem),
		timeout: timeout,
	}
}

// putServer registers a server with the given address.
func (r *TSRegistry) putServer(addr string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	s := r.servers[addr]
	if s == nil {
		r.servers[addr] = &ServerItem{Addr: addr, start: time.Now()}
	} else {
		s.start = time.Now()
	}
}

// aliveServers returns the addresses of all active servers.
func (r *TSRegistry) aliveServers() []string {
	r.mu.Lock()
	defer r.mu.Unlock()

	var alive []string
	for addr, s := range r.servers {
		if r.timeout == 0 || s.start.Add(r.timeout).After(time.Now()) {
			alive = append(alive, addr)
		} else {
			delete(r.servers, addr)
		}
	}
	sort.Strings(alive)
	return alive
}

// ServeHTTP handles HTTP requests for the registry.
func (r *TSRegistry) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case "GET":
		w.Header().Set("X-TSrpc-Servers", strings.Join(r.aliveServers(), ","))
	case "POST":
		addr := req.Header.Get("X-TSrpc-Server")
		if addr == "" {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		r.putServer(addr)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// HandleHTTP registers the registry's HTTP handler with the specified registry path.
func (r *TSRegistry) HandleHTTP(registryPath string) {
	http.Handle(registryPath, r)
	log.Println("rpc registry path:", registryPath)
}

// HandleHTTP registers the default registry's HTTP handler with the default registry path.
func HandleHTTP() {
	DefaultTSRegister.HandleHTTP(defaultPath)
}

// Heartbeat periodically sends heartbeats to the registry.
func Heartbeat(registry, addr string, duration time.Duration) {
	if duration == 0 {
		duration = defaultTimeout - time.Duration(1)*time.Minute
	}
	var err error
	err = sendHeartbeat(registry, addr)
	go func() {
		t := time.NewTicker(duration)
		for err == nil {
			<-t.C
			err = sendHeartbeat(registry, addr)
		}
	}()
}

// sendHeartbeat sends a heartbeat message to the registry for the specified address.
func sendHeartbeat(registry, addr string) error {
	log.Println(addr, "send heartbeat to registry", registry)
	httpClient := &http.Client{}
	req, _ := http.NewRequest("POST", registry, nil)
	req.Header.Set("X-TSrpc-Server", addr)
	if _, err := httpClient.Do(req); err != nil {
		log.Println("rpc server: heartbeat err:", err)
		return err
	}
	return nil
}
