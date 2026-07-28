package create

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/nexusops/proxmox-k3s/internal/adapter/ipam"
	"github.com/nexusops/proxmox-k3s/internal/adapter/k3s"
	"github.com/nexusops/proxmox-k3s/internal/adapter/kubernetes/software"
	"github.com/nexusops/proxmox-k3s/internal/adapter/ssh"
	proxmoxadapter "github.com/nexusops/proxmox-k3s/internal/adapter/proxmox"
	"github.com/nexusops/proxmox-k3s/internal/domain"
)

type Pipeline struct {
	provider  *proxmoxadapter.Provider
	out       io.Writer
}

func NewPipeline(provider *proxmoxadapter.Provider, out io.Writer) *Pipeline {
	return &Pipeline{provider: provider, out: out}
}

func (p *Pipeline) Run(ctx context.Context, cluster *domain.Cluster) error {
	p.log("==> Fase 1: Preflight")
	if err := p.provider.Preflight(ctx, cluster); err != nil {
		return fmt.Errorf("preflight: %w", err)
	}
	p.log("    Proxmox OK, template encontrado, hosts online")

	p.log("==> Fase 2: Reconcile")
	existing, err := p.provider.ListNodes(ctx, cluster.Name)
	if err != nil {
		return fmt.Errorf("listing existing nodes: %w", err)
	}
	p.log("    %d nós existentes encontrados", len(existing))

	p.log("==> Fase 3: Alocação de IPs")
	ipPlan, err := p.planIPs(cluster, existing)
	if err != nil {
		return fmt.Errorf("IP allocation: %w", err)
	}

	p.log("==> Fase 4: Provisionamento de VMs")
	created, err := p.provisionNodes(ctx, cluster, ipPlan, existing)
	if err != nil {
		return fmt.Errorf("provisioning: %w", err)
	}

	if len(created) == 0 && len(existing) > 0 {
		p.log("    Nenhuma VM nova — cluster já existe")
	} else {
		p.log("    %d VMs criadas", len(created))
	}

	allNodes := append(existing, created...)
	masters, workers := splitByRole(allNodes)

	p.log("==> Fase 5: Aguardando VMs ficarem prontas")
	masters, err = p.waitAllReady(ctx, masters)
	if err != nil {
		return fmt.Errorf("waiting for masters: %w", err)
	}
	workers, err = p.waitAllReady(ctx, workers)
	if err != nil {
		return fmt.Errorf("waiting for workers: %w", err)
	}

	sshExec, err := ssh.NewExecutor(
		cluster.Network.SSH.PrivateKeyPath,
		"ubuntu",
		cluster.Network.SSH.Port,
	)
	if err != nil {
		return fmt.Errorf("creating SSH executor: %w", err)
	}

	k3sInstaller := k3s.NewInstaller(sshExec, p.out)
	resolver := k3s.NewResolver()

	p.log("==> Resolvendo versão do K3s: %s", cluster.K3s.Version)
	version, err := resolver.Resolve(ctx, cluster.K3s.Version)
	if err != nil {
		return fmt.Errorf("resolving K3s version: %w", err)
	}
	cluster.K3s.Version = version
	p.log("    Versão resolvida: %s", version)

	p.log("==> Fase 6: Bootstrap do control plane")
	token, err := p.bootstrapControlPlane(ctx, cluster, masters, k3sInstaller)
	if err != nil {
		return fmt.Errorf("bootstrapping control plane: %w", err)
	}

	p.log("==> Fase 7: Workers")
	if err := p.joinWorkers(ctx, cluster, workers, token, k3sInstaller); err != nil {
		return fmt.Errorf("joining workers: %w", err)
	}

	p.log("==> Fase 8: Addons")
	if err := p.installAddons(ctx, cluster, masters, sshExec, k3sInstaller); err != nil {
		return fmt.Errorf("installing addons: %w", err)
	}

	p.log("==> Fase 9: Kubeconfig")
	if err := p.writeKubeconfig(ctx, cluster, masters[0], k3sInstaller); err != nil {
		return fmt.Errorf("writing kubeconfig: %w", err)
	}

	p.log("")
	p.log("Cluster %q pronto!", cluster.Name)
	p.log("  VIP: https://%s:6443", cluster.Network.ControlPlaneVIP)
	p.log("  Masters: %d  Workers: %d", len(masters), len(workers))
	p.log("  kubectl get nodes -o wide")

	return nil
}

