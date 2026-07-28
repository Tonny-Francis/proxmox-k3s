package config

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

var envPattern = regexp.MustCompile(`\$\{([A-Z0-9_]+)\}`)

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config %q: %w", path, err)
	}

	expanded := expandEnv(string(data))

	var cfg Config
	if err := yaml.Unmarshal([]byte(expanded), &cfg); err != nil {
		return nil, fmt.Errorf("parsing config %q: %w", path, err)
	}

	applyDefaults(&cfg)

	return &cfg, nil
}

func expandEnv(s string) string {
	return envPattern.ReplaceAllStringFunc(s, func(match string) string {
		sub := envPattern.FindStringSubmatch(match)
		if len(sub) < 2 {
			return match
		}
		val := os.Getenv(sub[1])
		if val == "" {
			return match
		}
		return val
	})
}

func applyDefaults(cfg *Config) {
	if cfg.Provider == "" {
		cfg.Provider = "proxmox"
	}
	if cfg.Proxmox.PlacementStrategy == "" {
		cfg.Proxmox.PlacementStrategy = "round-robin"
	}
	if cfg.Networking.Mode == "" {
		cfg.Networking.Mode = "static"
	}
	if cfg.Networking.Bridge == "" {
		cfg.Networking.Bridge = "vmbr0"
	}
	if cfg.Networking.SSH.Port == 0 {
		cfg.Networking.SSH.Port = 22
	}
	if cfg.K3s.CNI == "" {
		cfg.K3s.CNI = "flannel"
	}
	if cfg.K3s.ClusterCIDR == "" {
		cfg.K3s.ClusterCIDR = "10.42.0.0/16"
	}
	if cfg.K3s.ServiceCIDR == "" {
		cfg.K3s.ServiceCIDR = "10.43.0.0/16"
	}
	if cfg.K3s.Version == "" {
		cfg.K3s.Version = "stable"
	}
	if cfg.MastersPool.Name == "" {
		cfg.MastersPool.Name = "master"
	}
	if cfg.MastersPool.Count == 0 {
		cfg.MastersPool.Count = 3
	}
	if cfg.MastersPool.Size == "" && cfg.MastersPool.Resources == nil {
		cfg.MastersPool.Size = "cp-medium"
	}
	for i := range cfg.WorkerPools {
		if cfg.WorkerPools[i].Size == "" && cfg.WorkerPools[i].Resources == nil {
			cfg.WorkerPools[i].Size = "standard-4"
		}
	}

	cfg.Networking.SSH.PublicKeyPath = expandHome(cfg.Networking.SSH.PublicKeyPath)
	cfg.Networking.SSH.PrivateKeyPath = expandHome(cfg.Networking.SSH.PrivateKeyPath)
}

func expandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, _ := os.UserHomeDir()
		return home + path[1:]
	}
	return path
}
