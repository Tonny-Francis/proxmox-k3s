# Security Policy

## Versões suportadas

| Versão | Suporte de segurança |
|---|---|
| `main` (pre-alpha) | ✅ Recebe correções |
| versões anteriores | ❌ Não suportadas |

## Reportando vulnerabilidades

`proxmox-k3s` lida com credenciais de hypervisor (API tokens do Proxmox VE com permissões de administração de VMs). Vulnerabilidades de segurança neste projeto têm impacto real.

**Não abra issues públicas para vulnerabilidades de segurança.**

Use o **GitHub Private Security Advisory**: aba _Security_ → _Report a vulnerability_. Se não tiver acesso ao GitHub, envie um e-mail para o endereço listado no perfil do mantenedor com o assunto `[SECURITY] proxmox-k3s`.

Inclua no report: versão afetada, passos para reproduzir e impacto estimado. Confirmação em até 72 h.

## Decisões de segurança relevantes

- Tokens apenas via variável de ambiente (`${VAR}`) — nunca em plaintext no YAML
- `insecure_skip_tls_verify` exibe warning explícito; não é o default
- Role `K3sProvisioner` com o menor conjunto de permissões necessário — ver `docs/proxmox-setup.md`
- `gitleaks` em todo PR
- O binário não envia telemetria e não faz requisições para fora da LAN (exceto GitHub Releases para resolver versão do K3s)
