package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/user/bifrost/internal/crypto"
	"github.com/user/bifrost/internal/tunnel"
)

func newServerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "server",
		Short: "Inicia el demonio del servidor Tunnel",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf(`
██████╗ ██╗███████╗██████╗  ██████╗ ███████╗████████╗
██╔══██╗██║██╔════╝██╔══██╗██╔═══██╗██╔════╝╚══██╔══╝
██████╔╝██║█████╗  ██████╔╝██║   ██║███████╗   ██║   
██╔══██╗██║██╔══╝  ██╔══██╗██║   ██║╚════██║   ██║   
██████╔╝██║██║     ██║  ██║╚██████╔╝███████║   ██║   
╚═════╝ ╚═╝╚═╝     ╚═╝  ╚═╝ ╚═════╝ ╚══════╝   ╚═╝   			

 Version %s - Desarrollador: Hernan Lencinas
 
`, getAppVersion())

			// Leemos el archivo de configuración con Viper
			viper.SetConfigFile("config/server.conf")
			viper.SetConfigType("json")

			// Si el archivo existe, lo lee
			if err := viper.ReadInConfig(); err != nil {
				slog.Error("Fallo intentando iniciar el servidor: No se encontró o no se pudo leer el archivo 'config/server.conf'.", "error", err)
				os.Exit(1)
			}
			slog.Info("Cargando configuración desde archivo", "file", viper.ConfigFileUsed())

			// Armamos la dirección de escucha obligatoria desde el archivo
			address := viper.GetString("server.address")
			port := viper.GetString("server.port")
			if address == "" || port == "" {
				slog.Error("Fallo intentando iniciar el servidor: El archivo .conf debe contener 'server.address' y 'server.port'")
				os.Exit(1)
			}
			addr := address + ":" + port

			secret := viper.GetString("server.secret")
			if secret == "" {
				slog.Error("Fallo intentando iniciar el servidor: El archivo .conf debe contener el 'server.secret' autenticador")
				os.Exit(1)
			}

			// Proceso de desencriptación automática si detectamos prefijo de base64
			if strings.HasPrefix(secret, "ENC:") {
				decrypted, err := crypto.Decrypt(strings.TrimPrefix(secret, "ENC:"))
				if err != nil {
					slog.Error("Fallo intentando desencriptar el secret. El servidor no iniciará.", "error", err)
					os.Exit(1)
				}
				secret = decrypted
			}

			s := tunnel.NewServer(tunnel.ServerOptions{
				Addr:  addr,
				Token: secret,
			})

			ctx, cancel := context.WithCancel(context.Background())
			go func() {
				// Captura la señal de SIGINT (Ctrl+C) y apaga graciosamente.
				c := make(chan os.Signal, 1)
				signal.Notify(c, os.Interrupt, syscall.SIGTERM)
				<-c
				cancel()
			}()

			if err := s.Start(ctx); err != nil {
				slog.Error("El proceso del servidor terminó debido a un error crítico", "error", err)
			}
		},
	}

	return cmd
}
