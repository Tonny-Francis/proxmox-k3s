package proxmox

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/nexusops/proxmox-k3s/internal/adapter/proxmox/client"
	"github.com/nexusops/proxmox-k3s/internal/adapter/proxmox/node"
	"github.com/nexusops/proxmox-k3s/internal/adapter/proxmox/placement"
	"github.com/nexusops/proxmox-k3s/internal/adapter/proxmox/vm"
	"github.com/nexusops/proxmox-k3s/internal/domain"
	"github.com/nexusops/proxmox-k3s/internal/domain/ports"
)

const (
	tagCluster = "k3s-cluster"
	tagRole    = "k3s-role"
	tagPool    = "k3s-pool"
)

type Provider struct {
	client      *client.Client
	vmMgr       *vm.Manager
	nodeLister  *node.Lister
	strategy    placement.Strategy
	cloneSem    map[string]chan struct{}
	semMu       sync.Mutex
	maxClones   int
	templateName string
	storageName  string
}

var _ ports.InfraProvider = (*Provider)(nil)

func NewProvider(cfg domain.ProxmoxConfig) (*Provider, error) {
	c, err := client.New(client.Options{
		Endpoint:        cfg.Endpoint,
		TokenID:         cfg.TokenID,
		TokenSecret:     cfg.TokenSecret,
		InsecureSkipTLS: cfg.InsecureSkipTLS,
		CAFile:          cfg.CAFile,
	})
	if err != nil {
		return nil, fmt.Errorf("creating Proxmox client: %w", err)
	}

	return &Provider{
		client:       c,
		vmMgr:        vm.NewManager(c),
		nodeLister:   node.NewLister(c),
		strategy:     placement.NewStrategy(cfg.PlacementStrategy),
		cloneSem:     make(map[string]chan struct{}),
		maxClones:    2,
		templateName: cfg.Template,
		storageName:  cfg.Storage,
	}, nil
}

func (p *Provider) Name() string { return "proxmox" }

func (p *Provider) Capabilities() ports.Capabilities {
	return ports.Capabilities{
		SupportsManagedLB: false,
		SupportsSnapshot:  true,
	}
}

func (p *Provider) Preflight(ctx context.Context, cluster *domain.Cluster) error {
	if _, err := p.client.Version(ctx); err != nil {
		return fmt.Errorf("connecting to Proxmox at %s: %w", cluster.Proxmox.Endpoint, err)
	}

	pveNodes, err := p.nodeLister.ListPVENodes(ctx)
	if err != nil {
		return fmt.Errorf("listing Proxmox nodes: %w", err)
	}

	online := map[string]bool{}
	for _, n := range pveNodes {
		if n.Status == "online" {
			online[n.Name] = true
		}
	}

	for _, target := range cluster.Proxmox.TargetNodes {
		if !online[target] {
			return domain.ErrNodeNotFound(target)
		}
	}

	if _, err := p.nodeLister.FindTemplate(ctx, cluster.Proxmox.Template); err != nil {
		return domain.ErrTemplateNotFound(cluster.Proxmox.Template)
	}

	return nil
}

func (p *Provider) ListNodes(ctx context.Context, clusterName string) ([]domain.Node, error) {
	resources, err := p.nodeLister.ListResources(ctx, "vm")
	if err != nil {
		return nil, err
	}

	tag := tagCluster + "=" + clusterName
	var nodes []domain.Node

	for _, r := range resources {
		if r.Template == 1 {
			continue
		}
		if !hasTag(r.Tags, tag) {
			continue
		}

		role := domain.RoleWorker
		if hasTag(r.Tags, tagRole+"=master") {
			role = domain.RoleMaster
		}

		pool := extractTag(r.Tags, tagPool)

		nodes = append(nodes, domain.Node{
			Name:       r.Name,
			Role:       role,
			Pool:       pool,
			ProviderID: fmt.Sprintf("proxmox://%s/%d", r.Node, r.VMID),
			VMID:       r.VMID,
			HostNode:   r.Node,
			State:      domain.StateProvisioned,
		})
	}

	return nodes, nil
}

