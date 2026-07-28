# Roadmap

> Projeto pessoal, tempo parcial (~10–15 h/semana). As datas são estimativas, não compromissos.
> O escopo de cada fase pode mudar conforme o desenvolvimento avança.

## Fases

### Fase 0 — Documentação e fundação pública
**Status:** 🔄 Em andamento  
**Previsão:** Julho 2026

Repositório público desde o início porque uma ferramenta que provisiona acesso root em máquinas alheias precisa ter o código aberto para ser adotada. A documentação veio antes do código: se não dá pra escrever o README, o desenho não está pronto.

**Entregáveis:**
- [ ] README completo com quickstart, arquitetura e comparação com hetzner-k3s
- [ ] ROADMAP com fases, critérios de sucesso e estimativas
- [ ] `docs/architecture.md` — camadas, decisões e trade-offs
- [ ] `docs/adr/` — Architecture Decision Records numerados
- [ ] `docs/configuration.md` — referência completa do YAML
- [ ] `docs/proxmox-setup.md` — setup passo a passo do Proxmox
- [ ] `examples/` — YAMLs comentados (HA, DHCP, simples)
- [ ] `CONTRIBUTING.md`, `SECURITY.md`, `CODE_OF_CONDUCT.md`
- [ ] `LICENSE` (Apache 2.0)
- [ ] `.github/` — templates de issue/PR, dependabot, CI stub

**Critério de sucesso:** alguém que nunca viu o projeto entende o que ele faz, o que ainda não faz, quando pretende fazer e como contribuir — sem precisar ler código.

---

### MVP / v0.1 — "Um comando cria um cluster HA utilizável"
**Status:** 📅 Planejado  
**Previsão:** Setembro 2026

O núcleo do projeto: dado um YAML válido, subir um cluster K3s em HA do zero, com rollback limpo se algo der errado.

**Escopo:**
- `proxmox-k3s create` — provisionamento completo
- `proxmox-k3s delete` — destruição completa, sem órfãos
- `proxmox-k3s validate` — validação estática e de preflight sem criar nada
- `proxmox-k3s sizes` — catálogo de tamanhos com uso de recursos
- `proxmox-k3s template validate` — verificação do template cloud-init
- Client HTTP do Proxmox VE com task polling (UPID), retry e erros tipados
- Catálogo de tamanhos com mínimos aplicados no preflight
- IPAM: modo estático (pool declarativo) e DHCP (qemu-guest-agent)
- Provisionamento paralelo de VMs com semáforo de clone por host
- Preflight forte: credenciais, template, storage, capacidade, IPs livres, VIP
- Rollback transacional das VMs criadas na sessão corrente
- K3s HA: 3+ masters com etcd embutido
- kube-vip como static pod (VIP L2/ARP)
- Workers com labels e taints por pool
- MetalLB (L2) com pool de IPs configurável
- Kubeconfig com merge em `~/.kube/config`
- Logs paralelos prefixados por nó
- Idempotência: re-executar `create` é sempre seguro

**Fora do MVP:** `upgrade`, `scale`, CSI/CCM, Cilium, autoscaling, painel web.

**Riscos técnicos:**

| Risco | Probabilidade | Mitigação |
|---|---|---|
| kube-vip depende de ARP gratuito na L2 | Médio | Preflight detecta; flag `--single-master` como escape |
| Clones simultâneos saturam storage e geram locks | Alto | Semáforo por host (default: 2), retry no lock error |
| Divergência de comportamento entre PVE 8.x e 9.x | Médio | Suporte declarado a PVE 8.0+, verificação de `/version` |
| qemu-guest-agent ausente/lento no template | Médio | `template validate` obrigatório; timeout generoso + mensagem clara |
| Conflito de IP na LAN (modo estático) | Baixo | Preflight com ping+ARP de todos os IPs do pool e VIP |
| Usuário dimensionar VMs abaixo do viável | Alto | Catálogo + mínimos que **falham** o preflight, não apenas avisam |

**Critérios de sucesso:**
- `create` em Proxmox limpo → cluster 3+3 nós Ready em < 10 min
- `delete` não deixa nenhuma VM com a tag `k3s-cluster` do cluster
- Derrubar qualquer master → `kubectl` continua via VIP em < 30s
- `Service type: LoadBalancer` recebe IP da LAN via MetalLB
- Re-executar `create` em cluster existente → no-op
- Falha na fase de provisionamento → nenhuma VM órfã
- Config abaixo dos mínimos → preflight falha com mensagem acionável
- Cobertura de testes do `usecase` ≥ 70% com fakes

---

### v1 — "Produção e operação contínua"
**Status:** 📅 Planejado  
**Previsão:** Janeiro 2027

Cluster funcionando, agora precisa ser operável no dia a dia.

