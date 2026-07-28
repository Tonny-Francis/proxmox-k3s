# Setup do Proxmox VE

O que você precisa configurar no Proxmox antes de rodar o framework.

## Ambiente de testes

O `proxmox-k3s` é desenvolvido e testado num servidor físico com:

- **CPU:** 28 núcleos / 56 threads
- **RAM:** 64 GB DDR4
- **Armazenamento:** 1 TB NVMe SSD
- **Proxmox VE:** 8.x

Configurações menores também funcionam, desde que respeitem os [mínimos de recurso](#mínimos-de-recurso).

---

## Versão do Proxmox VE

**Requisito mínimo:** Proxmox VE 8.0

O `proxmox-k3s` usa a API REST `/api2/json` (v2) do PVE e verifica a versão no preflight. Versões 7.x podem funcionar parcialmente, mas não são suportadas oficialmente.

---

## API Token

O framework usa **API Token** — nunca usuário/senha.

### Criar o role com permissões mínimas

No shell do Proxmox (como root):

```bash
pveum role add K3sProvisioner \
  --privs "VM.Allocate,VM.Clone,VM.Config.CDROM,VM.Config.CPU,VM.Config.Cloudinit,VM.Config.Disk,VM.Config.HWType,VM.Config.Memory,VM.Config.Network,VM.Config.Options,VM.Migrate,VM.Monitor,VM.Audit,VM.PowerMgmt,VM.Snapshot,Datastore.AllocateSpace,Datastore.Audit,Sys.Audit,SDN.Use"
```

### Criar o usuário (opcional, recomendado)

```bash
pveum user add k3s@pve
```

### Associar o role ao usuário

```bash
# Permissão global (cobre todos os nós e storages)
pveum aclmod / --user k3s@pve --role K3sProvisioner
```

> **Alternativa mais restrita:** Se preferir limitar a storage e nós específicos, substitua `/` pelos paths correspondentes (`/storage/local-lvm`, `/nodes/pve1`). A permissão global é mais simples e recomendada para início.

### Criar o API Token

```bash
pveum user token add k3s@pve k3s --expire 0 --privsep 0
```

O output será algo como:
```
┌──────────────┬──────────────────────────────────────┐
│ key          │ value                                │
╞══════════════╪══════════════════════════════════════╡
│ full-tokenid │ k3s@pve!k3s                          │
│ info         │ {"privsep":"0"}                      │
│ value        │ xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx │
└──────────────┴──────────────────────────────────────┘
```

**Anote o `value` agora** — ele não é exibido novamente.

No seu YAML de cluster:
```yaml
proxmox:
  token_id: "k3s@pve!k3s"
  token_secret: "${PROXMOX_TOKEN}"   # nunca coloque o valor diretamente
```

E exporte antes de rodar:
```bash
export PROXMOX_TOKEN="xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
```

---

## Template Cloud-Init

O `proxmox-k3s` clona VMs a partir de um **template com Cloud-Init e qemu-guest-agent**. Você pode criar o template manualmente (abaixo) ou usar o comando `proxmox-k3s template build` (v1).

### Opção A: Manualmente (recomendado para MVP)

No shell do Proxmox (como root):

```bash
# 1. Baixar a cloud image
wget https://cloud-images.ubuntu.com/noble/current/noble-server-cloudimg-amd64.img \
  -O /tmp/ubuntu-2404.img

# 2. Criar a VM base (VMID 9000 — escolha um ID livre)
qm create 9000 \
  --name ubuntu-2404-cloudinit \
  --memory 2048 \
  --cores 2 \
  --net0 virtio,bridge=vmbr0 \
  --ostype l26 \
  --agent enabled=1 \
  --serial0 socket \
  --vga serial0

# 3. Importar o disco para o storage desejado
qm importdisk 9000 /tmp/ubuntu-2404.img local-lvm

# 4. Associar o disco importado
qm set 9000 --scsihw virtio-scsi-pci --scsi0 local-lvm:vm-9000-disk-0

# 5. Adicionar o drive Cloud-Init
qm set 9000 --ide2 local-lvm:cloudinit

# 6. Configurar boot pelo disco
qm set 9000 --boot c --bootdisk scsi0

# 7. Converter em template
qm template 9000
```

### Verificar o template

```bash
proxmox-k3s template validate \
  --template ubuntu-2404-cloudinit \
  --storage local-lvm
```

O comando verifica:
- Template existe e está marcado como `template=1`
- Tem disco Cloud-Init
- qemu-guest-agent está habilitado na config
- Storage é acessível

---

## Mínimos de recurso

| Papel | vCPU | RAM | Disco |
|---|---|---|---|
| Master | 2 | 4 GB | 20 GB |
| Worker | 2 | 2 GB | 20 GB |

Abaixo desses valores, o `create` falha no preflight com explicação. Ver [ADR-006](adr/006-vm-size-catalog.md).

---

## Rede

O `proxmox-k3s` **não cria** bridges ou VLANs — ele consome as existentes. Configure a rede no Proxmox antes de rodar o framework.

### Modo estático (recomendado)

- Uma bridge (ex.: `vmbr0`) conectada à rede onde os nós vão operar
- Um intervalo de IPs reservados para os nós (ex.: `192.168.20.50-99`)
- Um IP reservado para o VIP do control plane (ex.: `192.168.20.10`) — fora do pool dos nós e fora do pool do MetalLB
- Gateway configurado e acessível
- DNS funcional

O preflight valida que nenhum IP do pool responde a ping/ARP antes de iniciar.

### Modo DHCP

- Bridge configurada
- DHCP server com leases suficientes para todos os nós
- Recomendado: reservas DHCP por MAC para IPs estáveis (o framework gera MACs determinísticos: `02:XX:XX:XX:XX:XX` baseado no nome do cluster + índice)

### Requisito para kube-vip (HA)

Os masters precisam estar na **mesma rede L2** (mesma broadcast domain, sem NAT entre eles). O VIP L2/ARP não funciona entre redes L3 diferentes. Se seus hosts PVE estão em VLANs diferentes com roteamento entre elas, o kube-vip em modo ARP não vai funcionar — use a flag `--single-master` como escape.

---

## Storage para snippets (opcional)

Para usar Cloud-Init customizado além dos campos nativos do Proxmox (pacotes, hardening, sysctl), o framework gera snippets YAML e os armazena num storage com content type `snippets`.

Verificar se o storage suporta snippets:
```bash
pvesm status --content snippets
```

Se nenhum storage aparecer, habilitar no storage `local`:
```bash
pvesm set local --content backup,iso,snippets,vztmpl
```

Se não houver storage com snippets disponível, o framework usa apenas os campos nativos Cloud-Init do Proxmox (suficiente para o MVP).

---

## Verificar pré-requisitos com um comando

```bash
proxmox-k3s validate -c cluster.yaml
```

O `validate` executa todas as verificações acima sem criar nada. É o primeiro comando a rodar após configurar o YAML.
