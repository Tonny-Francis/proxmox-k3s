package client_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/nexusops/proxmox-k3s/internal/adapter/proxmox/client"
	"github.com/nexusops/proxmox-k3s/internal/adapter/proxmox/testserver"
)

func newTestClient(t *testing.T) (*client.Client, *testserver.Server) {
	t.Helper()
	srv := testserver.New()
	t.Cleanup(srv.Close)

	c, err := client.New(client.Options{
		Endpoint:        srv.URL(),
		TokenID:         "test@pve!test",
		TokenSecret:     "fake-secret",
		InsecureSkipTLS: true,
	})
	if err != nil {
		t.Fatalf("creating client: %v", err)
	}
	return c, srv
}

func TestVersion(t *testing.T) {
	c, _ := newTestClient(t)
	v, err := c.Version(context.Background())
	if err != nil {
		t.Fatalf("Version: %v", err)
	}
	if v["version"] != "8.2.0" {
		t.Errorf("expected version 8.2.0, got %v", v["version"])
	}
}

func TestNextVMID_ReturnsIncrementing(t *testing.T) {
	c, _ := newTestClient(t)

	id1, err := c.NextVMID(context.Background(), 0, 0)
	if err != nil {
		t.Fatalf("first NextVMID: %v", err)
	}
	id2, err := c.NextVMID(context.Background(), 0, 0)
	if err != nil {
		t.Fatalf("second NextVMID: %v", err)
	}

	if id1 == id2 {
		t.Errorf("expected different VMIDs, got %d twice", id1)
	}
	if id1 <= 0 || id2 <= 0 {
		t.Errorf("VMIDs must be positive, got %d and %d", id1, id2)
	}
}

func TestNextVMID_RangeExceeded(t *testing.T) {
	c, _ := newTestClient(t)

	// The fake server starts at VMID 101; ask for a max of 50 → should fail
	_, err := c.NextVMID(context.Background(), 0, 50)
	if err == nil {
		t.Error("expected error when VMID exceeds max range, got nil")
	}
}

func TestWaitTask_ImmediateCompletion(t *testing.T) {
	c, srv := newTestClient(t)

	// Trigger a clone to get a real UPID registered in the fake server
	type cloneReq struct {
		NewID int    `json:"newid"`
		Name  string `json:"name"`
		Full  int    `json:"full"`
	}
	upid, err := client.PostJSON[string](context.Background(), c,
		"/api2/json/nodes/pve1/qemu/9000/clone",
		cloneReq{NewID: 200, Name: "test-vm", Full: 1})
	if err != nil {
		t.Fatalf("clone: %v", err)
	}
	if upid == "" {
		t.Fatal("expected non-empty UPID")
	}

	// Task polling should complete immediately (fake server returns stopped+OK)
	if err := c.WaitTask(context.Background(), "pve1", upid); err != nil {
		t.Errorf("WaitTask: %v", err)
	}

	// VM should be tracked
	if srv.VMCount() != 1 {
		t.Errorf("expected 1 VM after clone, got %d", srv.VMCount())
	}
}

func TestClone_CreatesVM(t *testing.T) {
	c, srv := newTestClient(t)

	type cloneReq struct {
		NewID int    `json:"newid"`
		Name  string `json:"name"`
		Full  int    `json:"full"`
	}
	_, err := client.PostJSON[string](context.Background(), c,
		"/api2/json/nodes/pve1/qemu/9000/clone",
		cloneReq{NewID: 300, Name: "homelab-master-1", Full: 1})
	if err != nil {
		t.Fatalf("clone: %v", err)
	}

	vms := srv.VMs()
	if len(vms) != 1 {
		t.Fatalf("expected 1 VM, got %d", len(vms))
	}
	if vms[0].Name != "homelab-master-1" {
		t.Errorf("expected name 'homelab-master-1', got %q", vms[0].Name)
	}
	if vms[0].VMID != 300 {
		t.Errorf("expected VMID 300, got %d", vms[0].VMID)
	}
}

func TestClone_ErrorInjection(t *testing.T) {
	c, srv := newTestClient(t)
	srv.Errors.Set("clone", fmt.Errorf("storage locked"))

	type cloneReq struct {
		NewID int    `json:"newid"`
		Name  string `json:"name"`
		Full  int    `json:"full"`
	}
	_, err := client.PostJSON[string](context.Background(), c,
		"/api2/json/nodes/pve1/qemu/9000/clone",
		cloneReq{NewID: 400, Name: "fail-vm", Full: 1})
	if err == nil {
		t.Error("expected error from injected clone failure, got nil")
	}

	if srv.VMCount() != 0 {
		t.Error("VM should not exist after failed clone")
	}
}
