package node

import (
	"context"
	"fmt"

	"github.com/nexusops/proxmox-k3s/internal/adapter/proxmox/client"
)

type PVENode struct {
	Name   string  `json:"node"`
	Status string  `json:"status"`
	CPU    float64 `json:"cpu"`
	MaxCPU int     `json:"maxcpu"`
	Mem    int64   `json:"mem"`
	MaxMem int64   `json:"maxmem"`
	Disk   int64   `json:"disk"`
	MaxDisk int64  `json:"maxdisk"`
}

type ClusterResource struct {
	VMID    int    `json:"vmid"`
	Name    string `json:"name"`
	Node    string `json:"node"`
	Type    string `json:"type"`
	Status  string `json:"status"`
	Tags    string `json:"tags"`
	Template int   `json:"template"`
}

type Lister struct {
	c *client.Client
}

func NewLister(c *client.Client) *Lister {
	return &Lister{c: c}
}

func (l *Lister) ListPVENodes(ctx context.Context) ([]PVENode, error) {
	type resp struct {
		Nodes []PVENode
	}
	return client.GetJSON[[]PVENode](ctx, l.c, "/api2/json/nodes")
}

func (l *Lister) ListResources(ctx context.Context, resType string) ([]ClusterResource, error) {
	path := "/api2/json/cluster/resources"
	if resType != "" {
		path += "?type=" + resType
	}
	return client.GetJSON[[]ClusterResource](ctx, l.c, path)
}

func (l *Lister) FindTemplate(ctx context.Context, name string) (*ClusterResource, error) {
	resources, err := l.ListResources(ctx, "vm")
	if err != nil {
		return nil, err
	}
	for _, r := range resources {
		if r.Name == name && r.Template == 1 {
			return &r, nil
		}
	}
	return nil, fmt.Errorf("template %q not found", name)
}
