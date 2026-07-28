# Referência de Configuração

Todos os campos do YAML, com defaults e exemplos.

> Para arquivos prontos para copiar: [`examples/`](../examples/).

---

## Estrutura geral

```yaml
cluster_name: <string>
provider: proxmox

proxmox:
  # Conexão e autenticação
  endpoint: <url>
  token_id: <string>
  token_secret: <string ou "${VAR}">
  insecure_skip_tls_verify: <bool>
  ca_file: <path>

  # Infra
  target_nodes: <[]string>
  template: <string>
  storage: <string>
  snippets_storage: <string>
  placement_strategy: <round-robin|least-loaded|pinned>
  vmid_range: [<min>, <max>]

networking:
  mode: <static|dhcp>
  bridge: <string>
  vlan_tag: <int>
  cidr: <CIDR>
  gateway: <IP>
  nameservers: <[]IP>
  search_domains: <[]string>
  node_pool_range: <IP-IP>
  control_plane_vip: <IP>
  ssh:
    public_key_path: <path>
    private_key_path: <path>
    port: <int>

k3s:
  version: <string>
  cni: <flannel|cilium>
  cluster_cidr: <CIDR>
  service_cidr: <CIDR>
  disable: <[]string>
  kubelet_args: <[]string>
  kube_api_args: <[]string>

masters_pool:
  count: <int>
  size: <catalog-size>
  resources: <NodeResources>
  labels: <map[string]string>
  taints: <[]string>

worker_node_pools:
  - name: <string>
    count: <int>
    size: <catalog-size>
    resources: <NodeResources>
    labels: <map[string]string>
    taints: <[]string>
    autoscaling:
      enabled: <bool>
      min: <int>
      max: <int>

addons:
  kube_vip:
    enabled: <bool>
    version: <string>
  metallb:
    enabled: <bool>
    version: <string>
    address_pool: <IP-IP ou CIDR>
  proxmox_ccm:
    enabled: <bool>
    version: <string>
  proxmox_csi:
    enabled: <bool>
    version: <string>
    storage: <string>
  system_upgrade_controller:
    enabled: <bool>
    version: <string>
```

---

## `cluster_name`

**Tipo:** `string` — **Obrigatório**

Nome do cluster. Usado como prefixo dos nomes das VMs (`{cluster_name}-master-1`, `{cluster_name}-general-1`) e como valor da tag de identificação no Proxmox (`k3s-cluster={cluster_name}`).

Restrições: apenas letras minúsculas, números e hífens. Máximo de 20 caracteres.

```yaml
cluster_name: homelab
```

---

## `provider`

**Tipo:** `string` — **Obrigatório**

O provedor de infraestrutura a ser usado. Atualmente apenas `proxmox` é suportado.

```yaml
provider: proxmox
```

---

## `proxmox`

### `endpoint`

**Tipo:** `string` — **Obrigatório**

URL da API do Proxmox VE, incluindo porta.

```yaml
proxmox:
  endpoint: https://pve.local:8006
```

### `token_id`

**Tipo:** `string` — **Obrigatório**

ID do API token no formato `user@realm!tokenname`.

```yaml
  token_id: "k3s@pve!k3s"
```

### `token_secret`

**Tipo:** `string` — **Obrigatório**

Valor secreto do API token. **Nunca coloque o valor diretamente no YAML** — use uma variável de ambiente:

```yaml
  token_secret: "${PROXMOX_TOKEN}"
```

### `insecure_skip_tls_verify`

**Tipo:** `bool` — Default: `false`

Desabilita a verificação do certificado TLS. **Para uso em homelab** com certificados self-signed. Um aviso é exibido quando habilitado.

```yaml
  insecure_skip_tls_verify: true
```

### `ca_file`

**Tipo:** `path` — Default: vazio

Caminho para o arquivo CA para verificação do certificado. Alternativa recomendada ao `insecure_skip_tls_verify`.

```yaml
  ca_file: /etc/ssl/certs/proxmox-ca.pem
```

### `target_nodes`

**Tipo:** `[]string` — Default: todos os nós do cluster PVE

Limita os hosts PVE onde as VMs serão criadas. Se omitido, todos os nós online do cluster são elegíveis.

```yaml
  target_nodes: [pve1, pve2, pve3]
```

### `template`

**Tipo:** `string` — **Obrigatório**