func (p *Provider) CreateNode(ctx context.Context, spec domain.NodeSpec) (domain.Node, error) {
	pveNodes, err := p.nodeLister.ListPVENodes(ctx)
	if err != nil {
		return domain.Node{}, err
	}

	var exclude []string
	hostNode := spec.HostNode
	if hostNode == "" {
		hostNode, err = p.strategy.Pick(pveNodes, exclude)
		if err != nil {
			return domain.Node{}, err
		}
	}

	tmpl, err := p.nodeLister.FindTemplate(ctx, p.templateName)
	if err != nil {
		return domain.Node{}, err
	}

	vmid, err := p.client.NextVMID(ctx, 0, 0)
	if err != nil {
		return domain.Node{}, err
	}

	sem := p.cloneSemaphore(hostNode)
	sem <- struct{}{}
	defer func() { <-sem }()

	upid, err := p.vmMgr.Clone(ctx, tmpl.Node, tmpl.VMID, vm.CloneRequest{
		NewID:      vmid,
		Name:       spec.Name,
		Full:       1,
		Storage:    p.storageName,
		TargetNode: hostNode,
	})
	if err != nil {
		return domain.Node{}, fmt.Errorf("cloning template for %s: %w", spec.Name, err)
	}
	if err := p.client.WaitTask(ctx, tmpl.Node, upid); err != nil {
		return domain.Node{}, fmt.Errorf("waiting for clone of %s: %w", spec.Name, err)
	}

	tags := buildTags(spec.ClusterName, spec)
	cfgReq := vm.ConfigRequest{
		Cores:   spec.Size.CPU,
		Memory:  spec.Size.MemoryMB,
		Agent:   "enabled=1",
		OnBoot:  1,
		Tags:    tags,
		CIUser:  spec.CloudInit.User,
		SSHKeys: encodeSSHKeys(spec.CloudInit.SSHKeys),
	}

	if len(spec.CloudInit.Nameservers) > 0 {
		cfgReq.Nameserver = strings.Join(spec.CloudInit.Nameservers, " ")
	}
	if spec.CloudInit.NetworkMode == domain.IPModeStatic && spec.CloudInit.IPConfig != "" {
		cfgReq.IPConfig0 = spec.CloudInit.IPConfig
	} else {
		cfgReq.IPConfig0 = "ip=dhcp"
	}

	bridge := "vmbr0"
	if spec.CloudInit.Gateway != "" {
		bridge = "vmbr0"
	}
	cfgReq.Net0 = fmt.Sprintf("virtio,bridge=%s", bridge)

	upid, err = p.vmMgr.Configure(ctx, hostNode, vmid, cfgReq)
	if err != nil {
		return domain.Node{}, fmt.Errorf("configuring %s: %w", spec.Name, err)
	}
	if upid != "" {
		if err := p.client.WaitTask(ctx, hostNode, upid); err != nil {
			return domain.Node{}, fmt.Errorf("waiting for config of %s: %w", spec.Name, err)
		}
	}

	diskSize := fmt.Sprintf("%dG", spec.Size.DiskGB)
	upid, err = p.vmMgr.Resize(ctx, hostNode, vmid, "scsi0", diskSize)
	if err != nil {
		return domain.Node{}, fmt.Errorf("resizing disk for %s: %w", spec.Name, err)
	}
	if upid != "" {
		if err := p.client.WaitTask(ctx, hostNode, upid); err != nil {
			return domain.Node{}, fmt.Errorf("waiting for disk resize of %s: %w", spec.Name, err)
		}
	}

	upid, err = p.vmMgr.Start(ctx, hostNode, vmid)
	if err != nil {
		return domain.Node{}, fmt.Errorf("starting %s: %w", spec.Name, err)
	}
	if upid != "" {
		if err := p.client.WaitTask(ctx, hostNode, upid); err != nil {
			return domain.Node{}, fmt.Errorf("waiting for start of %s: %w", spec.Name, err)
		}
	}

	return domain.Node{
		Name:       spec.Name,
		Role:       spec.Role,
		Pool:       spec.Pool,
		ProviderID: fmt.Sprintf("proxmox://%s/%d", hostNode, vmid),
		VMID:       vmid,
		HostNode:   hostNode,
		DesiredIP:  spec.DesiredIP,
		State:      domain.StateProvisioned,
		Labels:     spec.Labels,
		Taints:     spec.Taints,
	}, nil
}

func (p *Provider) WaitReady(ctx context.Context, n domain.Node) (domain.Node, error) {
	ifaces, err := p.waitAgentReady(ctx, n)
	if err != nil {
		return n, err
	}

	ip := p.vmMgr.FindGuestIP(ifaces)
	if ip != "" {
		n.ObservedIP = ip
	}

	return n, nil
}

func (p *Provider) waitAgentReady(ctx context.Context, n domain.Node) ([]vm.AgentNetworkInterface, error) {
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		ifaces, err := p.vmMgr.AgentNetworkInterfaces(ctx, n.HostNode, n.VMID)
		if err == nil {
			return ifaces, nil
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(5 * time.Second):
		}
	}
}

func (p *Provider) DeleteNode(ctx context.Context, n domain.Node) error {
	upid, err := p.vmMgr.Stop(ctx, n.HostNode, n.VMID)
	if err != nil && !strings.Contains(err.Error(), "not running") {
		return fmt.Errorf("stopping %s: %w", n.Name, err)
	}
	if upid != "" {
		_ = p.client.WaitTask(ctx, n.HostNode, upid)
	}

	upid, err = p.vmMgr.Delete(ctx, n.HostNode, n.VMID)
	if err != nil {
		return fmt.Errorf("deleting %s (VMID %d): %w", n.Name, n.VMID, err)
	}
	if upid != "" {
		if err := p.client.WaitTask(ctx, n.HostNode, upid); err != nil {
			return fmt.Errorf("waiting for delete of %s: %w", n.Name, err)
		}
	}
	return nil
}

func (p *Provider) cloneSemaphore(hostNode string) chan struct{} {
	p.semMu.Lock()
	defer p.semMu.Unlock()

	if _, ok := p.cloneSem[hostNode]; !ok {
		p.cloneSem[hostNode] = make(chan struct{}, p.maxClones)
	}
	return p.cloneSem[hostNode]
}

func buildTags(clusterName string, spec domain.NodeSpec) string {
	parts := []string{
		tagCluster + "=" + clusterName,
		tagRole + "=" + string(spec.Role),
		tagPool + "=" + spec.Pool,
	}
	return strings.Join(parts, ";")
}

func encodeSSHKeys(keys []string) string {
	return strings.Join(keys, "\n")
}

func hasTag(tags, tag string) bool {
	for _, t := range strings.Split(tags, ";") {
		if strings.TrimSpace(t) == tag {
			return true
		}
	}
	return false
}

func extractTag(tags, prefix string) string {
	for _, t := range strings.Split(tags, ";") {
		t = strings.TrimSpace(t)
		if strings.HasPrefix(t, prefix+"=") {
			return strings.TrimPrefix(t, prefix+"=")
		}
	}
	return ""
}
