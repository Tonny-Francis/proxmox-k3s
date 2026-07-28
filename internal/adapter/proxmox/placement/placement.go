package placement

import (
	"fmt"
	"sync"

	"github.com/nexusops/proxmox-k3s/internal/adapter/proxmox/node"
)

type Strategy interface {
	Pick(nodes []node.PVENode, exclude []string) (string, error)
}

type RoundRobin struct {
	mu      sync.Mutex
	counter int
}

func NewRoundRobin() *RoundRobin {
	return &RoundRobin{}
}

func (r *RoundRobin) Pick(nodes []node.PVENode, exclude []string) (string, error) {
	online := filterOnline(nodes, exclude)
	if len(online) == 0 {
		return "", fmt.Errorf("no Proxmox nodes available (online and not excluded)")
	}

	r.mu.Lock()
	idx := r.counter % len(online)
	r.counter++
	r.mu.Unlock()

	return online[idx].Name, nil
}

type LeastLoaded struct{}

func NewLeastLoaded() *LeastLoaded {
	return &LeastLoaded{}
}

func (l *LeastLoaded) Pick(nodes []node.PVENode, exclude []string) (string, error) {
	online := filterOnline(nodes, exclude)
	if len(online) == 0 {
		return "", fmt.Errorf("no Proxmox nodes available (online and not excluded)")
	}

	var best node.PVENode
	var bestRatio float64 = 2.0

	for _, n := range online {
		if n.MaxMem == 0 {
			continue
		}
		ratio := float64(n.Mem) / float64(n.MaxMem)
		if ratio < bestRatio {
			bestRatio = ratio
			best = n
		}
	}

	if best.Name == "" {
		return online[0].Name, nil
	}
	return best.Name, nil
}

func NewStrategy(name string) Strategy {
	switch name {
	case "least-loaded":
		return NewLeastLoaded()
	default:
		return NewRoundRobin()
	}
}

func filterOnline(nodes []node.PVENode, exclude []string) []node.PVENode {
	excl := make(map[string]bool, len(exclude))
	for _, e := range exclude {
		excl[e] = true
	}

	var out []node.PVENode
	for _, n := range nodes {
		if n.Status == "online" && !excl[n.Name] {
			out = append(out, n)
		}
	}
	return out
}