**Escopo:**
- `proxmox-k3s upgrade` — rolling upgrade do K3s via system-upgrade-controller (masters → workers, com drain e health gates)
- `proxmox-k3s scale` — adicionar/remover nós por pool (drain + cordon antes de remover)
- `proxmox-k3s status` — reconciliador exposto: diff entre config desejada e realidade
- `proxmox-k3s template build` — download de cloud image, configuração e conversão em template
- Addon: [proxmox-cloud-controller-manager](https://github.com/sergelogvinov/proxmox-cloud-controller-manager) — topology labels, remoção de taint `uninitialized`
- Addon: [proxmox-csi-plugin](https://github.com/sergelogvinov/proxmox-csi-plugin) — PVs reais no storage do Proxmox
- CNI Cilium como alternativa ao Flannel
- Adapter `external-nodes`: nós SSH/bare-metal já existentes sem criação de VM (valida a abstração de provider)
- Backup do etcd para storage do Proxmox antes de upgrades
- Hardening dos nós via nftables configurado por cloud-init
- GoReleaser: binários multi-arch, Homebrew tap, pacote .deb/.rpm
- Documentação completa de operação

**Riscos técnicos:**

| Risco | Mitigação |
|---|---|
| Upgrade do etcd embutido (operação mais perigosa) | Snapshot obrigatório, um master por vez, health gate entre etapas |
| CSI exige topology labels antes de provisionar PVs | CCM obrigatório quando CSI está ativo; validação no preflight |
| `scale down` com PVs locais no nó removido | Drain bloqueado pelo PDB; mensagem explícita com instrução de migração |

**Critérios de sucesso:**
- Upgrade de patch e de minor sem downtime da API do cluster
- Scale up/down sem perda de workload
- PVC provisionado e montado em Pod com dados persistentes após restart
- Primeiros contribuidores externos conseguindo rodar o e2e

---

### v2 — "Plataforma"
**Status:** 📅 Planejado  
**Previsão:** Junho 2027

Autoscaling real, painel web e o projeto começa a se abrir para outros provedores.

**Escopo:**
- **Autoscaling** via servidor `externalgrpc` do [cluster-autoscaler](https://github.com/kubernetes/autoscaler): Pods pendentes por falta de capacidade → cria workers automaticamente; workers ociosos → drena e remove
  - Pool de VMs pré-aquecidas (criadas e desligadas) para reduzir latência de escala reativa
  - Spike de validação da API `externalgrpc` antes de commitar com essa via
- **Painel web** (`proxmox-k3s ui`): servidor HTTP local com frontend embutido no binário
  - Visão do cluster: nós, IPs, VMIDs, hosts PVE, estado K8s, métricas de recursos
  - Editor do YAML com validação em tempo real
  - Plan/apply visual (diff do reconciliador antes de agir)
  - Streaming de logs por nó (SSE)
  - Scale por slider com preview de consumo de recursos
  - Nenhuma chamada para serviço externo; roda com suas credenciais locais
- **Segundo provider** (vSphere ou XCP-ng): valida e refina a interface `InfraProvider` com dois casos reais
- Datastore externo (Postgres/MySQL) para o K3s
- Instalação air-gapped (mirror URL do K3s + imagens em registry local)
- Firewall do Proxmox VE via API (complemento ao nftables)
- Snapshot e restore de cluster
- Addons plugáveis por manifests do usuário
- Modo biblioteca: `pkg/` público para uso como SDK

**Critérios de sucesso:**
- Deployment com réplicas além da capacidade → novo worker criado em < 5 min, scale-down automático após ociosidade, respeitando PDBs
- Painel executa create/scale/upgrade sem terminal após o primeiro uso
- Segundo provider passa a suíte de contrato sem alterar `usecase`

---

## O que não está no roadmap (e por quê)

| Item | Motivo |
|---|---|
| Operador Kubernetes para gerenciar clusters | Cria dependência circular (a UI morre junto com o cluster); avaliado para v3 |
| Suporte a Windows Nodes | Fora do scope homelab/on-prem do projeto |
| Integração com ArgoCD/Flux pós-criação | O cluster entregue já é compatível; automação de GitOps é responsabilidade da camada acima |
| Provider Terraform para proxmox-k3s | O projeto é a alternativa ao Terraform, não um plugin dele |

---

## Como acompanhar o progresso

- [Issues abertas](https://github.com/nexusops/proxmox-k3s/issues) — bugs e features
- [Milestones](https://github.com/nexusops/proxmox-k3s/milestones) — agrupamento por fase
- [Discussions](https://github.com/nexusops/proxmox-k3s/discussions) — perguntas e ideias
- [CHANGELOG.md](CHANGELOG.md) — mudanças por versão (criado no primeiro release)
