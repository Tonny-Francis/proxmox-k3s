# ADR-003 — Tags do Proxmox como verdade, sem state file

**Data:** 2026-07-28  
**Status:** ✅ Aceito

## Contexto

Ferramentas de provisionamento de infra geralmente mantêm um **state file** que registra os recursos criados: VMIDs, IPs, nomes, o que existe vs. o que foi solicitado. O Terraform é o exemplo mais conhecido. O problema é que o state file é um passivo.

## Decisão

**Zero state file local.** A verdade sobre o que existe vive em três fontes primárias:

1. **Tags do Proxmox** — `k3s-cluster={nome}`, `k3s-role=master|worker`, `k3s-pool={nome}` — para VMID, host PVE, estado de energia e nome.
2. **qemu-guest-agent** (`network-get-interfaces`) — para IP observado.
3. **Kubernetes API** (`client-go`) — para estado do nó no cluster (Ready, NotReady, etc.).

O reconciliador no `usecase/reconcile` cruza as três e calcula o diff contra o YAML desejado: `toCreate`, `toKeep`, `toDelete`. Isso alimenta `create` (idempotente), `delete`, `scale` e `status`.

## Razões

O Proxmox já tem o state — os recursos existem lá, não num arquivo na sua máquina. Um state file local é apenas uma cópia que fica desatualizada: um admin apaga uma VM pela UI, alguém cria uma com o mesmo nome manualmente, o qemu-guest-agent trava e o IP anotado não bate com o real. Estado silenciosamente errado é o pior tipo.

Em vez de state file, o framework usa **tags do Proxmox** como identidade: `k3s-cluster`, `k3s-role`, `k3s-pool`. O reconciliador cruza o Proxmox, o qemu-guest-agent e a API do Kubernetes para calcular o diff. A ideia veio do `node_detection` do hetzner-k3s — funciona bem na prática e sobrevive a qualquer `rm` local.

## Consequências

- **`create` é sempre idempotente.** Pode ser re-executado quantas vezes necessário. Se o cluster já existe, é um no-op. Se faltam nós (por escala ou falha), apenas eles são criados.
- **Tags são contrato público.** A esquema de tags (`k3s-cluster`, `k3s-role`, `k3s-pool`) precisa ser documentada e estável — é a API de identidade dos recursos.
- **Nós precisam de `ProviderID` consistente.** Para o cluster-autoscaler (v2) correlacionar nó K8s ↔ VM, o `ProviderID` precisa ser determinístico: `proxmox://{hostname}/{vmid}`.
- **Namespacing de nomes.** Nomes de VMs incluem o nome do cluster para evitar colisões: `{cluster}-{pool}-{índice}`. Múltiplos clusters no mesmo Proxmox são suportados.

## Riscos e mitigações

| Risco | Mitigação |
|---|---|
| A API do Proxmox está offline quando `delete` roda | `delete` falha explicitamente — não remove silenciosamente metade dos recursos |
| VM criada manualmente com nome que colide com o padrão | `reconcile` a classifica como `Orphan`, **nunca** a destrói silenciosamente; `delete` só remove o que foi criado pelo framework (por tag) |
| Tag removida acidentalmente da VM | `reconcile` classifica como `Missing` e tentaria recriar. Documentar o risco de remover tags manualmente. Futuramente: proteção de tag via permissão Proxmox. |

## Alternativas consideradas

- **Arquivo YAML local** (`~/.proxmox-k3s/{cluster}.state.yaml`): Mais simples de implementar, mas sofre todos os problemas de drift listados acima.
- **etcd ou Redis remoto**: Complexidade desnecessária para o que é essencialmente um mapeamento de "recursos que existem no Proxmox".
- **State no próprio cluster Kubernetes** (ConfigMap/CRD): Dependência circular — o state do cluster está no cluster que pode não estar acessível quando você precisa deletar.
