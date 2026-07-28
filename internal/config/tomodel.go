package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/nexusops/proxmox-k3s/internal/domain"
)

func ToModel(cfg *Config) (*domain.Cluster, error) {
	masters, err := poolToModel("master", cfg.MastersPool, domain.RoleMaster)
	if err != nil {
		return nil, fmt.Errorf("masters_pool: %w", err)
	}

	workers := make([]domain.NodePool, 0, len(cfg.WorkerPools))
	for i, pool := range cfg.WorkerPools {
		w, err := poolToModel(pool.Name, pool, domain.RoleWorker)
		if err != nil {
			return nil, fmt.Errorf("worker_node_pools[%d]: %w", i, err)
		}
		workers = append(workers, w)
	}

	pubKey, err := readFile(cfg.Networking.SSH.PublicKeyPath)
	if err != nil {
		return nil, fmt.Errorf("reading public key: %w", err)
	}

	vmidRange := [2]int{}
	if len(cfg.Proxmox.VMIDRange) == 2 {
		vmidRange[0] = cfg.Proxmox.VMIDRange[0]
		vmidRange[1] = cfg.Proxmox.VMIDRange[1]
	}

	return &domain.Cluster{
		Name:    cfg.ClusterName,
		Masters: masters,
		Workers: workers,
		Network: domain.NetworkPlan{
			Mode:            domain.IPMode(cfg.Networking.Mode),
			Bridge:          cfg.Networking.Bridge,
			VlanTag:         cfg.Networking.VlanTag,
			CIDR:            cfg.Networking.CIDR,
			Gateway:         cfg.Networking.Gateway,
			Nameservers:     cfg.Networking.Nameservers,
			NodePoolRange:   cfg.Networking.NodePoolRange,
			ControlPlaneVIP: cfg.Networking.ControlPlaneVIP,
			SSH: domain.SSHConfig{
				PublicKeyPath:  cfg.Networking.SSH.PublicKeyPath,
				PrivateKeyPath: cfg.Networking.SSH.PrivateKeyPath,
				Port:           cfg.Networking.SSH.Port,
				PublicKey:      strings.TrimSpace(string(pubKey)),
			},
		},
		K3s: domain.K3sConfig{
			Version:      cfg.K3s.Version,
			CNI:          cfg.K3s.CNI,
			ClusterCIDR:  cfg.K3s.ClusterCIDR,
			ServiceCIDR:  cfg.K3s.ServiceCIDR,
			DisableItems: cfg.K3s.Disable,
		},
		Addons: domain.AddonsConfig{
			KubeVIP: domain.KubeVIPConfig{
				Enabled: cfg.Addons.KubeVIP.Enabled,
			},
			MetalLB: domain.MetalLBConfig{
				Enabled:     cfg.Addons.MetalLB.Enabled,
				AddressPool: cfg.Addons.MetalLB.AddressPool,
			},
		},
		Proxmox: domain.ProxmoxConfig{
			Endpoint:          cfg.Proxmox.Endpoint,
			TokenID:           cfg.Proxmox.TokenID,
			TokenSecret:       cfg.Proxmox.TokenSecret,
			InsecureSkipTLS:   cfg.Proxmox.InsecureSkipTLS,
			CAFile:            cfg.Proxmox.CAFile,
			TargetNodes:       cfg.Proxmox.TargetNodes,
			Template:          cfg.Proxmox.Template,
			Storage:           cfg.Proxmox.Storage,
			SnippetsStorage:   cfg.Proxmox.SnippetsStorage,
			PlacementStrategy: cfg.Proxmox.PlacementStrategy,
			VMIDRange:         vmidRange,
		},
	}, nil
}

func poolToModel(name string, pool PoolConfig, role domain.NodeRole) (domain.NodePool, error) {
	var cpuOvr, memOvr, diskOvr int
	if pool.Resources != nil {
		cpuOvr = pool.Resources.CPU
		memOvr = pool.Resources.MemoryMB
		diskOvr = pool.Resources.DiskGB
	}

	size, err := domain.ResolveSize(pool.Size, cpuOvr, memOvr, diskOvr)
	if err != nil {
		return domain.NodePool{}, err
	}

	var autoscaling *domain.AutoscalingConfig
	if pool.Autoscaling != nil {
		autoscaling = &domain.AutoscalingConfig{
			Enabled: pool.Autoscaling.Enabled,
			Min:     pool.Autoscaling.Min,
			Max:     pool.Autoscaling.Max,
		}
	}

	return domain.NodePool{
		Name:        name,
		Count:       pool.Count,
		Size:        size,
		Labels:      pool.Labels,
		Taints:      pool.Taints,
		Autoscaling: autoscaling,
	}, nil
}

func readFile(path string) ([]byte, error) {
	if path == "" {
		return nil, nil
	}
	return os.ReadFile(path)
}
