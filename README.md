<div align="center">

# proxmox-k3s

**Clusters K3s em alta disponibilidade no Proxmox VE — um comando, um arquivo YAML.**

[![Status](https://img.shields.io/badge/status-pre--alpha-orange?style=flat-square)](ROADMAP.md) [![License](https://img.shields.io/badge/license-Apache%202.0-blue?style=flat-square)](LICENSE) [![Go](https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat-square&logo=go&logoColor=white)](https://go.dev) [![Proxmox](https://img.shields.io/badge/Proxmox%20VE-8.0+-E57000?style=flat-square&logo=proxmox&logoColor=white)](https://www.proxmox.com) [![K3s](https://img.shields.io/badge/K3s-compatible-326CE5?style=flat-square&logo=kubernetes&logoColor=white)](https://k3s.io)

[![CI](https://img.shields.io/github/actions/workflow/status/nexusops/proxmox-k3s/ci.yml?branch=main&style=flat-square&label=CI)](https://github.com/nexusops/proxmox-k3s/actions) [![Release](https://img.shields.io/github/v/release/nexusops/proxmox-k3s?style=flat-square&color=brightgreen)](https://github.com/nexusops/proxmox-k3s/releases) [![Issues](https://img.shields.io/github/issues/nexusops/proxmox-k3s?style=flat-square)](https://github.com/nexusops/proxmox-k3s/issues) [![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen?style=flat-square)](CONTRIBUTING.md)

```bash
proxmox-k3s create -c cluster.yaml
```

> 🚧 **pre-alpha** — arquitetura em definição. Não use em produção.

</div>

---

## O que é

`proxmox-k3s` automatiza o ciclo de vida completo de clusters K3s no Proxmox VE: cria as VMs, configura rede e SSH, instala K3s em HA com 3+ masters, sobe kube-vip e MetalLB, e entrega o kubeconfig pronto para uso.

Sem Terraform. Sem Ansible. Um binário, um YAML.

**Inspirado em** [vitobotta/hetzner-k3s](https://github.com/vitobotta/hetzner-k3s) — excelente framework para Hetzner Cloud que provou que essa abordagem funciona. O `proxmox-k3s` traz a mesma filosofia para ambientes on-premises com Proxmox VE.

---

## Como funciona

```
proxmox-k3s create -c cluster.yaml
│
├── Preflight ──────── valida credenciais, template, storage, IPs, capacidade dos hosts
├── Provisioning ───── clona VMs em paralelo, distribui entre hosts PVE, cloud-init
├── Convergência ───── aguarda SSH, guest-agent, boot completo
├── Control Plane ──── K3s HA (3 masters, etcd) + kube-vip como VIP L2
├── Workers ────────── joins em paralelo com labels e taints por pool
├── Addons ─────────── MetalLB, system-upgrade-controller
└── Kubeconfig ──────── merge em ~/.kube/config apontando para o VIP
```

Re-executar `create` é sempre seguro — o reconciliador descobre o que já existe e só cria o que falta.

---

## Ambiente de testes

Desenvolvido e validado num servidor físico dedicado:

```
CPU   ›  28 núcleos / 56 threads
RAM   ›  64 GB DDR4
Disco ›  1 TB NVMe SSD
HV    ›  Proxmox VE 8.x
```

Topologias testadas: 1 e 3 hosts PVE · 3 masters + até 6 workers · IP estático e DHCP.

---

## Por que não usar Terraform ou Ansible

O Proxmox não tem Load Balancer gerenciado, não tem catálogo de instâncias com mínimos garantidos, e a API é assíncrona por tarefas (UPID). Essas diferenças precisam de tratamento específico que não cabe bem numa abstração genérica.

| Problema real | Como o proxmox-k3s resolve |
|---|---|
| Proxmox não tem LB gerenciado para o API Server | `kube-vip` como static pod — VIP L2/ARP nos próprios masters |
| VM com 1 GB "sobe" mas não roda K3s | Catálogo de tamanhos com mínimos que **travam o create** no preflight |
| Terraform exige state file e runtime instalado | Tags do Proxmox como identidade; zero estado local |
| Falha no meio do provisionamento deixa VMs órfãs | Rollback transacional automático das VMs criadas na sessão |
| API do Proxmox é assíncrona (retorna UPID) | Client HTTP próprio com task polling, retry e erros acionáveis |

---

## Stack

| | |
|---|---|
| **Linguagem** | Go — binário estático, multi-arch, zero runtime para o usuário |
| **API do Proxmox** | Client HTTP próprio (sem SDK de terceiros) |
| **HA do API Server** | [kube-vip](https://kube-vip.io) — VIP L2/ARP |
| **LB de aplicações** | [MetalLB](https://metallb.universe.tf) — IPs reais da sua LAN |
| **Storage** *(v1)* | [proxmox-csi-plugin](https://github.com/sergelogvinov/proxmox-csi-plugin) |
| **Node lifecycle** *(v1)* | [proxmox-cloud-controller-manager](https://github.com/sergelogvinov/proxmox-cloud-controller-manager) |
| **Upgrade K3s** | [system-upgrade-controller](https://github.com/rancher/system-upgrade-controller) |
| **Autoscaling** *(v2)* | [cluster-autoscaler](https://github.com/kubernetes/autoscaler) via `externalgrpc` |

---

## Quickstart

> ⚠️ Ainda não funcional — código em desenvolvimento. Os comandos abaixo refletem a API planejada.

```bash
# Validar configuração sem criar nada
proxmox-k3s validate -c cluster.yaml

# Ver o catálogo de tamanhos e consumo estimado de recursos
proxmox-k3s sizes

# Criar o cluster
proxmox-k3s create -c cluster.yaml

# Cluster pronto
kubectl get nodes -o wide
```

## Configuração mínima

```yaml
cluster_name: homelab
provider: proxmox

proxmox:
  endpoint: https://pve.local:8006
  token_id: "k3s@pve!k3s"
  token_secret: "${PROXMOX_TOKEN}"
  insecure_skip_tls_verify: true
  template: ubuntu-2404-cloudinit
  storage: local-lvm

networking:
  mode: static
  bridge: vmbr0
  cidr: 192.168.20.0/24
  gateway: 192.168.20.1
  node_pool_range: 192.168.20.50-192.168.20.99
  control_plane_vip: 192.168.20.10   # VIP do kube-vip — fora do pool
  ssh:
    public_key_path: ~/.ssh/id_ed25519.pub
    private_key_path: ~/.ssh/id_ed25519

k3s:
  version: v1.31.4+k3s1

masters_pool:
  count: 3
  size: cp-medium        # 4 vCPU / 8 GB / 60 GB

worker_node_pools:
  - name: general
    count: 3
    size: standard-4     # 4 vCPU / 8 GB / 100 GB

addons:
  kube_vip:  { enabled: true }
  metallb:   { enabled: true, address_pool: 192.168.20.200-192.168.20.220 }
```

→ Referência completa em [docs/configuration.md](docs/configuration.md)  
→ Exemplos prontos em [examples/](examples/)

---

## Roadmap

| Fase | O que entra | Status | Previsão |
|---|---|---|---|
| **0 — Docs** | Repo público, ADRs, exemplos, CI | 🔄 Em andamento | Jul 2026 |
| **MVP v0.1** | `create` · `delete` · `validate` · HA · kube-vip · MetalLB | 📅 Planejado | Set 2026 |
| **v1** | `upgrade` · `scale` · CSI · CCM · Cilium · `template build` | 📅 Planejado | Jan 2027 |
| **v2** | Autoscaling · painel web · 2º provider · air-gapped | 📅 Planejado | Jun 2027 |

→ Detalhes, critérios de sucesso e riscos em [ROADMAP.md](ROADMAP.md)

---

## Documentação

| | |
|---|---|
| [Arquitetura](docs/architecture.md) | Camadas, ports & adapters, fluxo de provisionamento |
| [Configuração](docs/configuration.md) | Referência completa do YAML |
| [Setup do Proxmox](docs/proxmox-setup.md) | Template cloud-init, API token, rede |
| [ADRs](docs/adr/) | Decisões arquiteturais e seus porquês |
| [Contribuindo](CONTRIBUTING.md) | Setup de dev, padrões, testes |

---

## Pré-requisitos

- Proxmox VE **8.0+**
- Template VM com Cloud-Init e `qemu-guest-agent` ([docs/proxmox-setup.md](docs/proxmox-setup.md))
- API Token com permissões mínimas ([docs/proxmox-setup.md](docs/proxmox-setup.md))
- Chave SSH (ED25519 ou RSA)
- Masters na mesma rede L2 (requisito do kube-vip em modo ARP)

---

## Contribuindo

O projeto está em fase inicial — o YAML de configuração ainda pode mudar. Feedback sobre a API, testes em topologias diferentes (versões do PVE, múltiplos hosts, VLANs) e documentação são as contribuições mais valiosas agora.

→ [CONTRIBUTING.md](CONTRIBUTING.md) para setup de dev e processo de PR  
→ [SECURITY.md](SECURITY.md) para reportar vulnerabilidades em privado

---

## Crédito

Conceitualmente inspirado em [hetzner-k3s](https://github.com/vitobotta/hetzner-k3s) de Vito Botta (MIT). Zero código reutilizado — as APIs são incompatíveis — mas a filosofia de binário único, SSH direto e YAML declarativo veio de lá.

---

<div align="center">

[Apache License 2.0](LICENSE) · Proxmox VE 8.x · K3s · Go

</div>
