# ADR-001 — Go em vez de Crystal

**Data:** 2026-07-28  
**Status:** ✅ Aceito

## Contexto

O projeto de referência (`hetzner-k3s`) é escrito em Crystal — uma linguagem de tipagem estática com sintaxe Ruby-like. Crystal é uma escolha válida: gera binários rápidos e tem boa biblioteca padrão. No entanto, o `proxmox-k3s` é uma nova implementação, não um fork, e a escolha de linguagem está em aberto.

## Decisão

**Go.**

## Razões

O ecossistema Kubernetes é Go: `client-go`, `x/crypto/ssh`, `helm/v3`, `controller-runtime`. Em Crystal, a maioria dessas bibliotecas não existe ou é imatura. Isso sozinho teria sido suficiente.

Go compila para binário estático (`CGO_ENABLED=0`) — o usuário não instala nada além do binário. `GOOS=linux GOARCH=arm64 go build` funciona sem container. Crystal tem o mesmo objetivo, mas o ecossistema é incomparavelmente menor — tanto para o projeto em si quanto para quem vai contribuir.

O sistema de interfaces estruturais do Go torna trivial criar fakes de `InfraProvider` e `CommandExecutor` nos testes unitários sem um único mock framework.

## Trade-offs

Go é mais verboso que Crystal. Para um projeto com lógica rica de domínio isso aparece — mas é um custo aceitável dado o ecossistema. O hetzner-k3s em Crystal não pode ser reaproveitado diretamente, mas esse nunca foi o objetivo.

## Alternativas consideradas

- **Crystal:** Eliminada pelos motivos acima (ecossistema, cross-compilation, contratação).
- **Rust:** Performance superior, mas curva de aprendizado alta, ecossistema Kubernetes imaturo, e o problema aqui não é CPU-bound — é latência de API/SSH.
- **Python:** Interpretado, exige runtime, tipo de erro só em runtime. Não adequado para uma CLI de infra.
- **Terraform Provider:** Seria uma extensão do ecossistema existente, não um framework. Herda o ciclo de vida do state do Terraform e a dependência do runtime. Ver ADR-002.
