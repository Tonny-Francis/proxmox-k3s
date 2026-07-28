# ADR-005 — Client HTTP próprio para a API do Proxmox

**Data:** 2026-07-28  
**Status:** ✅ Aceito

## Contexto

Existem SDKs Go para a API do Proxmox VE:

- [`luthermonson/go-proxmox`](https://github.com/luthermonson/go-proxmox) — mais moderno, com cloud-init e host-targeting dinâmico
- [`Telmate/proxmox-api-go`](https://github.com/Telmate/proxmox-api-go) — base do provider Terraform bpg/proxmox, mais adotado

A decisão é usar um deles ou implementar um client HTTP próprio.

## Decisão

**Client HTTP próprio**, com `luthermonson/go-proxmox` como **referência de implementação** para os quirks da API (especialmente cloud-init e qemu-guest-agent).

## Razões

Usamos ~12 endpoints estáveis do PVE. Um SDK genérico que cobre 200+ endpoints traz peso desnecessário e tira o controle fino sobre três coisas que importam aqui:

**Task polling:** toda chamada mutante do Proxmox é assíncrona — retorna um UPID. O `waitTask` precisa de timeout e backoff por operação, não uma política global. SDKs tendem a abstrair isso de formas que não servem.

**Erros acionáveis:** queremos `ErrTemplateNotFound: template "ubuntu-2404" não encontrado em "local-lvm" — verifique com: qm list | grep ubuntu`. Um SDK retorna o texto cru da API.

**Retry por operação:** retry em `clone` sem checar idempotência cria VMs duplicadas. Retry em `DELETE` pode falhar silenciosamente. O comportamento precisa ser diferente por endpoint.

O roteamento por host (`/nodes/{node}/qemu/...`) também é algo que SDKs genéricos costumam abstrair mal — e aqui é fundamental, porque uma VM pode estar em `pve1` ou `pve3` dependendo de onde foi criada.

## Endpoints utilizados

| Operação | Endpoint |
|---|---|
| Smoke test / versão | `GET /api2/json/version` |
| Hosts e capacidade | `GET /api2/json/cluster/resources?type=node` |
| Inventário de VMs (por tag) | `GET /api2/json/cluster/resources?type=vm` |
| Próximo VMID | `GET /api2/json/cluster/nextid` |
| Clonar template | `POST /api2/json/nodes/{node}/qemu/{vmid}/clone` |
| Configurar VM | `POST /api2/json/nodes/{node}/qemu/{vmid}/config` |
| Redimensionar disco | `PUT /api2/json/nodes/{node}/qemu/{vmid}/resize` |
| Ligar | `POST /api2/json/nodes/{node}/qemu/{vmid}/status/start` |
| Desligar | `POST /api2/json/nodes/{node}/qemu/{vmid}/status/shutdown` |
| Interfaces de rede (guest-agent) | `GET /api2/json/nodes/{node}/qemu/{vmid}/agent/network-get-interfaces` |
| Destruir VM | `DELETE /api2/json/nodes/{node}/qemu/{vmid}` |
| Status de tarefa | `GET /api2/json/nodes/{node}/tasks/{upid}/status` |

## Padrões implementados

- **Autenticação**: `Authorization: PVEAPIToken=user@pam!tokenid=uuid` (stateless, sem CSRF)
- **TLS**: suporte a `ca_file` e `insecure_skip_tls_verify` com warning visível
- **Retry**: backoff exponencial com jitter para 5xx e timeouts; não em DELETE/clone sem checar idempotência
- **Lock contention**: `VM is locked (clone)` é condição retryable específica com backoff mais longo
- **Semáforo de clone**: máximo de 2 clones simultâneos por host PVE (configurável)
- **Task polling**: `waitTask` com 500ms inicial, até 2s, com timeout total configurável por operação

## Quando reconsiderar

Se a superfície de endpoints crescer para cobrir storage, SDN, backup ou cluster management do PVE, avaliar integração com `luthermonson/go-proxmox` para os endpoints adicionais, mantendo o client próprio para os endpoints core do provisionamento.
