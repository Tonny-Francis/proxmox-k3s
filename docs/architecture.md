# Arquitetura

Decisões importantes têm um ADR correspondente em `docs/adr/` — nesses arquivos está o raciocínio completo. Este documento é a visão de cima.

## Como está organizado

O `proxmox-k3s` é um binário CLI em Go que fala diretamente com a API REST do Proxmox VE e com os nós via SSH. Sem Terraform, sem Ansible, sem agentes.

A organização segue Clean Architecture em 4 camadas, com dependências sempre apontando para dentro:

```
┌──────────────────────────────────────────────────────────────────┐
│  cmd/ + internal/cli           (adaptadores de ENTRADA: Cobra)    │
│  internal/adapter/http/        (adaptador de entrada: painel v2)  │
├──────────────────────────────────────────────────────────────────┤
│  internal/usecase/             (orquestração: CreateCluster,      │
│                                 DeleteCluster, UpgradeCluster)    │
├──────────────────────────────────────────────────────────────────┤
│  internal/domain/              (entidades + PORTS / interfaces)   │
│    Cluster, NodePool, Node, NetworkPlan, K3sVersion, Catalog      │
│    ports: InfraProvider, IPAllocator, CommandExecutor,            │
│           K3sInstaller, AddonInstaller, KubeClient                │
├──────────────────────────────────────────────────────────────────┤
│  internal/adapter/             (adaptadores de SAÍDA)             │
│    proxmox/  ssh/  k3s/  kubernetes/  cloudinit/  ipam/           │
└──────────────────────────────────────────────────────────────────┘
```

Regras que mantêm a arquitetura intacta:
- `domain/` não importa nada externo além da stdlib básica.
- `usecase/` depende apenas de `domain/` — testável com fakes, sem tocar em infra.
- `adapter/` implementa as interfaces de `domain/ports`. Trocar o provedor = novo pacote em `adapter/`, zero mudança em `usecase/`.
- Injeção de dependência explícita em `cmd/`. Sem framework de DI, sem `init()` com efeitos colaterais.

## Por que essa separação

O hetzner-k3s funde os três eixos abaixo propositalmente — faz sentido para um projeto focado num único provedor. O `proxmox-k3s` os separa porque cada um muda por razões diferentes:

| Eixo | Interface | Implementações |
|---|---|---|
| Onde as máquinas nascem | `InfraProvider` | Proxmox (MVP), bare-metal/external (v1), vSphere (v2) |
| Como se fala com elas | `CommandExecutor` | SSH (MVP), poderia ser WinRM, agent, cloud-init phone-home |
| O que se instala | `K3sInstaller` | K3s (hoje), poderia ser RKE2, k0s |

---

## Estrutura de diretórios

```
proxmox-k3s/
├── cmd/
│   └── proxmox-k3s/
│       └── main.go                    # composition root
├── internal/
│   ├── cli/                           # Cobra commands
│   │   ├── root.go
│   │   ├── create.go / delete.go / upgrade.go / scale.go
│   │   ├── validate.go / sizes.go / status.go
│   │   ├── template.go                # template build / validate
│   │   └── output/                    # progresso, spinner, logs prefixados por nó
│   │
│   ├── config/                        # anti-corruption layer do YAML
│   │   ├── schema.go                  # structs com tags yaml + defaults
│   │   ├── loader.go                  # leitura, env expansion, merge de defaults
│   │   ├── tomodel.go                 # config → domain.Cluster
│   │   └── validate/                  # validadores estáticos (sem I/O)
│   │       ├── cluster.go  network.go  nodepools.go  k3s.go  addons.go
│   │
│   ├── domain/
│   │   ├── cluster.go                 # Cluster, ControlPlaneEndpoint
│   │   ├── nodepool.go / node.go      # NodePool, Node, NodeRole, NodeState
│   │   ├── network.go                 # NetworkPlan, IPAssignment, IPMode
│   │   ├── catalog/
│   │   │   └── sizes.go               # catálogo de tamanhos com mínimos
│   │   ├── errors.go                  # erros tipados/sentinela do domínio
│   │   └── ports/
│   │       ├── infra.go               # InfraProvider
│   │       ├── ipam.go                # IPAllocator
│   │       ├── executor.go            # CommandExecutor (SSH)
│   │       ├── k3s.go                 # K3sInstaller
│   │       ├── addons.go              # AddonInstaller
│   │       └── kube.go                # KubeClient
│   │
│   ├── usecase/
│   │   ├── create/
│   │   │   ├── create.go              # pipeline de fases
│   │   │   ├── preflight.go           # validações com I/O (read-only)
│   │   │   ├── provision.go           # criação paralela de VMs
│   │   │   ├── bootstrap.go           # bootstrap do control plane
│   │   │   └── addons.go
│   │   ├── delete/ upgrade/ scale/ reconcile/
│   │
│   ├── adapter/
│   │   ├── proxmox/                   # implementação do InfraProvider
│   │   │   ├── client/                # HTTP: auth, retry, rate limit, task polling
│   │   │   ├── vm/                    # clone, config, resize, start, stop, delete
│   │   │   ├── node/                  # hosts PVE, capacidade
│   │   │   ├── storage/ template/ placement/
│   │   │   └── provider.go
│   │   ├── ipam/
│   │   │   ├── static.go              # pool declarativo, alocação determinística
│   │   │   └── dhcp.go                # descoberta via qemu-guest-agent
│   │   ├── cloudinit/
│   │   │   ├── render.go
│   │   │   └── templates/*.tmpl       # embed.FS
│   │   ├── ssh/
│   │   │   ├── client.go              # x/crypto/ssh
│   │   │   └── executor.go            # implementa CommandExecutor
│   │   ├── k3s/
│   │   │   ├── installer.go
│   │   │   ├── scripts/*.sh.tmpl      # embed.FS
│   │   │   ├── version.go
│   │   │   └── kubeconfig.go
│   │   └── kubernetes/
│   │       ├── client.go
│   │       └── software/              # kube-vip, metallb, csi, ccm, upgrade-controller
│   │           └── manifests/*.yaml.tmpl
│   │
│   └── pkg/
│       └── retry/  logx/  parallel/
│
├── examples/
├── docs/
│   └── adr/
└── e2e/
```

