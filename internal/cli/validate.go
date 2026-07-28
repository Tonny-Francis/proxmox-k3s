package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	proxmoxadapter "github.com/nexusops/proxmox-k3s/internal/adapter/proxmox"
	"github.com/nexusops/proxmox-k3s/internal/config"
	"github.com/nexusops/proxmox-k3s/internal/config/validate"
)

func newValidateCmd() *cobra.Command {
	var configPath string
	var skipRemote bool

	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Valida a configuração sem criar nada",
		Long: `Roda todas as validações estáticas e, por padrão, o preflight contra o Proxmox.
Use --skip-remote para validar só a sintaxe do YAML sem conectar ao Proxmox.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(configPath)
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}

			fmt.Println("Validando configuração...")
			if err := validate.Cluster(cfg); err != nil {
				return err
			}
			fmt.Println("✓ Configuração YAML válida")

			if skipRemote {
				fmt.Println("✓ (preflight remoto ignorado com --skip-remote)")
				return nil
			}

			cluster, err := config.ToModel(cfg)
			if err != nil {
				return err
			}

			provider, err := proxmoxadapter.NewProvider(cluster.Proxmox)
			if err != nil {
				return fmt.Errorf("initializing Proxmox provider: %w", err)
			}

			fmt.Println("Executando preflight no Proxmox...")
			if err := provider.Preflight(context.Background(), cluster); err != nil {
				return fmt.Errorf("preflight failed: %w", err)
			}

			fmt.Println("✓ Preflight OK — credenciais, template e hosts validados")
			fmt.Println("\nCluster pronto para ser criado com 'proxmox-k3s create -c", configPath+"'")

			_ = os.Stdout
			return nil
		},
	}

	cmd.Flags().StringVarP(&configPath, "config", "c", "cluster.yaml", "caminho para o arquivo de configuração YAML")
	cmd.Flags().BoolVar(&skipRemote, "skip-remote", false, "valida só o YAML, sem conectar ao Proxmox")
	cmd.MarkFlagRequired("config")

	return cmd
}
