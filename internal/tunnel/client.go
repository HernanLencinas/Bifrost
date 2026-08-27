package tunnel

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/hashicorp/yamux"
)

// ClientOptions manejan la conexión hacia el Host Remoto de Bifrost.
type ClientOptions struct {
	ServerURL      string
	Token          string
	ListenAddr     string
	TargetAddr     string
	InsecureSkipVerify bool
	OnStatusChange func(status string)
	// OnTraffic se invoca con incrementos de bytes transmitidos (tx) y recibidos (rx)
	// agregados a nivel de túnel. Puede llamarse desde múltiples goroutines.
	OnTraffic func(txDelta, rxDelta int64)
	// OnReconnect se invoca cuando el cliente logra reconectar luego de haber estado conectado.
	OnReconnect func(totalReconnects int64)
	// OnError se invoca cuando ocurre un error relevante en la conexión al servidor.
	OnError func(err error)
}

// Client estructura principal del lado cliente.
type Client struct {
	opts ClientOptions
}

// NewClient instancia el túnel del cliente.
func NewClient(opts ClientOptions) *Client {
	return &Client{opts: opts}
}

// setStatus ejecuta el callback de cambio de estado de este cliente si fue proveido.
func (c *Client) setStatus(status string) {
	if c.opts.OnStatusChange != nil {
		c.opts.OnStatusChange(status)
	}
}

// Start levanta un listener TCP local y, de forma resiliente, mantiene el WebSoket 
// vivo contra el servidor con "exponential backoff".
func (c *Client) Start(ctx context.Context) error {
	c.setStatus("Iniciando Local")
	displayURL := c.opts.ServerURL
	displayURL = strings.TrimPrefix(displayURL, "wss://")
	displayURL = strings.TrimPrefix(displayURL, "ws://")
	displayURL = strings.TrimSuffix(displayURL, "/tunnel")
	displayURL = strings.TrimSuffix(displayURL, "/")

	slog.Info("Iniciando cliente Bifrost hacia", "addr", displayURL)

	// Inicia el listener local (port forwarding).
	ln, err := net.Listen("tcp", c.opts.ListenAddr)
	if err != nil {
		c.setStatus("Fallo Local")
		return fmt.Errorf("local listen: %w", err)
	}
	slog.Info("Escuchando proxy local", "addr", c.opts.ListenAddr)

	var session *yamux.Session
	var reconnects int64
	wasConnected := false

	// Goroutine que escucha el cierre del contexto para liberar el puerto inmediatamente.
	go func() {
		<-ctx.Done()
		ln.Close()
		if session != nil {
			session.Close()
		}
	}()
	
	// Goroutine que administra la conexión persistente con el Servidor
	// (Reconexión con Exponential Backoff).
	go func() {
		backoff := 1 * time.Second
		for {
			if ctx.Err() != nil {
				c.setStatus("Desconectado")
				return
			}
			
			// Si la sesión cae, se auto reconecta.
			if session == nil || session.IsClosed() {
				c.setStatus("Conectando...")
				slog.Debug("Intentando dial de WebSocket", "url", c.opts.ServerURL)
				
				conn, err := c.dialServer(ctx)
				if err != nil {
					if c.opts.OnError != nil {
						c.opts.OnError(err)
					}
					c.setStatus("Error (Reintentando)")
					slog.Warn("Fallo conectando al servidor", "error", err, "reintento_en", backoff)
					time.Sleep(backoff)
					// Exponential Backoff límite
					if backoff < 30*time.Second {
						backoff *= 2
					}
					continue
				}
				
				slog.Info("Conectado exitosamente al servidor Bifrost")
				sess, err := yamux.Client(conn, newYamuxConfig())
				if err != nil {
					if c.opts.OnError != nil {
						c.opts.OnError(err)
					}
					c.setStatus("Error Multiplexor")
					slog.Error("Fallo instanciando cliente Yamux", "error", err)
					conn.Close()
					time.Sleep(backoff)
					continue
				}
				
				session = sess
				if wasConnected {
					reconnects++
					if c.opts.OnReconnect != nil {
						c.opts.OnReconnect(reconnects)
					}
				}
				wasConnected = true
				c.setStatus("Conectado")
				backoff = 1 * time.Second // Reset tras conexión exitosa
			}
			
			// Pequeña pausa para no ciclar vacíamente
			time.Sleep(500 * time.Millisecond)
		}
	}()

	// Ciclo de aceptación Local:
	for {
		localConn, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			slog.Error("Error aceptando conexión tcp local", "error", err)
			continue
		}

		go c.handleLocalConnection(ctx, session, localConn)
	}
}

// dialServer envía la solicitud de HTTP Upgrade y establece el header del token.
func (c *Client) dialServer(ctx context.Context) (net.Conn, error) {
	headers := make(http.Header)
	if c.opts.Token != "" {
		headers.Add("Authorization", strings.TrimSpace(c.opts.Token))
	}
	if c.opts.TargetAddr != "" {
		headers.Add("X-Tunnel-Target", c.opts.TargetAddr)
	}

	dialer := websocket.Dialer{
		HandshakeTimeout:  10 * time.Second,
		EnableCompression: true,
	}

	if c.opts.InsecureSkipVerify {
		dialer.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}

	wsConn, resp, err := dialer.DialContext(ctx, c.opts.ServerURL, headers)
	if err != nil {
		if resp != nil {
			return nil, fmt.Errorf("bad handshake: %d %s - %w", resp.StatusCode, resp.Status, err)
		}
		return nil, err
	}

	return NewWSConn(wsConn), nil
}

// handleLocalConnection toma la conexión entrante, pide un nuevo 'stream'
// al multiplexor, y puentea los bytes de lado a lado.
func (c *Client) handleLocalConnection(ctx context.Context, session *yamux.Session, localConn net.Conn) {
	defer localConn.Close()

	if session == nil || session.IsClosed() {
		slog.Warn("Llegó conexión local pero no hay túnel activo hacia el servidor")
		return
	}

	// Abre un canal virtual en la sesión
	stream, err := session.OpenStream()
	if err != nil {
		slog.Error("No se pudo instanciar un Stream de Yamux", "error", err)
		return
	}
	defer stream.Close()

	// Enlaza el TCP local y el Stream del WebSotcket
	if err := ProxyWithTraffic(
		ctx,
		localConn,
		stream,
		func(n int64) {
			if c.opts.OnTraffic != nil {
				c.opts.OnTraffic(n, 0)
			}
		},
		func(n int64) {
			if c.opts.OnTraffic != nil {
				c.opts.OnTraffic(0, n)
			}
		},
	); err != nil {
		slog.Debug("Puente TCP-Stream cerrado", "error", err)
	}
}
