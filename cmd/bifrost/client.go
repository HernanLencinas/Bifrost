package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/user/bifrost/internal/tunnel"
)

func newClientCmd() *cobra.Command {
	var (
		serverUrl  string
		token      string
		listenAddr string
	)

	cmd := &cobra.Command{
		Use:   "client",
		Short: "Inicia el cliente que se conecta al túnel",
		Run: func(cmd *cobra.Command, args []string) {
			c := tunnel.NewClient(tunnel.ClientOptions{
				ServerURL:  serverUrl,
				Token:      token,
				ListenAddr: listenAddr,
			})

			ctx, cancel := context.WithCancel(context.Background())
			go func() {
				// Cierre controlado ante señales del SO
				ch := make(chan os.Signal, 1)
				signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
				<-ch
				slog.Info("Apagando cliente amablemente...")
				cancel()
			}()

			if err := c.Start(ctx); err != nil {
				slog.Error("El cliente finalizó con un error fatal", "error", err)
			}
		},
	}

	cmd.Flags().StringVarP(&serverUrl, "url", "u", "ws://localhost:8000/tunnel", "La URL WebSocket del Servidor para el túnel")
	cmd.Flags().StringVarP(&token, "token", "t", "super_secreto", "Token para autorizar la conexión")
	cmd.Flags().StringVarP(&listenAddr, "listen", "l", "127.0.0.1:9090", "El puerto local a abrir")

	return cmd
}
