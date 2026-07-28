package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/nexusops/proxmox-k3s/internal/domain"
)

func newSizesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "sizes",
		Short: "Lista o catálogo de tamanhos de VM com recursos e mínimos",
		RunE: func(cmd *cobra.Command, args []string) error {
			sizes := domain.AllSizes()

			fmt.Printf("%-14s  %5s  %8s  %7s  %s\n", "NOME", "vCPU", "MEMÓRIA", "DISCO", "USO TÍPICO")
			fmt.Println("──────────────────────────────────────────────────────────────────")

			for _, s := range sizes {
				mem := formatMem(s.MemoryMB)
				fmt.Printf("%-14s  %5d  %8s  %5dGB  %s\n", s.Name, s.CPU, mem, s.DiskGB, sizeNote(s.Name))
			}

			fmt.Println("")
			fmt.Println("Mínimos absolutos (preflight falha se abaixo desses valores):")
			fmt.Println("  Masters:  2 vCPU  /  4 GB  /  20 GB")
			fmt.Println("  Workers:  2 vCPU  /  2 GB  /  20 GB")
			fmt.Println("")
			fmt.Println("Para ver o consumo total de um cluster, use:")
			fmt.Println("  proxmox-k3s validate -c cluster.yaml --skip-remote")

			return nil
		},
	}
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Mostra a versão do proxmox-k3s",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("proxmox-k3s %s\n", version)
		},
	}
}

const version = "v0.0.1-pre-alpha"

func formatMem(mb int) string {
	if mb >= 1024 {
		gb := mb / 1024
		return fmt.Sprintf("%d GB", gb)
	}
	return fmt.Sprintf("%d MB", mb)
}

func sizeNote(name string) string {
	notes := map[string]string{
		"cp-small":   "Control plane, clusters pequenos (< 20 nós)",
		"cp-medium":  "Control plane, uso geral (recomendado)",
		"cp-large":   "Control plane, clusters grandes",
		"standard-2": "Worker mínimo viável",
		"standard-4": "Worker, uso geral (recomendado)",
		"standard-8": "Worker de alta capacidade",
		"memory-4":   "Banco de dados, cache (Redis, Postgres)",
		"memory-8":   "Banco de dados, cache intensivo",
		"cpu-8":      "Build, CI, processamento batch",
	}
	if note, ok := notes[name]; ok {
		return note
	}
	return ""
}
