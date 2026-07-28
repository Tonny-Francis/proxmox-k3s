package ipam

import (
	"fmt"
	"net"
	"strings"
)

type IPPool struct {
	start net.IP
	end   net.IP
	ips   []net.IP
}

func ParsePool(rangeStr string) (*IPPool, error) {
	parts := strings.SplitN(rangeStr, "-", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid IP pool range %q: expected format start-end", rangeStr)
	}

	start := net.ParseIP(strings.TrimSpace(parts[0]))
	end := net.ParseIP(strings.TrimSpace(parts[1]))

	if start == nil {
		return nil, fmt.Errorf("invalid start IP %q", parts[0])
	}
	if end == nil {
		return nil, fmt.Errorf("invalid end IP %q", parts[1])
	}

	var ips []net.IP
	for ip := start; !ip.Equal(end); incrementIP(ip) {
		cp := make(net.IP, len(ip))
		copy(cp, ip)
		ips = append(ips, cp)
	}
	cp := make(net.IP, len(end))
	copy(cp, end)
	ips = append(ips, cp)

	return &IPPool{start: start, end: end, ips: ips}, nil
}

func (p *IPPool) Get(index int) (string, error) {
	if index < 0 || index >= len(p.ips) {
		return "", fmt.Errorf("index %d out of range (pool has %d IPs)", index, len(p.ips))
	}
	return p.ips[index].String(), nil
}

func (p *IPPool) Size() int {
	return len(p.ips)
}

func (p *IPPool) Contains(ip string) bool {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return false
	}
	for _, poolIP := range p.ips {
		if poolIP.Equal(parsed) {
			return true
		}
	}
	return false
}

func incrementIP(ip net.IP) {
	for i := len(ip) - 1; i >= 0; i-- {
		ip[i]++
		if ip[i] != 0 {
			break
		}
	}
}

type StaticAllocator struct {
	pool    *IPPool
	cidr    string
	gateway string
}

func NewStaticAllocator(poolRange, cidr, gateway string) (*StaticAllocator, error) {
	pool, err := ParsePool(poolRange)
	if err != nil {
		return nil, err
	}
	return &StaticAllocator{pool: pool, cidr: cidr, gateway: gateway}, nil
}

func (a *StaticAllocator) IPForIndex(index int) (string, error) {
	return a.pool.Get(index)
}

func (a *StaticAllocator) IPConfigForIndex(index int) (string, error) {
	ip, err := a.pool.Get(index)
	if err != nil {
		return "", err
	}

	prefix := "24"
	if a.cidr != "" {
		_, ipNet, err := net.ParseCIDR(a.cidr)
		if err == nil {
			ones, _ := ipNet.Mask.Size()
			prefix = fmt.Sprintf("%d", ones)
		}
	}

	cfg := fmt.Sprintf("ip=%s/%s", ip, prefix)
	if a.gateway != "" {
		cfg += ",gw=" + a.gateway
	}
	return cfg, nil
}

func (a *StaticAllocator) TotalIPs() int {
	return a.pool.Size()
}
