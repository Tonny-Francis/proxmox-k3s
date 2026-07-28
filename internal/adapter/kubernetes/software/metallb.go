package software

import (
	"context"
	"fmt"
	"strings"

	"github.com/nexusops/proxmox-k3s/internal/adapter/k3s"
	"github.com/nexusops/proxmox-k3s/internal/domain"
)

const metalLBVersion = "v0.14.8"

const metalLBManifestURL = "https://raw.githubusercontent.com/metallb/metallb/" + metalLBVersion + "/config/manifests/metallb-native.yaml"

const metalLBPoolTemplate = `apiVersion: metallb.io/v1beta1
kind: IPAddressPool
metadata:
  name: default-pool
  namespace: metallb-system
spec:
  addresses:
  - {{ .AddressPool }}
---
apiVersion: metallb.io/v1beta1
kind: L2Advertisement
metadata:
  name: default
  namespace: metallb-system
spec:
  ipAddressPools:
  - default-pool
`

type MetalLBInstaller struct {
	k3si *k3s.Installer
}

func NewMetalLBInstaller(k3si *k3s.Installer) *MetalLBInstaller {
	return &MetalLBInstaller{k3si: k3si}
}

func (m *MetalLBInstaller) Install(ctx context.Context, master domain.Node, addressPool string) error {
	installCmd := fmt.Sprintf(
		"kubectl apply -f %s && kubectl -n metallb-system wait --for=condition=Available deployment/controller --timeout=120s",
		metalLBManifestURL,
	)

	if _, _, err := m.k3si.GetExecutor().Run(ctx, master.IP(), installCmd); err != nil {
		return fmt.Errorf("installing MetalLB: %w", err)
	}

	poolManifest := strings.ReplaceAll(metalLBPoolTemplate, "{{ .AddressPool }}", addressPool)
	return m.k3si.ApplyManifest(ctx, master, []byte(poolManifest))
}
