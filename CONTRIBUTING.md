# Contribuindo

## Formas de contribuir

- **Reportar bugs** — abra uma issue com o template de bug report, incluindo versão do `proxmox-k3s`, versão do PVE e o output completo do comando com `--debug`
- **Testar** — o projeto é testado principalmente num servidor físico com PVE 8.x; feedback sobre outras topologias (versões diferentes do PVE, múltiplos hosts, VLANs corporativas) é valioso
- **Documentação** — melhorias nos exemplos, erros na doc, guias de casos de uso específicos
- **Código** — novas funcionalidades, correções, novos tamanhos no catálogo
- **ADRs** — se você tem uma opinião técnica fundamentada sobre uma decisão arquitetural, abra uma issue de discussão antes de propor mudanças

## Setup de desenvolvimento

### Requisitos

- Go 1.22 ou superior
- `golangci-lint` para lint
- Acesso a um Proxmox VE para testes de integração (opcional para o ciclo básico)

```bash
# Instalar Go: https://go.dev/dl/
go version  # deve mostrar 1.22+

# Instalar golangci-lint
curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh \
  | sh -s -- -b $(go env GOPATH)/bin

# Clonar e buildar
git clone https://github.com/nexusops/proxmox-k3s
cd proxmox-k3s
go build ./cmd/proxmox-k3s/
```

### Estrutura do projeto

Antes de contribuir, leia [docs/architecture.md](docs/architecture.md). A organização segue Clean Architecture — saber onde cada tipo de código vai evita PRs que misturam camadas.

Regra prática:
- Lógica de negócio → `internal/usecase/` (sem dependências externas)
- Código do Proxmox → `internal/adapter/proxmox/` (só essa camada fala com a API)
- Entidades e interfaces → `internal/domain/` (sem imports externos além da stdlib)
- CLI e output → `internal/cli/` (sem lógica, só orquestração)

### Rodando os testes

```bash
# Unitários (sem dependência de infra)
go test ./internal/...

# Com cobertura
go test ./internal/... -coverprofile=coverage.out
go tool cover -html=coverage.out

# Lint
golangci-lint run ./...
```

### Testes de integração (requer Proxmox)

Os testes de integração são opt-in e requerem um Proxmox real:

```bash
export PROXMOX_E2E=1
export PROXMOX_ENDPOINT=https://pve.local:8006
export PROXMOX_TOKEN=seu-token
export PROXMOX_TEMPLATE=ubuntu-2404-cloudinit
export PROXMOX_STORAGE=local-lvm

go test ./e2e/... -timeout 30m
```

## Padrões de código

### Commits

Formato convencional:

```
<type>(<scope>): <description>

[body opcional]

[footer opcional]
```

Tipos: `feat`, `fix`, `docs`, `test`, `refactor`, `chore`

Exemplos:
```
feat(proxmox): add task polling with configurable timeout
fix(ipam): handle CIDR boundary in static pool allocation
docs(adr): add ADR-009 for kube-vip BGP mode consideration
```

### Erros

Erros do domínio são tipados e ficam em `internal/domain/errors.go`. Erros de adapter são convertidos para erros de domínio antes de cruzar a camada. Mensagens de erro devem ser acionáveis — o usuário deve saber o que fazer, não apenas o que aconteceu.

```go
// Bom
return fmt.Errorf("template %q not found in storage %q: verify with 'qm list | grep %s'", name, storage, name)

// Ruim
return fmt.Errorf("not found")
```

### Testes

Novos comportamentos no `usecase` precisam de teste com fakes (`internal/domain/ports/`). Comportamento do adapter Proxmox pode ser testado com `httptest.Server` usando fixtures JSON.

Não é necessário testar o rendering de output da CLI.

## Propondo novas funcionalidades

Para mudanças grandes (novo comando, novo addon, novo provider), abra uma issue de discussão antes de escrever código. Isso evita trabalho duplicado e garante alinhamento arquitetural.

Para novos tamanhos no catálogo, abra um PR com a adição em `internal/domain/catalog/sizes.go` e a justificativa do caso de uso no corpo do PR.

Para decisões arquiteturais, proponha um ADR em `docs/adr/`. O template está em `docs/adr/000-template.md`.

## Processo de PR

1. Fork + branch com nome descritivo (`feat/metallb-ipv6`, `fix/vmid-collision`)
2. Testes passando localmente
3. Lint sem erros
4. Descrição do PR explicando o porquê da mudança, não apenas o quê
5. Para mudanças na API de configuração, atualizar `docs/configuration.md`
6. Para decisões arquiteturais, incluir ou referenciar o ADR correspondente

## Dúvidas

Use [Discussions](https://github.com/nexusops/proxmox-k3s/discussions) para perguntas abertas. Issues são para bugs e feature requests bem definidos.
