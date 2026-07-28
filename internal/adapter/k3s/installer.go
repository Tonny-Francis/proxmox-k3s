package k3s

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"io"
	"strings"
	"text/template"

	"github.com/nexusops/proxmox-k3s/internal/adapter/ssh"
	"github.com/nexusops/proxmox-k3s/internal/domain"
)

//go:embed scripts/master_first.sh.tmpl
var masterFirstScript string

//go:embed scripts/master_join.sh.tmpl
var masterJoinScript string

//go:embed scripts/worker_join.sh.tmpl
var workerJoinScript string

type masterScriptData struct {
	Version     string
	VIP         string
	ClusterCIDR string
	ServiceCIDR string
	CNI         string
	Token       string
	SANs        []string
	ExtraArgs   []string
}

type workerScriptData struct {
	Version    string
	VIP        string
	Token      string
	NodeLabels []string
	NodeTaints []string
	ExtraArgs  []string
}

type Installer struct {
	exec *ssh.Executor
	out  io.Writer
}

func NewInstaller(exec *ssh.Executor, out io.Writer) *Installer {
	return &Installer{exec: exec, out: out}
}

func (i *Installer) InstallFirstMaster(ctx context.Context, n domain.Node, cluster *domain.Cluster) error {
	var sans []string
	sans = append(sans, n.IP())

	data := masterScriptData{
		Version:     cluster.K3s.Version,
		VIP:         cluster.Network.ControlPlaneVIP,
		ClusterCIDR: cluster.K3s.ClusterCIDR,
		ServiceCIDR: cluster.K3s.ServiceCIDR,
		CNI:         cluster.K3s.CNI,
		Token:       clusterToken(cluster.Name),
		SANs:        sans,
	}

	script, err := renderTemplate(masterFirstScript, data)
	if err != nil {
		return fmt.Errorf("rendering master-first script: %w", err)
	}

	return i.runScript(ctx, n.IP(), "master-first", script)
}

func (i *Installer) InstallMaster(ctx context.Context, n domain.Node, cluster *domain.Cluster, token string) error {
	data := masterScriptData{
		Version:     cluster.K3s.Version,
		VIP:         cluster.Network.ControlPlaneVIP,
		ClusterCIDR: cluster.K3s.ClusterCIDR,
		ServiceCIDR: cluster.K3s.ServiceCIDR,
		CNI:         cluster.K3s.CNI,
		Token:       token,
		SANs:        []string{n.IP()},
	}

	script, err := renderTemplate(masterJoinScript, data)
	if err != nil {
		return fmt.Errorf("rendering master-join script: %w", err)
	}

	return i.runScript(ctx, n.IP(), "master-join", script)
}

func (i *Installer) InstallWorker(ctx context.Context, n domain.Node, cluster *domain.Cluster, token string) error {
	labels := make([]string, 0, len(n.Labels))
	for k, v := range n.Labels {
		labels = append(labels, k+"="+v)
	}

	data := workerScriptData{
		Version:    cluster.K3s.Version,
		VIP:        cluster.Network.ControlPlaneVIP,
		Token:      token,
		NodeLabels: labels,
		NodeTaints: n.Taints,
	}

	script, err := renderTemplate(workerJoinScript, data)
	if err != nil {
		return fmt.Errorf("rendering worker-join script: %w", err)
	}

	return i.runScript(ctx, n.IP(), "worker-join", script)
}

func (i *Installer) FetchToken(ctx context.Context, n domain.Node) (string, error) {
	stdout, _, err := i.exec.Run(ctx, n.IP(), "sudo cat /var/lib/rancher/k3s/server/node-token")
	if err != nil {
		return "", fmt.Errorf("fetching K3s token from %s: %w", n.Name, err)
	}
	return strings.TrimSpace(stdout), nil
}

func (i *Installer) FetchKubeconfig(ctx context.Context, n domain.Node) ([]byte, error) {
	stdout, _, err := i.exec.Run(ctx, n.IP(), "sudo cat /etc/rancher/k3s/k3s.yaml")
	if err != nil {
		return nil, fmt.Errorf("fetching kubeconfig from %s: %w", n.Name, err)
	}
	return []byte(stdout), nil
}

func (i *Installer) ApplyManifest(ctx context.Context, n domain.Node, manifest []byte) error {
	remotePath := "/tmp/proxmox-k3s-manifest.yaml"
	if err := i.exec.Upload(ctx, n.IP(), remotePath, manifest, 0644); err != nil {
		return fmt.Errorf("uploading manifest to %s: %w", n.Name, err)
	}

	_, _, err := i.exec.Run(ctx, n.IP(), "kubectl apply -f "+remotePath)
	return err
}

func (i *Installer) GetExecutor() *ssh.Executor {
	return i.exec
}

func (i *Installer) WaitNodesReady(ctx context.Context, n domain.Node, count int) error {
	cmd := fmt.Sprintf(
		`until [ "$(kubectl get nodes --no-headers 2>/dev/null | grep -c ' Ready')" -ge %d ]; do sleep 5; done`,
		count,
	)
	_, _, err := i.exec.Run(ctx, n.IP(), cmd)
	return err
}

func (i *Installer) runScript(ctx context.Context, host, name string, script []byte) error {
	remotePath := fmt.Sprintf("/tmp/proxmox-k3s-%s.sh", name)
	if err := i.exec.Upload(ctx, host, remotePath, script, 0755); err != nil {
		return fmt.Errorf("uploading %s script: %w", name, err)
	}

	w := prefixedWriter(i.out, host)
	if err := i.exec.RunStream(ctx, host, "sudo bash "+remotePath, w); err != nil {
		return domain.ErrK3sInstall(host, err)
	}
	return nil
}

func renderTemplate(tmpl string, data any) ([]byte, error) {
	t, err := template.New("").Parse(tmpl)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func clusterToken(clusterName string) string {
	return "proxmox-k3s-" + clusterName
}

type prefixWriter struct {
	w      io.Writer
	prefix string
}

func prefixedWriter(w io.Writer, prefix string) io.Writer {
	return &prefixWriter{w: w, prefix: "[" + prefix + "] "}
}

func (p *prefixWriter) Write(b []byte) (int, error) {
	lines := strings.Split(string(b), "\n")
	var out strings.Builder
	for _, line := range lines {
		if line != "" {
			out.WriteString(p.prefix + line + "\n")
		}
	}
	_, err := p.w.Write([]byte(out.String()))
	return len(b), err
}
