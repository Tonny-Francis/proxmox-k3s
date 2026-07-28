package ipam

import (
	"testing"
)

func TestParsePool(t *testing.T) {
	pool, err := ParsePool("192.168.1.10-192.168.1.20")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pool.Size() != 11 {
		t.Errorf("expected 11 IPs, got %d", pool.Size())
	}
}

func TestParsePool_InvalidRange(t *testing.T) {
	_, err := ParsePool("192.168.1.10")
	if err == nil {
		t.Error("expected error for invalid range")
	}
}

func TestIPPool_Get(t *testing.T) {
	pool, _ := ParsePool("10.0.0.1-10.0.0.5")

	first, err := pool.Get(0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if first != "10.0.0.1" {
		t.Errorf("expected 10.0.0.1, got %s", first)
	}

	last, err := pool.Get(4)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if last != "10.0.0.5" {
		t.Errorf("expected 10.0.0.5, got %s", last)
	}
}

func TestIPPool_GetOutOfRange(t *testing.T) {
	pool, _ := ParsePool("10.0.0.1-10.0.0.5")
	_, err := pool.Get(10)
	if err == nil {
		t.Error("expected error for out-of-range index")
	}
}

func TestStaticAllocator_IPConfig(t *testing.T) {
	alloc, err := NewStaticAllocator("192.168.20.50-192.168.20.99", "192.168.20.0/24", "192.168.20.1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ipCfg, err := alloc.IPConfigForIndex(0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if ipCfg != "ip=192.168.20.50/24,gw=192.168.20.1" {
		t.Errorf("unexpected IP config: %s", ipCfg)
	}
}

func TestStaticAllocator_Deterministic(t *testing.T) {
	alloc, _ := NewStaticAllocator("10.0.0.10-10.0.0.20", "10.0.0.0/24", "10.0.0.1")

	ip1, _ := alloc.IPForIndex(3)
	ip2, _ := alloc.IPForIndex(3)

	if ip1 != ip2 {
		t.Errorf("allocation is not deterministic: %s != %s", ip1, ip2)
	}
	if ip1 != "10.0.0.13" {
		t.Errorf("expected 10.0.0.13, got %s", ip1)
	}
}

func TestStaticAllocator_TotalIPs(t *testing.T) {
	alloc, _ := NewStaticAllocator("192.168.1.100-192.168.1.109", "192.168.1.0/24", "192.168.1.1")
	if alloc.TotalIPs() != 10 {
		t.Errorf("expected 10 IPs, got %d", alloc.TotalIPs())
	}
}