---

## Fluxo de provisionamento (`create`)

O pipeline é dividido em fases. Cada fase é idempotente; o pipeline para na primeira falha e faz rollback do que foi criado **nesta execução**.

```
Fase 0  Load & Validate      (sem I/O)
         ↓
Fase 1  Preflight             (API Proxmox, read-only)
         ↓
Fase 2  Reconcile             (descobre o que já existe via tags)
         ↓
Fase 3  Alocação de IPs
         ↓
Fase 4  ┌─ clone VM 1 ─┐
        │  clone VM 2   │  (paralelo, semáforo de clone por host)
        └─ clone VM N ─┘
         ↓
Fase 5  Convergência          (guest-agent, SSH, cloud-init)
         ↓
Fase 6  Bootstrap CP          (master-1 → kube-vip → master-2,3)
         ↓
Fase 7  Workers               (paralelo)
         ↓
Fase 8  Kubeconfig
         ↓
Fase 9  Addons
         ↓
Fase 10 Sumário
```

### Rollback

| Fase | Comportamento na falha |
|---|---|
| 0–2 (validação/reconcile) | Aborta. Nada foi criado. |
| 4 (provisionamento) | Rollback transacional das VMs criadas **nesta sessão**. VMs pré-existentes nunca são tocadas. `--no-rollback` para preservar e debugar. |
| 5 (convergência) | Retry com backoff. Timeout → rollback das VMs desta sessão. |
| 6 (bootstrap CP) | **Sem rollback automático.** Falha é quase sempre acionável. Instrui: corrigir e re-rodar `create` (idempotente) ou `delete` explícito. |
| 7 (workers) | Falha isolada por worker: reporta e continua. Cluster sobe parcial. |
| 9 (addons) | Falha não-fatal. Cluster entregue. `addons install` para retentar. |

### Por que sem state file

Um state file local sofre drift (admin apagou uma VM pela UI), pode ser corrompido, conflita entre operadores e levanta a questão "onde guardo isso". No `proxmox-k3s`, as tags do Proxmox (`k3s-cluster`, `k3s-role`, `k3s-pool`) são a identidade dos recursos — sobrevivem a qualquer `rm` local. O qemu-guest-agent e a API do Kubernetes completam o quadro. O reconciliador cruza as três fontes e calcula o diff. É a mesma ideia do `node_detection` do hetzner-k3s, levada um passo adiante.

---

## Comunicação com a API do Proxmox

### Autenticação

Exclusivamente via **API Token** (`PVEAPIToken=user@pam!tokenid=uuid`). Stateless, sem expiração, sem CSRF. O secret é lido de variável de ambiente no YAML (`${PROXMOX_TOKEN}`).

### Client HTTP próprio

Não usamos SDK de terceiros (avaliamos `luthermonson/go-proxmox` e `Telmate/proxmox-api-go`). Motivos:

1. A superfície usada é pequena (~12 endpoints) e estável.
2. Precisamos de controle fino sobre task polling, retry e qualidade dos erros.
3. SDKs genéricos vazam abstração do provedor para dentro do adapter.

Ver [ADR-005](adr/005-proxmox-http-client.md) para o detalhamento.

### Padrões obrigatórios do client