func (p *Pipeline) planIPs(cluster *domain.Cluster, existing []domain.Node) (map[string]string, error) {
	if cluster.Network.Mode == domain.IPModeDHCP {
		return nil, nil
	}

	alloc, err := ipam.NewStaticAllocator(
		cluster.Network.NodePoolRange,
		cluster.Network.CIDR,
		cluster.Network.Gateway,
	)
	if err != nil {
		return nil, err
	}

	totalNeeded := cluster.Masters.Count
	for _, pool := range cluster.Workers {
		totalNeeded += pool.Count
	}

	if alloc.TotalIPs() < totalNeeded {
		return nil, domain.ErrIPPoolExhausted(cluster.Network.NodePoolRange)
	}

	existingIPs := map[string]bool{}
	for _, n := range existing {
		if n.DesiredIP != "" {
			existingIPs[n.DesiredIP] = true
		}
	}

	plan := make(map[string]string)
	idx := 0

	for i := 0; i < cluster.Masters.Count; i++ {
		name := fmt.Sprintf("%s-%s-%d", cluster.Name, cluster.Masters.Name, i+1)
		ipCfg, err := alloc.IPConfigForIndex(idx)
		if err != nil {
			return nil, err
		}
		ip, _ := alloc.IPForIndex(idx)
		plan[name] = ipCfg
		_ = ip
		idx++
	}

	for _, pool := range cluster.Workers {
		for i := 0; i < pool.Count; i++ {
			name := fmt.Sprintf("%s-%s-%d", cluster.Name, pool.Name, i+1)
			ipCfg, err := alloc.IPConfigForIndex(idx)
			if err != nil {
				return nil, err
			}
			plan[name] = ipCfg
			idx++
		}
	}

	return plan, nil
}

func (p *Pipeline) provisionNodes(ctx context.Context, cluster *domain.Cluster, ipPlan map[string]string, existing []domain.Node) ([]domain.Node, error) {
	existingNames := map[string]bool{}
	for _, n := range existing {
		existingNames[n.Name] = true
	}

	var toCreate []domain.NodeSpec

	for i := 0; i < cluster.Masters.Count; i++ {
		name := fmt.Sprintf("%s-%s-%d", cluster.Name, cluster.Masters.Name, i+1)
		if existingNames[name] {
			continue
		}

		spec := domain.NodeSpec{
			Name:        name,
			ClusterName: cluster.Name,
			Role:        domain.RoleMaster,
			Pool:        cluster.Masters.Name,
			Size:        cluster.Masters.Size,
			Labels:      cluster.Masters.Labels,
			Taints:      cluster.Masters.Taints,
			CloudInit: domain.CloudInitSpec{
				User:        "ubuntu",
				SSHKeys:     []string{cluster.Network.SSH.PublicKey},
				Nameservers: cluster.Network.Nameservers,
				NetworkMode: cluster.Network.Mode,
				IPConfig:    ipPlan[name],
				Gateway:     cluster.Network.Gateway,
			},
		}
		toCreate = append(toCreate, spec)
	}

	for _, pool := range cluster.Workers {
		for i := 0; i < pool.Count; i++ {
			name := fmt.Sprintf("%s-%s-%d", cluster.Name, pool.Name, i+1)
			if existingNames[name] {
				continue
			}

			spec := domain.NodeSpec{
				Name:        name,
				ClusterName: cluster.Name,
				Role:        domain.RoleWorker,
				Pool:        pool.Name,
				Size:        pool.Size,
				Labels:      pool.Labels,
				Taints:      pool.Taints,
				CloudInit: domain.CloudInitSpec{
					User:        "ubuntu",
					SSHKeys:     []string{cluster.Network.SSH.PublicKey},
					Nameservers: cluster.Network.Nameservers,
					NetworkMode: cluster.Network.Mode,
					IPConfig:    ipPlan[name],
					Gateway:     cluster.Network.Gateway,
				},
			}
			toCreate = append(toCreate, spec)
		}
	}

	if len(toCreate) == 0 {
		return nil, nil
	}

	var created []domain.Node
	var rollback []domain.Node

	for _, spec := range toCreate {
		p.log("    Criando %s (%s, %d CPU, %d MB)...", spec.Name, spec.Size.Name, spec.Size.CPU, spec.Size.MemoryMB)
		n, err := p.provider.CreateNode(ctx, spec)
		if err != nil {
			p.rollback(context.Background(), rollback)
			return nil, fmt.Errorf("creating %s: %w", spec.Name, err)
		}
		created = append(created, n)
		rollback = append(rollback, n)
		p.log("    %s criado (VMID %d em %s)", n.Name, n.VMID, n.HostNode)
	}

	return created, nil
}

func (p *Pipeline) waitAllReady(ctx context.Context, nodes []domain.Node) ([]domain.Node, error) {
	timeout := 10 * time.Minute
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var ready []domain.Node
	for _, n := range nodes {
		p.log("    Aguardando %s ficar pronto...", n.Name)
		readyNode, err := p.provider.WaitReady(ctx, n)
		if err != nil {
			return nil, fmt.Errorf("waiting for %s: %w", n.Name, err)
		}
		p.log("    %s pronto — IP: %s", readyNode.Name, readyNode.IP())
		ready = append(ready, readyNode)
	}
	return ready, nil
}

