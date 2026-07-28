package software

import (
	"bytes"
	"context"
	"fmt"
	"text/template"

	"github.com/nexusops/proxmox-k3s/internal/adapter/k3s"
	"github.com/nexusops/proxmox-k3s/internal/adapter/ssh"
	"github.com/nexusops/proxmox-k3s/internal/domain"
)

const kubeVIPVersion = "v0.8.2"

const kubeVIPManifest = `apiVersion: v1
kind: Pod
metadata:
  name: kube-vip
  namespace: kube-system
spec:
  containers:
  - name: kube-vip
    image: ghcr.io/kube-vip/kube-vip:{{ .Version }}
    args:
    - manager
    env:
    - name: vip_arp
      value: "true"
    - name: port
      value: "6443"
    - name: vip_interface
      value: "{{ .Interface }}"
    - name: vip_cidr
      value: "32"
    - name: cp_enable
      value: "true"
    - name: cp_namespace
      value: kube-system
    - name: vip_ddns
      value: "false"
    - name: vip_leaderelection
      value: "true"
    - name: vip_leaseduration
      value: "5"
    - name: vip_renewdeadline
      value: "3"
    - name: vip_retryperiod
      value: "1"
    - name: address
      value: "{{ .VIP }}"
    securityContext:
      capabilities:
        add:
        - NET_ADMIN
        - NET_RAW
    volumeMounts:
    - mountPath: /etc/kubernetes/admin.conf
      name: kubeconfig
  hostNetwork: true
  hostAliases:
  - ip: 127.0.0.1
    hostnames:
    - kubernetes
  volumes:
  - name: kubeconfig
    hostPath:
      path: /etc/rancher/k3s/k3s.yaml
`

type kubeVIPData struct {
	Version   string
	VIP       string
	Interface string
}

type KubeVIPInstaller struct {
	exec *ssh.Executor
	k3si *k3s.Installer
}

func NewKubeVIPInstaller(exec *ssh.Executor, k3si *k3s.Installer) *KubeVIPInstaller {
	return &KubeVIPInstaller{exec: exec, k3si: k3si}
}

func (kv *KubeVIPInstaller) Install(ctx context.Context, masters []domain.Node, vip string) error {
	t, err := template.New("kubevip").Parse(kubeVIPManifest)
	if err != nil {
		return fmt.Errorf("parsing kube-vip template: %w", err)
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, kubeVIPData{
		Version:   kubeVIPVersion,
		VIP:       vip,
		Interface: "eth0",
	}); err != nil {
		return fmt.Errorf("rendering kube-vip manifest: %w", err)
	}

	manifest := buf.Bytes()
	remotePath := "/var/lib/rancher/k3s/server/manifests/kube-vip.yaml"

	for _, master := range masters {
		if err := kv.exec.Upload(ctx, master.IP(), remotePath, manifest, 0644); err != nil {
			return fmt.Errorf("uploading kube-vip manifest to %s: %w", master.Name, err)
		}
	}

	return nil
}
