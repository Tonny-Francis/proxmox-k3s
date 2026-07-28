package vm

import (
	"context"
	"fmt"
	"strings"

	"github.com/nexusops/proxmox-k3s/internal/adapter/proxmox/client"
)

type CloneRequest struct {
	NewID    int    `json:"newid"`
	Name     string `json:"name"`
	Full     int    `json:"full"`
	Storage  string `json:"storage"`
	TargetNode string `json:"target,omitempty"`
}

type ConfigRequest struct {
	Cores    int    `json:"cores,omitempty"`
	Memory   int    `json:"memory,omitempty"`
	Agent    string `json:"agent,omitempty"`
	OnBoot   int    `json:"onboot,omitempty"`
	Tags     string `json:"tags,omitempty"`
	IPConfig0 string `json:"ipconfig0,omitempty"`
	CIUser   string `json:"ciuser,omitempty"`
	SSHKeys  string `json:"sshkeys,omitempty"`
	Nameserver string `json:"nameserver,omitempty"`
	SearchDomain string `json:"searchdomain,omitempty"`
	Net0     string `json:"net0,omitempty"`
}

type ResizeRequest struct {
	Disk string `json:"disk"`
	Size string `json:"size"`
}

type AgentNetworkInterface struct {
	Name            string                  `json:"name"`
	IPAddresses     []AgentIPAddress        `json:"ip-addresses"`
	HardwareAddress string                  `json:"hardware-address"`
}

type AgentIPAddress struct {
	IPAddress     string `json:"ip-address"`
	IPAddressType string `json:"ip-address-type"`
	Prefix        int    `json:"prefix"`
}

type Manager struct {
	c *client.Client
}

func NewManager(c *client.Client) *Manager {
	return &Manager{c: c}
}

func (m *Manager) Clone(ctx context.Context, pveNode string, templateVMID int, req CloneRequest) (string, error) {
	path := fmt.Sprintf("/api2/json/nodes/%s/qemu/%d/clone", pveNode, templateVMID)
	upid, err := client.PostJSON[string](ctx, m.c, path, req)
	if err != nil {
		return "", fmt.Errorf("clone VMID %d: %w", templateVMID, err)
	}
	return upid, nil
}

func (m *Manager) Configure(ctx context.Context, pveNode string, vmid int, req ConfigRequest) (string, error) {
	path := fmt.Sprintf("/api2/json/nodes/%s/qemu/%d/config", pveNode, vmid)
	upid, err := client.PostJSON[string](ctx, m.c, path, req)
	if err != nil {
		return "", fmt.Errorf("configure VMID %d: %w", vmid, err)
	}
	return upid, nil
}

func (m *Manager) Resize(ctx context.Context, pveNode string, vmid int, disk, size string) (string, error) {
	path := fmt.Sprintf("/api2/json/nodes/%s/qemu/%d/resize", pveNode, vmid)
	upid, err := client.PutJSON[string](ctx, m.c, path, ResizeRequest{Disk: disk, Size: size})
	if err != nil {
		return "", fmt.Errorf("resize disk on VMID %d: %w", vmid, err)
	}
	return upid, nil
}

func (m *Manager) Start(ctx context.Context, pveNode string, vmid int) (string, error) {
	path := fmt.Sprintf("/api2/json/nodes/%s/qemu/%d/status/start", pveNode, vmid)
	upid, err := client.PostJSON[string](ctx, m.c, path, nil)
	if err != nil {
		return "", fmt.Errorf("start VMID %d: %w", vmid, err)
	}
	return upid, nil
}

func (m *Manager) Stop(ctx context.Context, pveNode string, vmid int) (string, error) {
	path := fmt.Sprintf("/api2/json/nodes/%s/qemu/%d/status/stop", pveNode, vmid)
	upid, err := client.PostJSON[string](ctx, m.c, path, nil)
	if err != nil {
		return "", fmt.Errorf("stop VMID %d: %w", vmid, err)
	}
	return upid, nil
}

func (m *Manager) Delete(ctx context.Context, pveNode string, vmid int) (string, error) {
	path := fmt.Sprintf("/api2/json/nodes/%s/qemu/%d?purge=1&destroy-unreferenced-disks=1", pveNode, vmid)
	upid, err := client.DeleteJSON(ctx, m.c, path)
	if err != nil {
		return "", fmt.Errorf("delete VMID %d: %w", vmid, err)
	}
	return upid, nil
}

func (m *Manager) AgentNetworkInterfaces(ctx context.Context, pveNode string, vmid int) ([]AgentNetworkInterface, error) {
	path := fmt.Sprintf("/api2/json/nodes/%s/qemu/%d/agent/network-get-interfaces", pveNode, vmid)
	type result struct {
		Result []AgentNetworkInterface `json:"result"`
	}
	r, err := client.GetJSON[result](ctx, m.c, path)
	if err != nil {
		return nil, fmt.Errorf("agent network-get-interfaces on VMID %d: %w", vmid, err)
	}
	return r.Result, nil
}

func (m *Manager) FindGuestIP(interfaces []AgentNetworkInterface) string {
	for _, iface := range interfaces {
		if iface.Name == "lo" || strings.HasPrefix(iface.Name, "cni") || strings.HasPrefix(iface.Name, "flannel") {
			continue
		}
		for _, addr := range iface.IPAddresses {
			if addr.IPAddressType == "ipv4" && addr.IPAddress != "127.0.0.1" {
				return addr.IPAddress
			}
		}
	}
	return ""
}

func (m *Manager) UploadSnippet(ctx context.Context, pveNode, storage, name, content string) error {
	path := fmt.Sprintf("/api2/json/nodes/%s/storage/%s/upload", pveNode, storage)
	_, err := client.PostJSON[string](ctx, m.c, path, map[string]string{
		"content":  "snippets",
		"filename": name,
		"content-body": content,
	})
	return err
}
