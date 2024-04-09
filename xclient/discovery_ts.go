package xclient

import (
	"log"
	"net/http"
	"strings"
	"time"
)

// TSRegistryDiscovery is a Discovery implementation that retrieves server addresses from a registry.
type TSRegistryDiscovery struct {
	*MultiServerDiscovery               // TSRegistryDiscovery embeds MultiServerDiscovery for server management.
	registry              string        // registry is the address of the registry.
	timeout               time.Duration // timeout is the duration after which a refresh is triggered.
	lastUpdate            time.Time     // lastUpdate is the time when the server list was last updated.
}

const defaultUpdateTimeout = time.Second * 10 // defaultUpdateTimeout is the default timeout for updates.

// NewTSRegistryDiscovery creates a new TSRegistryDiscovery instance with the specified registry address and timeout duration.
func NewTSRegistryDiscovery(registerAddr string, timeout time.Duration) *TSRegistryDiscovery {
	if timeout == 0 {
		timeout = defaultUpdateTimeout
	}

	return &TSRegistryDiscovery{
		MultiServerDiscovery: NewMultiServerDiscovery(make([]string, 0)),
		registry:             registerAddr,
		timeout:              timeout,
	}
}

// Update updates the list of servers, but this implementation does nothing as servers are refreshed asynchronously.
func (d *TSRegistryDiscovery) Update(servers []string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.servers = servers
	d.lastUpdate = time.Now()
	return nil
}

// Refresh refreshes the server list from the registry if the timeout has expired since the last update.
func (d *TSRegistryDiscovery) Refresh() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.lastUpdate.Add(d.timeout).After(time.Now()) {
		return nil
	}

	log.Println("rpc registry: refresh servers from registry", d.registry)
	resp, err := http.Get(d.registry)
	if err != nil {
		log.Println("rpc registry refresh err:", err)
		return err
	}
	servers := strings.Split(resp.Header.Get("X-TSrpc-Servers"), ",")
	d.servers = make([]string, 0, len(servers))
	for _, server := range servers {
		trimServer := strings.TrimSpace(server)
		if trimServer != "" {
			d.servers = append(d.servers, trimServer)
		}
	}
	d.lastUpdate = time.Now()
	return nil
}

// Get retrieves a server address based on the specified selection mode, triggering a refresh if necessary.
func (d *TSRegistryDiscovery) Get(mode SelectMode) (string, error) {
	if err := d.Refresh(); err != nil {
		return "", err
	}
	return d.MultiServerDiscovery.Get(mode)
}

// GetAll returns all available server addresses, triggering a refresh if necessary.
func (d *TSRegistryDiscovery) GetAll() ([]string, error) {
	if err := d.Refresh(); err != nil {
		return nil, err
	}
	return d.MultiServerDiscovery.GetAll()
}
