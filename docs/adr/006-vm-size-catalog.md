# ADR-006 — Catálogo de tamanhos de VM com mínimos aplicados

**Data:** 2026-07-28  
**Status:** ✅ Aceito

## Contexto

No Proxmox VE, os recursos de uma VM são configuráveis livremente: o usuário pode criar uma VM com 1 vCPU e 512 MB de RAM. Uma VM com esses recursos "sobe" mas não roda K8s de forma estável — o etcd sofre de OOM, o kubelet começa a despejar pods, e o cluster se comporta de forma imprevisível.

Na Hetzner Cloud, esse problema não existe: o catálogo de instâncias da Hetzner simplesmente não oferece máquinas com menos de 2 GB de RAM. O `hetzner-k3s` herda essa proteção sem esforço. O `proxmox-k3s` precisa implementá-la ativamente.

## Decisão

**Catálogo de tamanhos nomeados com mínimos absolutos aplicados no preflight.**

O caminho recomendado é declarar `size: cp-medium` em vez de `cores: 4, memory: 8192, disk_size: 60`. Recursos crus ainda são suportados via `resources:`, mas passam pelas mesmas validações.

## Catálogo

| Nome | vCPU | RAM | Disco | Uso |
|---|---|---|---|---|
| `cp-small` | 2 | 4 GB | 40 GB | Control plane, cluster pequeno (< 20 nós) |
| `cp-medium` | 4 | 8 GB | 60 GB | Control plane, **default recomendado** |
| `cp-large` | 8 | 16 GB | 100 GB | Control plane, cluster grande |
| `standard-2` | 2 | 4 GB | 60 GB | Worker mínimo viável |
| `standard-4` | 4 | 8 GB | 100 GB | Worker, **default recomendado** |
| `standard-8` | 8 | 16 GB | 200 GB | Worker |
| `memory-4` | 4 | 16 GB | 200 GB | Bancos de dados, cache |
| `memory-8` | 8 | 32 GB | 400 GB | Bancos de dados, cache |
| `cpu-8` | 8 | 8 GB | 100 GB | Build, CI, processamento |

## Mínimos absolutos (preflight que falha, não avisa)

| Papel | vCPU | RAM | Disco |
|---|---|---|---|
| Master | 2 | 4 GB | 20 GB |
| Worker | 2 | 2 GB | 20 GB |

Abaixo desses valores, o `create` falha no preflight com mensagem que explica o porquê concreto:
```
ERRO: master-pool solicita 1 GB de RAM (mínimo: 4 GB).
K3s com etcd embutido requer no mínimo 4 GB no control plane.
Abaixo desse valor, o etcd tende a sofrer OOM sob carga leve.
Use 'size: cp-small' (4 GB) ou configure 'resources.memory: 4096'.
```

## Zona de warning (avisa, não falha)

Entre o mínimo e o tamanho recomendado para o papel, o framework avisa e continua:
```
AVISO: master-pool usa 'standard-2' (4 GB RAM). Recomendado: 'cp-medium' (8 GB).
Com mais de 20 nós, o control plane com 4 GB tende a ficar instável sob carga do etcd.
```

## Override pontual

```yaml
worker_node_pools:
  - name: db
    size: memory-4       # 4 vCPU / 16 GB / 200 GB
    resources:
      disk_size: 400     # override só do disco, mantém cpu/memory do preset
```

## Comando `sizes`

```bash
proxmox-k3s sizes
```

Lista o catálogo com o total de recursos que cada combinação vai consumir, e compara com a capacidade disponível nos hosts PVE alvo.

## Razões

Erros de dimensionamento são a causa mais comum de "o framework não funciona" em ambientes Proxmox — uma VM com 1 GB sobe, mas o etcd não aguenta. Na Hetzner isso não acontece porque instâncias pequenas demais simplesmente não existem no catálogo; no Proxmox o campo é livre, então a validação precisa existir no framework.

`size: standard-4` também é mais legível e fácil de comunicar do que `cores: 4, memory: 8192, disk_size: 100`. E com tamanhos nomeados, o preflight consegue calcular o total de recursos antes de criar qualquer VM e comparar com o que os hosts PVE têm disponível.

## Consequências

- O catálogo vive em `internal/domain/catalog/sizes.go` — dado versionado, não hardcode espalhado.
- `proxmox-k3s sizes` é um comando de primeira classe, não uma flag escondida.
- A documentação precisa explicar claramente por que os mínimos existem (não apenas o quê).
- Contribuidores podem propor novos tamanhos via PR com justificativa de caso de uso.
