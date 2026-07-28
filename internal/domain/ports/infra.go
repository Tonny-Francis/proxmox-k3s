package ports

import (
	"context"

	"github.com/nexusops/proxmox-k3s/internal/domain"
)

type Capabilities struct {
	SupportsManagedLB bool
	SupportsSnapshot  bool
}

type InfraProvider interface {
	Name() string
	Capabilities() Capabilities
	Preflight(ctx context.Context, cluster *domain.Cluster) error
	ListNodes(ctx context.Context, clusterName string) ([]domain.Node, error)
	CreateNode(ctx context.Context, spec domain.NodeSpec) (domain.Node, error)
	DeleteNode(ctx context.Context, node domain.Node) error
	WaitReady(ctx context.Context, node domain.Node) (domain.Node, error)
}
