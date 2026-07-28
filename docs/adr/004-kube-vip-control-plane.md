# ADR-004 — kube-vip para HA do API Server

**Data:** 2026-07-28  
**Status:** ✅ Aceito

## Contexto

O K3s em modo HA requer um endpoint único e estável para o API Server — um endereço que funcione independentemente de qual master está rodando. O `hetzner-k3s` resolve isso com um **Load Balancer gerenciado** da Hetzner Cloud. O Proxmox **não oferece LB gerenciado**. Precisamos de uma solução que:

- Funcione em homelab sem hardware dedicado
- Não exija VMs extras para gerenciar
- Seja transparente para o cluster — o `kubeconfig` aponta para um IP fixo
- Sobreviva à falha de qualquer master individual

## Decisão

**`kube-vip` em modo ARP (L2), rodando como static pod** no diretório `/var/lib/rancher/k3s/server/manifests/` de cada master.

O `kube-vip` anuncia um VIP via ARP gratuito. O master eleito lider mantém o VIP em sua interface de rede. Quando o master cai, outro master assume o VIP em segundos. Masters e workers apontam para o VIP — nunca para o IP de um master específico.

## Por que kube-vip

A alternativa mais comum é HAProxy+keepalived em VMs dedicadas. O problema é óbvio: são mais duas VMs para criar, atualizar, destruir e monitorar — e o LB em si fica em single-point-of-failure se o keepalived não estiver configurado direito.

O kube-vip roda como static pod nos próprios masters. O manifest vai em `/var/lib/rancher/k3s/server/manifests/` e o K3s o gerencia automaticamente — ele sobe junto com o kubelet, não depende do cluster estar saudável. O VIP é um campo no YAML; o framework gera o manifest. É a abordagem padrão da comunidade K3s para HA sem LB gerenciado.

## Requisitos e restrições

- **Rede L2 entre masters**: os masters precisam estar na mesma broadcast domain. O ARP gratuito não atravessa roteadores. O preflight valida que o VIP está alcançável pela bridge/VLAN configurada.
- **VIP fora do pool de nós**: o IP do VIP não pode ser atribuído a nenhuma VM. O preflight valida isso e também testa que o VIP não responde a ping antes da criação (conflito de IP).
- **≥ 3 masters**: etcd precisa de quorum. 2 masters não oferecem HA real — uma falha paralisa o cluster. O preflight rejeita `count: 2` com mensagem explicativa.

## Alternativas consideradas

| Alternativa | Por que não |
|---|---|
| HAProxy + keepalived em VMs dedicadas | 2 VMs extras para gerenciar (criar, atualizar, destruir); o LB em si não tem HA se uma das VMs cair sem keepalived, que adiciona complexidade |
| kube-vip em modo BGP | Requer roteador com suporte a BGP e configuração adicional; muito mais complexo para homelab |
| DNS round-robin para os masters | Sem failover real — DNS tem TTL que mantém o IP do master morto em cache |
| Single master (sem HA no MVP) | Usuário pediu HA sempre; mais importante, entregar single-master como default leva o usuário a nunca migrar |
| MetalLB como LB do API Server | MetalLB é para Services do cluster; usá-lo para o API Server antes do cluster existir cria uma dependência circular |

## Escape

Para ambientes onde ARP gratuito é bloqueado (VLANs corporativas com port security, alguns switches gerenciados): flag `--single-master` permite criar um cluster com 1 master e SQLite (sem HA). O preflight detecta se o VIP não responde a ARP e sugere o flag como alternativa, sem falhar silenciosamente.

## Consequências

- O YAML deve declarar `control_plane_vip` obrigatoriamente quando `masters_pool.count > 1`.
- O script de instalação do primeiro master inclui `--tls-san={VIP}` e os SANs de todos os IPs de master.
- O kubeconfig gerado aponta `server: https://{VIP}:6443`, nunca para o IP de um master específico.
- A documentação de pré-requisitos deve deixar claro o requisito de L2.
