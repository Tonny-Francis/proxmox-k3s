# ADR-008 — Autoscaling via cluster-autoscaler externalgrpc

**Data:** 2026-07-28  
**Status:** ✅ Planejado (v2)

## Contexto

Autoscaling de nós (escalar o número de workers baseado em demanda de Pods) é um requisito essencial para v2. As opções são:

1. Reimplementar a lógica de autoscaling (decisão de quando escalar, quais Pods estão pendentes, drain seguro, PDB).
2. Usar o [cluster-autoscaler](https://github.com/kubernetes/autoscaler) existente com uma extensão para o Proxmox.
3. Usar o mecanismo `externalgrpc` do cluster-autoscaler.

## Decisão

**`externalgrpc` cloud provider do cluster-autoscaler.**

O cluster-autoscaler suporta um provider externo que se comunica por gRPC. O `proxmox-k3s` implementa um servidor gRPC que expõe a API `NodeGroups`, `NodeGroupIncreaseSize`, `NodeGroupDeleteNodes`, etc. O cluster-autoscaler é deployado no cluster e se comunica com esse servidor.

## Por que não reimplementar do zero

A parte difícil do autoscaling não é chamar a API do Proxmox para criar um nó — é decidir *quando* criar: simular o scheduling para saber quais Pods ficariam running com um worker adicional, respeitar PDBs no scale-down, evitar loops de scale-up quando o problema é outro. O cluster-autoscaler faz isso há anos. Reimplementar seria meses de trabalho para chegar num resultado pior.

## Por que não fork do cluster-autoscaler

O CA tem providers in-tree (AWS, GCP, Azure, Hetzner). Um fork Proxmox exigiria rebase periódico contra o upstream, e o usuário instalaria um fork não-oficial. Não faz sentido.

## Por que `externalgrpc`

O CA suporta providers externos via gRPC. O `proxmox-k3s` implementa o servidor gRPC — que reutiliza o mesmo `InfraProvider` do `usecase`, sem duplicar nada — e o CA upstream cuida de todo o motor de decisão. Sem fork, sem rebase, código no nosso repositório.

## Impacto no design atual (por isso está documentado no MVP)

Para que o autoscaling funcione sem refatoração na v2, o MVP precisa:

- **`CreateNode`/`DeleteNode` por-nó** em vez de um `ApplyCluster` monolítico — o CA chama operações por nó, não "sync completo do cluster".
- **IPAM determinístico por índice** — com um processo autônomo criando nós, um pool aleatório geraria conflito. A alocação por `(pool, índice)` é estável.
- **`ProviderID` consistente** — `proxmox://{hostname}/{vmid}` — para o CA correlacionar nó K8s ↔ VM Proxmox.
- **Descoberta sem state file** — com o CA criando nós, um state file local estaria permanentemente desatualizado. Tags do Proxmox como verdade já resolve.
- **Schema do YAML com `autoscaling: {enabled, min, max}`** em cada pool — campos reservados no MVP, inertes até v2.

## Riscos

| Risco | Mitigação |
|---|---|
| Clone de template leva minutos — scale-up reativo é lento | Pool de VMs pré-aquecidas (criadas e desligadas) em v2; ligar é muito mais rápido que clonar |
| Scale-down com PVs locais no nó | CA respeita PDBs; `proxmox-csi` com topology-aware scheduling reduz o problema |
| API `externalgrpc` pode mudar entre versões do CA | Spike de validação obrigatório antes de começar a v2; fixar versão mínima do CA suportada |

## Spike obrigatório (antes de commitar a implementação)

Validar:
- Estabilidade atual da API `externalgrpc` do cluster-autoscaler
- Versão mínima do CA que suporta `externalgrpc` de forma estável
- Testar o ciclo de vida completo com um servidor gRPC mock antes de implementar o real

Esse é o único ponto do plano que ainda não foi verificado em código. O spike acontece no início da v2, não antes.
