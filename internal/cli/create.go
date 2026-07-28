package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	proxmoxadapter "github.com/nexusops/proxmox-k3s/internal/adapter/proxmox"
	"github.com/nexusops/proxmox-k3s/internal/config"
	"github.com/nexusops/proxmox-k3s/internal/config/validate"
	"github.com/nexusops/proxmox-k3s/internal/usecase/create"
)

func newCreateCmd() *cobra.Command {
	var configPath string
	var noRollback bool

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Cria um cluster K3s no Proxmox VE",
		Long: `Lê a configuração YAML e provisiona um cluster K3s em alta disponibilidade.

Re-executar create em um cluster existente é seguro — o reconciliador descobre
o que já existe e só cria o que falta.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(configPath)
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}

			if cfg.Proxmox.InsecureSkipTLS {
				fmt.Fprintln(os.Stderr, "AVISO: insecure_skip_tls_verify=true — conexão TLS não verificada")
			}

			if err := validate.Cluster(cfg); err != nil {
				return err
			}

			cluster, err := config.ToModel(cfg)
			if err != nil {
				return fmt.Errorf("converting config to model: %w", err)
			}

			provider, err := proxmoxadapter.NewProvider(cluster.Proxmox)
			if err != nil {
				return fmt.Errorf("initializing Proxmox provider: %w", err)
			}

			ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer cancel()

			pipeline := create.NewPipeline(provider, os.Stdout)
			if err := pipeline.Run(ctx, cluster); err != nil {
				if !noRollback {
					fmt.Fprintf(os.Stderr, "\nErro durante provisionamento: %v\n", err)
					fmt.Fprintf(os.Stderr, "Use --no-rollback para preservar o estado e debugar\n")
				}
				return err
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&configPath, "config", "c", "cluster.yaml", "caminho para o arquivo de configuração YAML")
	cmd.Flags().BoolVar(&noRollback, "no-rollback", false, "não remover VMs criadas em caso de falha")
	cmd.MarkFlagRequired("config")

	return cmd
}
