package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/cobra"
)

func initLogger() {
	handler := newCustomHandler(os.Stderr, slog.HandlerOptions{
		Level: slog.LevelInfo,
	}, false)
	slog.SetDefault(slog.New(handler))
}

func main() {
	initLogger()

	var rootCmd = &cobra.Command{
		Use:   "bifrost",
		Short: "Bifrost es un túnel TCP rápido sobre WebSockets",
		Long:  `Una herramienta robusta para Reverse y Local Port Forwarding sobre conexiones HTTP seguras.`,
		Run: func(cmd *cobra.Command, args []string) {
			if err := StartInteractiveUI(); err != nil {
				fmt.Printf("Fallo en Interfaz TUI: %v\n", err)
				os.Exit(1)
			}
		},
	}

	rootCmd.AddCommand(newServerCmd())
	rootCmd.AddCommand(newClientCmd())
	rootCmd.AddCommand(newEncryptCmd())

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
