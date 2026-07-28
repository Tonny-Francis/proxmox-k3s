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
	deleteusecase "github.com/nexusops/proxmox-k3s/internal/usecase/delete"
)

func newDeleteCmd() *cobra.Command {
	var configPath string
	var force bool

	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Remove um cluster K3s e todas as suas VMs do Proxmox",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(configPath)
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}

			if err := validate.Cluster(cfg); err != nil {
				return err
			}

			cluster, err := config.ToModel(cfg)
			if err != nil {
				return err
			}

			if !force {
				fmt.Printf("Isso vai remover todas as VMs do cluster %q. Confirmar? [y/N] ", cluster.Name)
				var answer string
				fmt.Scanln(&answer)
				if answer != "y" && answer != "Y" {
					fmt.Println("Cancelado.")
					return nil
				}
			}

			provider, err := proxmoxadapter.NewProvider(cluster.Proxmox)
			if err != nil {
				return fmt.Errorf("initializing Proxmox provider: %w", err)
			}

			ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer cancel()

			pipeline := deleteusecase.NewPipeline(provider, os.Stdout)
			return pipeline.Run(ctx, cluster)
		},
	}

	cmd.Flags().StringVarP(&configPath, "config", "c", "cluster.yaml", "caminho para o arquivo de configuração YAML")
	cmd.Flags().BoolVar(&force, "force", false, "não pedir confirmação")
	cmd.MarkFlagRequired("config")

	return cmd
}
