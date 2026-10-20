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
		targetAddr string
		insecure   bool
	)

	cmd := &cobra.Command{
		Use:   "client",
		Short: "Inicia el cliente que se conecta al túnel",
		Run: func(cmd *cobra.Command, args []string) {
			c := tunnel.NewClient(tunnel.ClientOptions{
				ServerURL:          tunnel.NormalizeServerURL(serverUrl, false),
				Token:              token,
				ListenAddr:         listenAddr,
				TargetAddr:         targetAddr,
				InsecureSkipVerify: insecure,
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

	cmd.Flags().StringVarP(&serverUrl, "url", "u", "ws://localhost:8000/tunnel", "URL WebSocket del servidor (completa o host:puerto; se completa esquema y /tunnel)")
	cmd.Flags().StringVarP(&token, "token", "t", "super_secreto", "Token para autorizar la conexión")
	cmd.Flags().StringVarP(&listenAddr, "listen", "l", "127.0.0.1:9090", "El puerto local a abrir")
	cmd.Flags().StringVarP(&targetAddr, "target", "r", "", "Destino remoto resuelto desde la red del servidor (ej: db:5432)")
	cmd.Flags().BoolVarP(&insecure, "insecure", "k", false, "Omitir verificación del certificado TLS (certs autofirmados)")

	return cmd
}
