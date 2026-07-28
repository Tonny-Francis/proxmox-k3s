package delete

import (
	"context"
	"fmt"
	"io"

	proxmoxadapter "github.com/nexusops/proxmox-k3s/internal/adapter/proxmox"
	"github.com/nexusops/proxmox-k3s/internal/domain"
)

type Pipeline struct {
	provider *proxmoxadapter.Provider
	out      io.Writer
}

func NewPipeline(provider *proxmoxadapter.Provider, out io.Writer) *Pipeline {
	return &Pipeline{provider: provider, out: out}
}

func (p *Pipeline) Run(ctx context.Context, cluster *domain.Cluster) error {
	p.log("==> Buscando nós do cluster %q...", cluster.Name)
	nodes, err := p.provider.ListNodes(ctx, cluster.Name)
	if err != nil {
		return fmt.Errorf("listing nodes: %w", err)
	}

	if len(nodes) == 0 {
		p.log("    Nenhum nó encontrado para o cluster %q — nada a remover", cluster.Name)
		return nil
	}

	p.log("    %d nós encontrados:", len(nodes))
	for _, n := range nodes {
		p.log("      %s (VMID %d, %s, host: %s)", n.Name, n.VMID, n.Role, n.HostNode)
	}

	p.log("==> Removendo nós...")
	var failed []string

	for _, n := range nodes {
		p.log("    Removendo %s (VMID %d)...", n.Name, n.VMID)
		if err := p.provider.DeleteNode(ctx, n); err != nil {
			p.log("    ERRO: %s: %v", n.Name, err)
			failed = append(failed, n.Name)
		} else {
			p.log("    %s removido", n.Name)
		}
	}

	if len(failed) > 0 {
		return fmt.Errorf("failed to delete nodes: %v", failed)
	}

	p.log("Cluster %q removido. Nenhuma VM com tag k3s-cluster=%s restante.", cluster.Name, cluster.Name)
	return nil
}

func (p *Pipeline) log(format string, args ...any) {
	fmt.Fprintf(p.out, format+"\n", args...)
}
