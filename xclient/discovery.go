package xclient

import (
	"errors"
	"math"
	"math/rand"
	"sync"
	"time"
)

// SelectMode represents the mode for selecting servers.
type SelectMode int

const (
	// RandomSelect selects a server randomly.
	RandomSelect SelectMode = iota
	// RoundRobinSelect selects servers in a round-robin manner.
	RoundRobinSelect
)

// Discovery defines the interface for server discovery.
type Discovery interface {
	Refresh() error
	Update(servers []string) error
	Get(mode SelectMode) (string, error)
	GetAll() ([]string, error)
}

// MultiServerDiscovery is a Discovery implementation that manages multiple servers.
type MultiServerDiscovery struct {
	r       *rand.Rand   // r is the random number generator.
	mu      sync.RWMutex // mu is used to synchronize access to the servers slice and index.
	servers []string     // servers stores the addresses of available servers.
	index   int          // index is used for round-robin server selection.
}

// NewMultiServerDiscovery creates a new MultiServerDiscovery instance with the given servers.
func NewMultiServerDiscovery(servers []string) *MultiServerDiscovery {
	d := &MultiServerDiscovery{
		servers: servers,
		r:       rand.New(rand.NewSource(time.Now().UnixNano())),
	}
	d.index = d.r.Intn(math.MaxInt32 - 1)
	return d
}

// Ensure MultiServerDiscovery implements the Discovery interface.
var _ Discovery = (*MultiServerDiscovery)(nil)

// Refresh refreshes the discovery, but this implementation does nothing.
func (d *MultiServerDiscovery) Refresh() error {
	return nil
}

// Update updates the list of servers.
func (d *MultiServerDiscovery) Update(servers []string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.servers = servers
	return nil
}

// Get retrieves a server address based on the specified selection mode.
func (d *MultiServerDiscovery) Get(mode SelectMode) (string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	n := len(d.servers)
	if n == 0 {
		return "", errors.New("rpc discovery: no available servers")
	}
	switch mode {
	case RandomSelect:
		return d.servers[d.r.Intn(n)], nil
	case RoundRobinSelect:
		s := d.servers[d.index%n]
		d.index = (d.index + 1) % n
		return s, nil
	default:
		return "", errors.New("rpc discovery: not supported select mode")
	}
}

// GetAll returns all available server addresses.
func (d *MultiServerDiscovery) GetAll() ([]string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	servers := make([]string, len(d.servers))
	copy(servers, d.servers)
	return servers, nil
}
