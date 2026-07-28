# ADR-007 — Extensibilidade para outros provedores

**Data:** 2026-07-28  
**Status:** ✅ Aceito

## Contexto

O projeto não deve ficar acoplado ao Proxmox para sempre. Ao mesmo tempo, abstração prematura de infra é uma armadilha clássica: Proxmox é assíncrono e por-host; Hetzner é síncrono e regional; vSphere tem clusters, resource pools e datastores. Uma interface desenhada para um provedor hipotético genérico frequentemente não serve bem a nenhum deles.

## Decisão

**Separar em camadas, mas não sobre-abstrair no MVP.** A interface `InfraProvider` existe desde o primeiro dia porque organiza o código e torna o `usecase` testável — não porque planejamos N provedores imediatamente. O caminho é:

1. **MVP**: `InfraProvider` com uma implementação (Proxmox). Interface ditada pelo que o `usecase` precisa.
2. **v1**: adapter `external-nodes` (bare-metal/SSH já existente). Valida a abstração com custo baixo e entrega valor real (cluster com nós físicos é caso de uso comum em homelab).
3. **v2**: segundo provider real (vSphere ou XCP-ng), com refatoração informada por dois casos concretos.

## Interface `InfraProvider`

```go
// internal/domain/ports/infra.go

type InfraProvider interface {
    // Identificação
    Name() string
    Capabilities() Capabilities

    // Validação (sem criar nada)
    Preflight(ctx context.Context, cluster *Cluster) error

    // Descoberta (sem criar nada)
    ListNodes(ctx context.Context, clusterName string) ([]Node, error)

    // Ciclo de vida
    CreateNode(ctx context.Context, spec NodeSpec) (Node, error)
    WaitReady(ctx context.Context, node Node) (Node, error) // devolve o nó com IP observado
    DeleteNode(ctx context.Context, node Node) error
}

type Capabilities struct {
    SupportsManagedLoadBalancer bool   // Hetzner: true; Proxmox: false → usecase usa kube-vip
    SupportsVolumeSnapshot      bool
    MaxParallelProvisions       int    // limite seguro de provisionamento paralelo
}
```

**`Capabilities()` é a válvula de escape honesta.** Em vez de fingir que todo provedor tem LB gerenciado, o `usecase` pergunta e decide. Isso é melhor que `if provider == "proxmox"` no core.

## Mecanismos de desacoplamento

1. **`ProviderID` opaco**: `proxmox://pve1/103`. O domínio nunca interpreta — é apenas uma string de identidade.
2. **Config por bloco**: `proxmox: {...}` isolado no YAML; o resto é agnóstico de provedor.
3. **Registry de providers**: `provider: proxmox` no YAML → factory no composition root. Adicionar um provider = novo pacote em `adapter/` + registro em `cmd/`.
4. **Teste de contrato compartilhado**: suíte que qualquer `InfraProvider` deve passar. Previne a abstração de virar ficção.

## Por que `external-nodes` como segundo adapter

O adapter de nós externos (máquinas já existentes, sem criar VMs) é o segundo caso mais barato de implementar. Valida que a interface funciona com um provedor que tem semântica completamente diferente (sem clone, sem cloud-init, sem task polling) com custo de implementação baixo. Se a abstração quebrar aqui, é o momento certo para refatorar — antes de comprometer com vSphere.

## Alternativas consideradas

- **Plugin system (`.so` dinâmico)**: Complexidade extrema, problemas de compatibilidade de versão do Go, não necessário.
- **Interface massiva cobrindo todos os casos**: Abstração vazada. Cada provider precisaria implementar métodos que não fazem sentido para ele (ex.: `CreateNetwork` para bare-metal).
- **Abstrair desde o MVP para N provedores**: Clássico over-engineering. Provedores reais diferem em formas que só aparecem na implementação.
