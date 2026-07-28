# ADR-002 — Sem Terraform e sem Ansible

**Data:** 2026-07-28  
**Status:** ✅ Aceito

## Contexto

As duas alternativas mais comuns para automação de infraestrutura Proxmox são:

- **Terraform** com o provider [`bpg/proxmox`](https://github.com/bpg/terraform-provider-proxmox) para criar as VMs.
- **Ansible** com o módulo `community.general.proxmox_kvm` para configurar os nós.

Ambas são ferramentas maduras e amplamente usadas. A questão é se elas agregam valor neste projeto ou adicionam complexidade.

## Decisão

**Nem Terraform nem Ansible.** Go puro, binário único, sem dependências de runtime para o usuário final.

## Por que não Terraform

O principal problema não é o Terraform em si, é o state file. O usuário teria que gerenciar onde guardar, como fazer backup, o que fazer quando fica desatualizado. O `proxmox-k3s` usa as tags do Proxmox como identidade dos recursos e não mantém estado local — ver ADR-003.

Além disso, o pipeline de provisionamento tem lógica condicional, rollback transacional e polling de tasks assíncronas do Proxmox que não cabem bem no modelo declarativo do Terraform. E um binário Go sem dependências externas é significativamente mais fácil de instalar e usar em ambientes air-gapped.

## Por que não Ansible

Ansible quebra o modelo de binário único: exige Python no ambiente do usuário, na versão certa, com os módulos corretos instalados. Scripts bash embutidos no binário via `embed.FS` são mais fáceis de debugar — você pode copiar o script exato que rodou e reexecutar manualmente.

O argumento principal para usar Ansible é idempotência. O `proxmox-k3s` é idempotente por design no `usecase` — re-executar `create` descobre o que já existe e não recria.

## Quando reconsiderar

- Se a superfície de configuração dos nós crescer a ponto de scripts bash se tornarem unmaintainable (> ~500 linhas por tipo de nó). Nesse caso, avaliar scripts bash estruturados ou um cliente de configuração em Go.
- Se houver demanda real de suporte a Windows Nodes (Ansible tem melhor suporte cross-platform).

## Alternativas consideradas

- **Pulumi (Go SDK):** Mais próximo do modelo de código, mas ainda carrega o conceito de state e engine. A simplicidade do cliente HTTP próprio vence aqui.
- **Terraform + Ansible:** A combinação mais comum em prod. Válida para uso direto, mas não como framework que outros vão usar — duas dependências a mais, duas fontes de problemas a mais.
