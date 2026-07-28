package ports

import (
	"context"

	"github.com/nexusops/proxmox-k3s/internal/domain"
)

type IPAllocator interface {
	Allocate(ctx context.Context, pool string, index int) (string, error)
	Discover(ctx context.Context, node domain.Node) (string, error)
	Release(ctx context.Context, ip string) error
}