Nome do template Cloud-Init no Proxmox. Deve existir como template (`template=1`) com Cloud-Init e qemu-guest-agent configurados. Ver [docs/proxmox-setup.md](proxmox-setup.md).

```yaml
  template: ubuntu-2404-cloudinit
```

### `storage`

**Tipo:** `string` — **Obrigatório**

Storage do Proxmox onde os discos das VMs serão criados (tipo `images` ou `qcow2`).

```yaml
  storage: local-lvm
```

### `snippets_storage`

**Tipo:** `string` — Default: autodetectado (primeiro storage com content `snippets`)

Storage com content type `snippets` para armazenar configurações Cloud-Init customizadas. Se não houver storage com snippets disponível, o framework usa apenas os campos nativos Cloud-Init do Proxmox.

```yaml
  snippets_storage: local
```

### `placement_strategy`

**Tipo:** `string` — Default: `round-robin`

Estratégia de distribuição de VMs entre os hosts PVE:

- `round-robin`: distribui uniformemente pelos hosts elegíveis.
- `least-loaded`: prefere o host com mais recursos disponíveis (RAM livre).
- `pinned`: o pool declara `target_nodes` explicitamente.

Masters são **sempre** distribuídos em hosts distintos quando possível (anti-afinidade).

```yaml
  placement_strategy: round-robin
```

### `vmid_range`

**Tipo:** `[int, int]` — Default: `[100, 999999]`

Intervalo de VMIDs para alocação. O framework usa `/cluster/nextid` dentro deste intervalo, com retry em colisão.

```yaml
  vmid_range: [200, 299]
```

---

## `networking`

### `mode`

**Tipo:** `string` — **Obrigatório** — Valores: `static`, `dhcp`

Modo de alocação de IP:

- `static`: IPs alocados do `node_pool_range`, configurados via `ipconfig0` no Cloud-Init. Determinístico e previsível.
- `dhcp`: Cloud-Init com `ip=dhcp`; IPs descobertos via qemu-guest-agent após o boot. Requer reservas DHCP para estabilidade.

```yaml
networking:
  mode: static
```

### `bridge`

**Tipo:** `string` — **Obrigatório**

Bridge do Proxmox para a interface de rede das VMs. Deve existir antes de rodar o framework.

```yaml
  bridge: vmbr0
```

### `vlan_tag`

**Tipo:** `int` — Default: sem VLAN

Tag VLAN para a interface de rede. Deixe omitido para redes sem VLAN.

```yaml
  vlan_tag: 20
```

### `cidr`

**Tipo:** `CIDR` — **Obrigatório em modo `static`**

CIDR da rede dos nós.

```yaml
  cidr: 192.168.20.0/24
```

### `gateway`

**Tipo:** `IP` — **Obrigatório em modo `static`**

Gateway padrão para os nós.

```yaml
  gateway: 192.168.20.1
```

### `nameservers`

**Tipo:** `[]IP` — Default: `[8.8.8.8, 1.1.1.1]`

Servidores DNS para os nós.

```yaml
  nameservers: [192.168.20.1, 1.1.1.1]
```

### `node_pool_range`

**Tipo:** `IP-IP` — **Obrigatório em modo `static`**

Intervalo de IPs alocados para os nós. Deve conter IPs suficientes para todos os masters + workers somados.

O framework aloca IPs deterministicamente: `{pool_name}-0` sempre recebe o primeiro IP do intervalo. Re-executar `create` produz os mesmos IPs.

```yaml
  node_pool_range: 192.168.20.50-192.168.20.99
```

### `control_plane_vip`

**Tipo:** `IP` — **Obrigatório quando `masters_pool.count > 1`**

IP virtual do API Server (kube-vip). Deve estar:
- Na mesma subnet dos masters (mesma L2)
- Fora do `node_pool_range`
- Fora do `addons.metallb.address_pool`
- Livre (sem resposta a ping/ARP) antes de `create`

```yaml
  control_plane_vip: 192.168.20.10
```

### `ssh`

```yaml
  ssh:
    public_key_path: ~/.ssh/id_ed25519.pub   # Obrigatório
    private_key_path: ~/.ssh/id_ed25519      # Obrigatório
    port: 22                                  # Default: 22
```

---

## `k3s`

### `version`

**Tipo:** `string` — Default: `stable`

Versão do K3s. Aceita versão exata ou canal:

- Versão exata: `v1.31.4+k3s1` (recomendado para produção — garante que todos os nós usam a mesma versão)
- Canal: `stable`, `latest` (resolvido no preflight e fixado para todos os nós)

