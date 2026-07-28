package proxmox_test

import (
	"context"
	"fmt"
	"testing"

	proxmoxadapter "github.com/nexusops/proxmox-k3s/internal/adapter/proxmox"
	"github.com/nexusops/proxmox-k3s/internal/adapter/proxmox/testserver"
	"github.com/nexusops/proxmox-k3s/internal/domain"
)

func newTestProvider(t *testing.T) (*proxmoxadapter.Provider, *testserver.Server) {
	t.Helper()
	srv := testserver.New()
	t.Cleanup(srv.Close)

	cfg := domain.ProxmoxConfig{
		Endpoint:          srv.URL(),
		TokenID:           "test@pve!test",
		TokenSecret:       "fake-secret",
		InsecureSkipTLS:   true,
		Template:          "ubuntu-2404-cloudinit",
		Storage:           "local-lvm",
		PlacementStrategy: "round-robin",
		TargetNodes:       []string{"pve1"},
	}

	provider, err := proxmoxadapter.NewProvider(cfg)
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}
	return provider, srv
}

func testCluster(name string) *domain.Cluster {
	return &domain.Cluster{
		Name: name,
		Proxmox: domain.ProxmoxConfig{
			Template:    "ubuntu-2404-cloudinit",
			TargetNodes: []string{"pve1"},
		},
		Network: domain.NetworkPlan{
			ControlPlaneVIP: "192.168.20.10",
			Mode:            domain.IPModeStatic,
		},
	}
}

func TestPreflight_Success(t *testing.T) {
	provider, _ := newTestProvider(t)
	cluster := testCluster("test-cluster")

	if err := provider.Preflight(context.Background(), cluster); err != nil {
		t.Errorf("Preflight failed unexpectedly: %v", err)
	}
}

func TestPreflight_MissingTemplate(t *testing.T) {
	provider, _ := newTestProvider(t)
	cluster := testCluster("test-cluster")
	cluster.Proxmox.Template = "nonexistent-template"

	if err := provider.Preflight(context.Background(), cluster); err == nil {
		t.Error("expected error for missing template, got nil")
	}
}

func TestPreflight_OfflineNode(t *testing.T) {
	provider, srv := newTestProvider(t)
	srv.AddNode(testserver.FakeNode{Name: "pve2", Status: "offline"})

	cluster := testCluster("test-cluster")
	cluster.Proxmox.TargetNodes = []string{"pve2"}

	if err := provider.Preflight(context.Background(), cluster); err == nil {
		t.Error("expected error for offline node, got nil")
	}
}

func TestListNodes_Empty(t *testing.T) {
	provider, _ := newTestProvider(t)

	nodes, err := provider.ListNodes(context.Background(), "my-cluster")
	if err != nil {
		t.Fatalf("ListNodes: %v", err)
	}
	if len(nodes) != 0 {
		t.Errorf("expected 0 nodes, got %d", len(nodes))
	}
}

func TestCreateNode_CreatesAndTracksVM(t *testing.T) {
	provider, srv := newTestProvider(t)

	spec := domain.NodeSpec{
		Name:        "homelab-master-1",
		ClusterName: "homelab",
		Role:        domain.RoleMaster,
		Pool:        "master",
		Size:        domain.VMSize{Name: "cp-medium", CPU: 4, MemoryMB: 8192, DiskGB: 60},
		CloudInit: domain.CloudInitSpec{
			User:        "ubuntu",
			SSHKeys:     []string{"ssh-ed25519 AAAAC3NzaC1 test"},
			NetworkMode: domain.IPModeStatic,
			IPConfig:    "ip=192.168.20.50/24,gw=192.168.20.1",
		},
	}

	node, err := provider.CreateNode(context.Background(), spec)
	if err != nil {
		t.Fatalf("CreateNode: %v", err)
	}

	if node.Name != "homelab-master-1" {
		t.Errorf("unexpected node name: %q", node.Name)
	}
	if node.VMID == 0 {
		t.Error("VMID should not be zero after creation")
	}
	if node.HostNode == "" {
		t.Error("HostNode should not be empty")
	}
	if node.Role != domain.RoleMaster {
		t.Errorf("expected role master, got %q", node.Role)
	}
	if node.State != domain.StateProvisioned {
		t.Errorf("expected state provisioned, got %q", node.State)
	}

	// Server-side: VM should exist
	if srv.VMCount() != 1 {
		t.Errorf("expected 1 VM in fake server, got %d", srv.VMCount())
	}
	vms := srv.VMs()
	if vms[0].Status != "running" {
		t.Errorf("VM should be running after create, got %q", vms[0].Status)
	}
}

func TestCreateNode_Operations(t *testing.T) {
	provider, srv := newTestProvider(t)

	spec := domain.NodeSpec{
		Name:        "homelab-worker-1",
		ClusterName: "homelab",
		Role:        domain.RoleWorker,
		Pool:        "general",
		Size:        domain.VMSize{Name: "standard-4", CPU: 4, MemoryMB: 8192, DiskGB: 100},
		CloudInit:   domain.CloudInitSpec{User: "ubuntu", NetworkMode: domain.IPModeDHCP},
	}

	_, err := provider.CreateNode(context.Background(), spec)
	if err != nil {
		t.Fatalf("CreateNode: %v", err)
	}

	// Verify the sequence of operations was recorded
	ops := srv.Ops
	assertContains(t, ops, "clone", "clone must happen")
	assertContains(t, ops, "resize", "disk resize must happen")
	assertContains(t, ops, "start", "VM must be started")
}

