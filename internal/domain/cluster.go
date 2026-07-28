package domain

type Cluster struct {
	Name    string
	Masters NodePool
	Workers []NodePool
	Network NetworkPlan
	K3s     K3sConfig
	Addons  AddonsConfig
	Proxmox ProxmoxConfig
}

type K3sConfig struct {
	Version      string
	CNI          string
	ClusterCIDR  string
	ServiceCIDR  string
	DisableItems []string
}

type AddonsConfig struct {
	KubeVIP  KubeVIPConfig
	MetalLB  MetalLBConfig
}

type KubeVIPConfig struct {
	Enabled bool
}

type MetalLBConfig struct {
	Enabled     bool
	AddressPool string
}

type ProxmoxConfig struct {
	Endpoint             string
	TokenID              string
	TokenSecret          string
	InsecureSkipTLS      bool
	CAFile               string
	TargetNodes          []string
	Template             string
	Storage              string
	SnippetsStorage      string
	PlacementStrategy    string
	VMIDRange            [2]int
}
