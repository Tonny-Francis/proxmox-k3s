package config

type Config struct {
	ClusterName string        `yaml:"cluster_name"`
	Provider    string        `yaml:"provider"`
	Proxmox     ProxmoxConfig `yaml:"proxmox"`
	Networking  NetworkConfig `yaml:"networking"`
	K3s         K3sConfig     `yaml:"k3s"`
	MastersPool PoolConfig    `yaml:"masters_pool"`
	WorkerPools []PoolConfig  `yaml:"worker_node_pools"`
	Addons      AddonsConfig  `yaml:"addons"`
}

type ProxmoxConfig struct {
	Endpoint          string `yaml:"endpoint"`
	TokenID           string `yaml:"token_id"`
	TokenSecret       string `yaml:"token_secret"`
	InsecureSkipTLS   bool   `yaml:"insecure_skip_tls_verify"`
	CAFile            string `yaml:"ca_file"`
	TargetNodes       []string `yaml:"target_nodes"`
	Template          string `yaml:"template"`
	Storage           string `yaml:"storage"`
	SnippetsStorage   string `yaml:"snippets_storage"`
	PlacementStrategy string `yaml:"placement_strategy"`
	VMIDRange         []int  `yaml:"vmid_range"`
}

type NetworkConfig struct {
	Mode            string    `yaml:"mode"`
	Bridge          string    `yaml:"bridge"`
	VlanTag         int       `yaml:"vlan_tag"`
	CIDR            string    `yaml:"cidr"`
	Gateway         string    `yaml:"gateway"`
	Nameservers     []string  `yaml:"nameservers"`
	NodePoolRange   string    `yaml:"node_pool_range"`
	ControlPlaneVIP string    `yaml:"control_plane_vip"`
	SSH             SSHConfig `yaml:"ssh"`
}

type SSHConfig struct {
	PublicKeyPath  string `yaml:"public_key_path"`
	PrivateKeyPath string `yaml:"private_key_path"`
	Port           int    `yaml:"port"`
}

type K3sConfig struct {
	Version     string   `yaml:"version"`
	CNI         string   `yaml:"cni"`
	ClusterCIDR string   `yaml:"cluster_cidr"`
	ServiceCIDR string   `yaml:"service_cidr"`
	Disable     []string `yaml:"disable"`
}

type PoolConfig struct {
	Name        string            `yaml:"name"`
	Count       int               `yaml:"count"`
	Size        string            `yaml:"size"`
	Resources   *ResourceOverride `yaml:"resources"`
	Labels      map[string]string `yaml:"labels"`
	Taints      []string          `yaml:"taints"`
	Autoscaling *AutoscalingConfig `yaml:"autoscaling"`
}

type ResourceOverride struct {
	CPU      int `yaml:"cpu"`
	MemoryMB int `yaml:"memory_mb"`
	DiskGB   int `yaml:"disk_gb"`
}

type AutoscalingConfig struct {
	Enabled bool `yaml:"enabled"`
	Min     int  `yaml:"min"`
	Max     int  `yaml:"max"`
}

type AddonsConfig struct {
	KubeVIP  KubeVIPConfig  `yaml:"kube_vip"`
	MetalLB  MetalLBConfig  `yaml:"metallb"`
}

type KubeVIPConfig struct {
	Enabled bool `yaml:"enabled"`
}

type MetalLBConfig struct {
	Enabled     bool   `yaml:"enabled"`
	AddressPool string `yaml:"address_pool"`
}