func TestCreateNode_CloneFailure_RollsBack(t *testing.T) {
	provider, srv := newTestProvider(t)
	srv.Errors.Set("clone", fmt.Errorf("storage is locked"))

	spec := domain.NodeSpec{
		Name:        "fail-node",
		ClusterName: "homelab",
		Role:        domain.RoleWorker,
		Pool:        "general",
		Size:        domain.VMSize{CPU: 2, MemoryMB: 4096, DiskGB: 40},
		CloudInit:   domain.CloudInitSpec{User: "ubuntu"},
	}

	_, err := provider.CreateNode(context.Background(), spec)
	if err == nil {
		t.Error("expected error from clone failure, got nil")
	}

	if srv.VMCount() != 0 {
		t.Errorf("no VMs should exist after failed clone, got %d", srv.VMCount())
	}
}

func TestCreateMultipleNodes_DifferentVMIDs(t *testing.T) {
	provider, srv := newTestProvider(t)

	vmids := make(map[int]bool)
	for i := 0; i < 3; i++ {
		spec := domain.NodeSpec{
			Name:        fmt.Sprintf("homelab-master-%d", i+1),
			ClusterName: "homelab",
			Role:        domain.RoleMaster,
			Pool:        "master",
			Size:        domain.VMSize{CPU: 4, MemoryMB: 8192, DiskGB: 60},
			CloudInit:   domain.CloudInitSpec{User: "ubuntu", NetworkMode: domain.IPModeStatic},
		}
		node, err := provider.CreateNode(context.Background(), spec)
		if err != nil {
			t.Fatalf("CreateNode %d: %v", i, err)
		}
		if vmids[node.VMID] {
			t.Errorf("VMID %d was reused", node.VMID)
		}
		vmids[node.VMID] = true
	}

	if srv.VMCount() != 3 {
		t.Errorf("expected 3 VMs, got %d", srv.VMCount())
	}
}

func TestDeleteNode_RemovesVM(t *testing.T) {
	provider, srv := newTestProvider(t)

	spec := domain.NodeSpec{
		Name:        "homelab-master-1",
		ClusterName: "homelab",
		Role:        domain.RoleMaster,
		Pool:        "master",
		Size:        domain.VMSize{CPU: 4, MemoryMB: 8192, DiskGB: 60},
		CloudInit:   domain.CloudInitSpec{User: "ubuntu"},
	}

	node, err := provider.CreateNode(context.Background(), spec)
	if err != nil {
		t.Fatalf("CreateNode: %v", err)
	}
	if srv.VMCount() != 1 {
		t.Fatalf("setup: expected 1 VM, got %d", srv.VMCount())
	}

	if err := provider.DeleteNode(context.Background(), node); err != nil {
		t.Fatalf("DeleteNode: %v", err)
	}

	if srv.VMCount() != 0 {
		t.Errorf("expected 0 VMs after delete, got %d", srv.VMCount())
	}
}

func TestListNodes_AfterTaggedCreate(t *testing.T) {
	provider, _ := newTestProvider(t)
	cluster := testCluster("homelab")

	// Create VMs — they get tagged with k3s-cluster=homelab
	for i := 0; i < 2; i++ {
		_, err := provider.CreateNode(context.Background(), domain.NodeSpec{
			Name:        fmt.Sprintf("homelab-master-%d", i+1),
			ClusterName: "homelab",
			Role:        domain.RoleMaster,
			Pool:        "master",
			Size:        domain.VMSize{CPU: 4, MemoryMB: 8192, DiskGB: 60},
			CloudInit:   domain.CloudInitSpec{User: "ubuntu"},
		})
		if err != nil {
			t.Fatalf("CreateNode %d: %v", i, err)
		}
	}

	// ListNodes should find them by tag
	nodes, err := provider.ListNodes(context.Background(), cluster.Name)
	if err != nil {
		t.Fatalf("ListNodes: %v", err)
	}
	if len(nodes) != 2 {
		t.Errorf("expected 2 nodes, got %d", len(nodes))
	}
	for _, n := range nodes {
		if n.Role != domain.RoleMaster {
			t.Errorf("expected master role, got %q", n.Role)
		}
		if n.Pool != "master" {
			t.Errorf("expected pool 'master', got %q", n.Pool)
		}
	}
}

func TestWaitReady_ReturnsObservedIP(t *testing.T) {
	provider, _ := newTestProvider(t)

	spec := domain.NodeSpec{
		Name:        "homelab-master-1",
		ClusterName: "homelab",
		Role:        domain.RoleMaster,
		Pool:        "master",
		Size:        domain.VMSize{CPU: 4, MemoryMB: 8192, DiskGB: 60},
		CloudInit:   domain.CloudInitSpec{User: "ubuntu"},
	}

	node, err := provider.CreateNode(context.Background(), spec)
	if err != nil {
		t.Fatalf("CreateNode: %v", err)
	}

	ready, err := provider.WaitReady(context.Background(), node)
	if err != nil {
		t.Fatalf("WaitReady: %v", err)
	}

	if ready.ObservedIP == "" {
		t.Error("expected ObservedIP to be set after WaitReady")
	}
	if ready.IP() == "" {
		t.Error("IP() should return a non-empty value")
	}
}

func assertContains(t *testing.T, ops []string, want, msg string) {
	t.Helper()
	for _, op := range ops {
		if op == want {
			return
		}
	}
	t.Errorf("%s: operation %q not found in %v", msg, want, ops)
}