```yaml
k3s:
  version: v1.31.4+k3s1
```

### `cni`

**Tipo:** `string` — Default: `flannel` — Valores: `flannel`, `cilium`

CNI a ser usado. Flannel é o default do K3s e funciona sem configuração adicional. Cilium oferece NetworkPolicy mais rico, eBPF e observabilidade (v1).

```yaml
  cni: flannel
```

### `cluster_cidr`

**Tipo:** `CIDR` — Default: `10.42.0.0/16`

CIDR para os Pods do cluster.

### `service_cidr`

**Tipo:** `CIDR` — Default: `10.43.0.0/16`

CIDR para os Services do cluster.

### `disable`

**Tipo:** `[]string` — Default: `["servicelb"]` quando MetalLB está habilitado

Componentes do K3s a desabilitar. O framework gerencia automaticamente:
- `servicelb` é desabilitado quando `addons.metallb.enabled: true`
- `traefik` pode ser desabilitado manualmente

```yaml
  disable: [traefik]
```

---

## `masters_pool`

### `count`

**Tipo:** `int` — **Obrigatório** — Mínimo: `1`

Número de masters. **Deve ser ímpar e ≥ 3** para HA real (quorum do etcd). Valor `1` cria um cluster single-master com SQLite (sem HA); o preflight avisa.

```yaml
masters_pool:
  count: 3
```

### `size`

**Tipo:** `string` — Default: `cp-medium`

Tamanho do catálogo. Ver [ADR-006](adr/006-vm-size-catalog.md) para a tabela completa.

```yaml
  size: cp-medium    # 4 vCPU / 8 GB / 60 GB
```

### `resources`

**Tipo:** `NodeResources` — Override pontual ou configuração completa

```yaml
  resources:
    cores: 4
    memory: 8192      # MB
    disk_size: 60     # GB
```

### `labels` e `taints`

```yaml
  labels:
    node-role.kubernetes.io/control-plane: "true"
  taints:
    - node-role.kubernetes.io/control-plane:NoSchedule
```

---

## `worker_node_pools`

Lista de pools de workers. Cada pool gera VMs com a mesma configuração.

```yaml
worker_node_pools:
  - name: general        # Obrigatório; deve ser único
    count: 3
    size: standard-4     # 4 vCPU / 8 GB / 100 GB

  - name: db
    count: 2
    size: memory-4       # 4 vCPU / 16 GB / 200 GB
    resources:
      disk_size: 400     # override do disco apenas
    labels:
      workload: database
    taints:
      - workload=database:NoSchedule

    # Campos reservados para autoscaling (v2)
    autoscaling:
      enabled: false
      min: 1
      max: 10
```

---

## `addons`

### `kube_vip`

**Habilitado automaticamente** quando `masters_pool.count > 1`.

```yaml
addons:
  kube_vip:
    enabled: true
    version: v0.8.0    # Default: latest estável testado
```

### `metallb`

```yaml
  metallb:
    enabled: true
    version: v0.14.0
    address_pool: 192.168.20.200-192.168.20.220   # ou CIDR: 192.168.20.200/28
```

### `proxmox_ccm`

Cloud Controller Manager para o Proxmox. Gerencia topology labels e remove o taint `node.cloudprovider.kubernetes.io/uninitialized`. **Obrigatório quando `proxmox_csi.enabled: true`** (v1).

```yaml
  proxmox_ccm:
    enabled: true
    version: v0.7.0
```

### `proxmox_csi`

Storage CSI para provisionar PersistentVolumes no Proxmox. Requer `proxmox_ccm.enabled: true` (v1).

```yaml
  proxmox_csi:
    enabled: true
    version: v0.8.0
    storage: local-lvm
```

### `system_upgrade_controller`

Gerencia upgrades do K3s via `Plan` CRDs. Usado pelo comando `proxmox-k3s upgrade` (v1).

```yaml
  system_upgrade_controller:
    enabled: true
    version: v0.13.0
```

---

## Variáveis de ambiente

O framework expande `${VAR}` em qualquer valor de string do YAML. Variáveis úteis:

| Variável | Descrição |
|---|---|
| `PROXMOX_TOKEN` | Valor do API token (obrigatório) |
| `PROXMOX_ENDPOINT` | URL da API (alternativa ao campo) |
| `K3S_VERSION` | Versão do K3s (alternativa ao campo) |
