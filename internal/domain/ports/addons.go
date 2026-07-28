package ports

import (
	"context"

	"github.com/nexusops/proxmox-k3s/internal/domain"
)

type AddonInstaller interface {
	InstallKubeVIP(ctx context.Context, masters []domain.Node, vip string) error
	InstallMetalLB(ctx context.Context, master domain.Node, addressPool string) error
}
