package ports

import (
	"context"

	"github.com/nexusops/proxmox-k3s/internal/domain"
)

type K3sInstaller interface {
	ResolveVersion(ctx context.Context, versionOrChannel string) (string, error)
	InstallFirstMaster(ctx context.Context, node domain.Node, cluster *domain.Cluster) error
	InstallMaster(ctx context.Context, node domain.Node, cluster *domain.Cluster, token string) error
	InstallWorker(ctx context.Context, node domain.Node, cluster *domain.Cluster, token string) error
	FetchToken(ctx context.Context, node domain.Node) (string, error)
	FetchKubeconfig(ctx context.Context, node domain.Node) ([]byte, error)
	ApplyManifest(ctx context.Context, node domain.Node, manifest []byte) error
}
