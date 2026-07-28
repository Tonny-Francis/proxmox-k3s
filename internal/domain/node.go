package domain

type NodeRole string

const (
	RoleMaster NodeRole = "master"
	RoleWorker NodeRole = "worker"
)

type NodeState string

const (
	StatePending     NodeState = "pending"
	StateProvisioned NodeState = "provisioned"
	StateJoined      NodeState = "joined"
	StateOrphan      NodeState = "orphan"
	StateUnhealthy   NodeState = "unhealthy"
)

type Node struct {
	Name       string
	Role       NodeRole
	Pool       string
	ProviderID string
	VMID       int
	HostNode   string
	DesiredIP  string
	ObservedIP string
	State      NodeState
	Labels     map[string]string
	Taints     []string
}

func (n Node) IP() string {
	if n.ObservedIP != "" {
		return n.ObservedIP
	}
	return n.DesiredIP
}
