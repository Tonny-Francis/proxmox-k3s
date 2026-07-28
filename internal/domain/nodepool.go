package domain

type NodePool struct {
	Name        string
	Count       int
	Size        VMSize
	Labels      map[string]string
	Taints      []string
	Autoscaling *AutoscalingConfig
}

type AutoscalingConfig struct {
	Enabled bool
	Min     int
	Max     int
}

type NodeSpec struct {
	Name        string
	ClusterName string
	Role        NodeRole
	Pool        string
	Size        VMSize
	HostNode    string
	DesiredIP   string
	Labels      map[string]string
	Taints      []string
	CloudInit   CloudInitSpec
}

type CloudInitSpec struct {
	User        string
	SSHKeys     []string
	Nameservers []string
	SearchDomain string
	NetworkMode  IPMode
	IPConfig    string
	Gateway     string
}
