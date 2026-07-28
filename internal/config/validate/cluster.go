package validate

import (
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/nexusops/proxmox-k3s/internal/config"
	"github.com/nexusops/proxmox-k3s/internal/domain"
)

type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("[%s] %s", e.Field, e.Message)
}

type ValidationErrors []ValidationError

func (ve ValidationErrors) Error() string {
	msgs := make([]string, len(ve))
	for i, e := range ve {
		msgs[i] = "  • " + e.Error()
	}
	return "config validation failed:\n" + strings.Join(msgs, "\n")
}

func Cluster(cfg *config.Config) error {
	var errs ValidationErrors

	if cfg.ClusterName == "" {
		errs = append(errs, ValidationError{"cluster_name", "required"})
	}
	if cfg.Proxmox.Endpoint == "" {
		errs = append(errs, ValidationError{"proxmox.endpoint", "required"})
	}
	if cfg.Proxmox.TokenID == "" {
		errs = append(errs, ValidationError{"proxmox.token_id", "required"})
	}
	if cfg.Proxmox.TokenSecret == "" {
		errs = append(errs, ValidationError{"proxmox.token_secret", "required (use ${ENV_VAR})"})
	} else if strings.HasPrefix(cfg.Proxmox.TokenSecret, "${") {
		errs = append(errs, ValidationError{"proxmox.token_secret", "environment variable not set: " + cfg.Proxmox.TokenSecret})
	}
	if cfg.Proxmox.Template == "" {
		errs = append(errs, ValidationError{"proxmox.template", "required"})
	}
	if cfg.Proxmox.Storage == "" {
		errs = append(errs, ValidationError{"proxmox.storage", "required"})
	}

	if cfg.Networking.ControlPlaneVIP == "" {
		errs = append(errs, ValidationError{"networking.control_plane_vip", "required for HA clusters"})
	} else if net.ParseIP(cfg.Networking.ControlPlaneVIP) == nil {
		errs = append(errs, ValidationError{"networking.control_plane_vip", "invalid IP address"})
	}

	if cfg.Networking.Mode == "static" {
		if cfg.Networking.CIDR == "" {
			errs = append(errs, ValidationError{"networking.cidr", "required for static mode"})
		} else {
			_, _, err := net.ParseCIDR(cfg.Networking.CIDR)
			if err != nil {
				errs = append(errs, ValidationError{"networking.cidr", "invalid CIDR: " + err.Error()})
			}
		}
		if cfg.Networking.Gateway == "" {
			errs = append(errs, ValidationError{"networking.gateway", "required for static mode"})
		}
		if cfg.Networking.NodePoolRange == "" {
			errs = append(errs, ValidationError{"networking.node_pool_range", "required for static mode"})
		}
	}

	if cfg.Networking.SSH.PublicKeyPath == "" {
		errs = append(errs, ValidationError{"networking.ssh.public_key_path", "required"})
	} else if _, err := os.ReadFile(cfg.Networking.SSH.PublicKeyPath); err != nil {
		errs = append(errs, ValidationError{"networking.ssh.public_key_path", "cannot read file: " + err.Error()})
	}

	if cfg.Networking.SSH.PrivateKeyPath == "" {
		errs = append(errs, ValidationError{"networking.ssh.private_key_path", "required"})
	} else if _, err := os.ReadFile(cfg.Networking.SSH.PrivateKeyPath); err != nil {
		errs = append(errs, ValidationError{"networking.ssh.private_key_path", "cannot read file: " + err.Error()})
	}

	if cfg.MastersPool.Count < 3 || cfg.MastersPool.Count%2 == 0 {
		errs = append(errs, ValidationError{"masters_pool.count", domain.ErrOddMasterCount(cfg.MastersPool.Count).Message})
	}

	if err := validatePool("masters_pool", cfg.MastersPool, domain.RoleMaster); err != nil {
		errs = append(errs, err...)
	}
	for i, pool := range cfg.WorkerPools {
		field := fmt.Sprintf("worker_node_pools[%d]", i)
		if pool.Name == "" {
			errs = append(errs, ValidationError{field + ".name", "required"})
		}
		if err := validatePool(field, pool, domain.RoleWorker); err != nil {
			errs = append(errs, err...)
		}
	}

	if cfg.Addons.MetalLB.Enabled && cfg.Addons.MetalLB.AddressPool == "" {
		errs = append(errs, ValidationError{"addons.metallb.address_pool", "required when metallb is enabled"})
	}

	if len(errs) > 0 {
		return errs
	}
	return nil
}

func validatePool(field string, pool config.PoolConfig, role domain.NodeRole) ValidationErrors {
	var errs ValidationErrors

	if pool.Count <= 0 {
		errs = append(errs, ValidationError{field + ".count", "must be > 0"})
		return errs
	}

	var cpuOverride, memOverride, diskOverride int
	if pool.Resources != nil {
		cpuOverride = pool.Resources.CPU
		memOverride = pool.Resources.MemoryMB
		diskOverride = pool.Resources.DiskGB
	}

	size, err := domain.ResolveSize(pool.Size, cpuOverride, memOverride, diskOverride)
	if err != nil {
		errs = append(errs, ValidationError{field + ".size", err.Error()})
		return errs
	}

	if err := domain.ValidateSize(size, role); err != nil {
		errs = append(errs, ValidationError{field + ".size", err.Error()})
	}

	return errs
}