func (p *Pipeline) bootstrapControlPlane(ctx context.Context, cluster *domain.Cluster, masters []domain.Node, inst *k3s.Installer) (string, error) {
	first := masters[0]
	alreadyRunning := false

	if out, _, err := inst.GetExecutor().Run(ctx, first.IP(), "systemctl is-active k3s"); err == nil {
		if out == "active\n" {
			alreadyRunning = true
			p.log("    K3s já rodando em %s — buscando token", first.Name)
		}
	}

	if !alreadyRunning {
		p.log("    Instalando K3s em %s (primeiro master, cluster-init)...", first.Name)
		if err := inst.InstallFirstMaster(ctx, first, cluster); err != nil {
			return "", err
		}
	}

	token, err := inst.FetchToken(ctx, first)
	if err != nil {
		return "", fmt.Errorf("fetching cluster token: %w", err)
	}

	for _, master := range masters[1:] {
		out, _, err := inst.GetExecutor().Run(ctx, master.IP(), "systemctl is-active k3s")
		if err == nil && out == "active\n" {
			p.log("    K3s já rodando em %s", master.Name)
			continue
		}

		p.log("    Instalando K3s em %s (join)...", master.Name)
		if err := inst.InstallMaster(ctx, master, cluster, token); err != nil {
			return token, fmt.Errorf("joining master %s: %w", master.Name, err)
		}
	}

	p.log("    Aguardando %d masters Ready...", len(masters))
	if err := inst.WaitNodesReady(ctx, first, len(masters)); err != nil {
		return token, fmt.Errorf("waiting for masters Ready: %w", err)
	}

	return token, nil
}

func (p *Pipeline) joinWorkers(ctx context.Context, cluster *domain.Cluster, workers []domain.Node, token string, inst *k3s.Installer) error {
	for _, worker := range workers {
		out, _, err := inst.GetExecutor().Run(ctx, worker.IP(), "systemctl is-active k3s-agent")
		if err == nil && out == "active\n" {
			p.log("    K3s-agent já rodando em %s", worker.Name)
			continue
		}

		p.log("    Instalando K3s agent em %s...", worker.Name)
		if err := inst.InstallWorker(ctx, worker, cluster, token); err != nil {
			p.log("    AVISO: falha em %s: %v — continuando", worker.Name, err)
		}
	}
	return nil
}

func (p *Pipeline) installAddons(ctx context.Context, cluster *domain.Cluster, masters []domain.Node, exec *ssh.Executor, k3sInst *k3s.Installer) error {
	if cluster.Addons.KubeVIP.Enabled {
		p.log("    Instalando kube-vip...")
		kvInstaller := software.NewKubeVIPInstaller(exec, k3sInst)
		if err := kvInstaller.Install(ctx, masters, cluster.Network.ControlPlaneVIP); err != nil {
			p.log("    AVISO: kube-vip: %v", err)
		}
	}

	if cluster.Addons.MetalLB.Enabled {
		p.log("    Instalando MetalLB (pool: %s)...", cluster.Addons.MetalLB.AddressPool)
		mlbInstaller := software.NewMetalLBInstaller(k3sInst)
		if err := mlbInstaller.Install(ctx, masters[0], cluster.Addons.MetalLB.AddressPool); err != nil {
			p.log("    AVISO: MetalLB: %v", err)
		}
	}

	return nil
}

func (p *Pipeline) writeKubeconfig(ctx context.Context, cluster *domain.Cluster, master domain.Node, inst *k3s.Installer) error {
	raw, err := inst.FetchKubeconfig(ctx, master)
	if err != nil {
		return err
	}

	rewritten, err := k3s.RewriteKubeconfig(raw, cluster.Name, cluster.Network.ControlPlaneVIP)
	if err != nil {
		return err
	}

	if err := k3s.MergeKubeconfig(rewritten, ""); err != nil {
		return err
	}

	p.log("    Kubeconfig merged → ~/.kube/config (contexto: %s)", cluster.Name)
	return nil
}

func (p *Pipeline) rollback(ctx context.Context, nodes []domain.Node) {
	if len(nodes) == 0 {
		return
	}
	p.log("==> Rollback: removendo %d VMs criadas nesta execução...", len(nodes))
	for _, n := range nodes {
		if err := p.provider.DeleteNode(ctx, n); err != nil {
			p.log("    AVISO: falha ao remover %s (VMID %d): %v", n.Name, n.VMID, err)
		} else {
			p.log("    %s removido", n.Name)
		}
	}
}

func (p *Pipeline) log(format string, args ...any) {
	fmt.Fprintf(p.out, format+"\n", args...)
}

func splitByRole(nodes []domain.Node) (masters, workers []domain.Node) {
	for _, n := range nodes {
		if n.Role == domain.RoleMaster {
			masters = append(masters, n)
		} else {
			workers = append(workers, n)
		}
	}
	return
}