- **Toda chamada mutante retorna um UPID** → `waitTask(ctx, node, upid)` com poll (500ms → 2s) e timeout.
- **Roteamento por host**: URLs carregam o nó PVE. `NodeResolver` decide o destino.
- **Retry com backoff exponencial + jitter** para 5xx e timeouts. Nunca em `DELETE`/`clone` sem checar idempotência.
- **Lock contention** (`VM is locked`): condição retryable específica.
- **Semáforo de clone por host** (default: 2) — clone é I/O-pesado.
- **Erros tipados**: `ErrTemplateNotFound`, `ErrStorageNotFound`, `ErrVMIDInUse`, etc., com sugestão de correção.

---

## Alta disponibilidade do control plane

A Hetzner oferece um Load Balancer gerenciado para o API Server. O Proxmox não. Solução: **kube-vip em modo ARP**.

Três masters com etcd embutido recebem um VIP L2 via `kube-vip` rodando como static pod em `/var/lib/rancher/k3s/server/manifests/`. O `kubeconfig` e todos os workers apontam para o VIP. Se qualquer master cair, o VIP migra para outro em segundos.

**Requisito:** os masters precisam estar na mesma rede L2 (sem NAT entre eles). O preflight valida.

**Escape:** flag `--single-master` para ambientes onde ARP gratuito é bloqueado (retorna single-master com SQLite — sem HA, mas funcional).

Ver [ADR-004](adr/004-kube-vip-control-plane.md).

---

## Catálogo de tamanhos

VMs com 1 vCPU e 1 GB de RAM "sobem" mas não rodam K8s de forma estável. No Proxmox não há proteção natural do catálogo do provedor (como na Hetzner, onde instâncias pequenas demais simplesmente não existem). O framework aplica mínimos no preflight que **falham o create** — não apenas avisam — e oferece tamanhos nomeados como caminho padrão.

| Nome | vCPU | RAM | Disco | Uso recomendado |
|---|---|---|---|---|
| `cp-small` | 2 | 4 GB | 40 GB | Control plane, cluster pequeno |
| `cp-medium` | 4 | 8 GB | 60 GB | Control plane, **default recomendado** |
| `cp-large` | 8 | 16 GB | 100 GB | Control plane, cluster grande |
| `standard-2` | 2 | 4 GB | 60 GB | Worker mínimo viável |
| `standard-4` | 4 | 8 GB | 100 GB | Worker, **default recomendado** |
| `standard-8` | 8 | 16 GB | 200 GB | Worker |
| `memory-4` | 4 | 16 GB | 200 GB | Bancos de dados, cache |
| `memory-8` | 8 | 32 GB | 400 GB | Bancos de dados, cache |
| `cpu-8` | 8 | 8 GB | 100 GB | Build, CI, processamento |

Recursos crus continuam disponíveis via `resources:` + possibilidade de override pontual.

Ver [ADR-006](adr/006-vm-size-catalog.md).

---

## Extensibilidade para outros provedores

A interface `InfraProvider` existe desde o MVP — não para cobrir provedores hipotéticos, mas porque organiza o código e torna o `usecase` testável. A abstração é ditada pelo que o `usecase` precisa, não pelo que o Proxmox expõe. O caminho:

1. **MVP**: uma implementação (Proxmox). Interface validada pelo usecase.
2. **v1**: adapter `external-nodes` (bare-metal/SSH já existente). Valida a abstração com custo baixo.
3. **v2**: segundo provider real com dois casos concretos na mão para informar a refatoração.

`Capabilities()` no provider permite que o usecase adapte o comportamento (ex.: Hetzner tem LB gerenciado, Proxmox usa kube-vip) sem `if provider == "proxmox"` no core.

Ver [ADR-007](adr/007-provider-extensibility.md).

---

## Descoberta de nós

Três fontes, em ordem de autoridade:

1. **Proxmox** (`/cluster/resources?type=vm` filtrado por tag) — verdade sobre infra
2. **qemu-guest-agent** (`agent/network-get-interfaces`) — verdade sobre rede
3. **Kubernetes** (`client-go`, `GET /nodes`) — verdade sobre o cluster

O reconciliador cruza as três e classifica: `Missing`, `Provisioned`, `Joined`, `Orphan`, `Unhealthy`. Alimenta `create` idempotente, `delete`, `scale` e (futuramente) `status`.

---

## Decisões relacionadas

- [ADR-001](adr/001-go-language.md) — Go em vez de Crystal
- [ADR-002](adr/002-no-terraform-ansible.md) — Sem Terraform e Ansible
- [ADR-003](adr/003-no-state-file.md) — Tags do Proxmox como verdade, sem state file
- [ADR-004](adr/004-kube-vip-control-plane.md) — kube-vip para HA do API Server
- [ADR-005](adr/005-proxmox-http-client.md) — Client HTTP próprio em vez de SDK
- [ADR-006](adr/006-vm-size-catalog.md) — Catálogo de tamanhos com mínimos
- [ADR-007](adr/007-provider-extensibility.md) — Extensibilidade para outros provedores
- [ADR-008](adr/008-autoscaler-externalgrpc.md) — Autoscaling via externalgrpc
