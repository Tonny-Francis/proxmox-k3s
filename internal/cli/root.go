package cli

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "proxmox-k3s",
	Short: "Clusters K3s em HA no Proxmox VE — um comando, um arquivo YAML",
	Long: `proxmox-k3s automatiza o ciclo de vida completo de clusters K3s no Proxmox VE.

Sem Terraform. Sem Ansible. Um binário, um YAML.

Documentação: https://github.com/nexusops/proxmox-k3s`,
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.AddCommand(
		newCreateCmd(),
		newDeleteCmd(),
		newValidateCmd(),
		newSizesCmd(),
		newVersionCmd(),
	)
}
