package tunnel

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"strings"


	"github.com/gorilla/websocket"
	"github.com/hashicorp/yamux"
)

// ServerOptions contiene la configuración básica del servidor de túneles.
type ServerOptions struct {
	Addr        string
	Token       string
}

// Server estructura que maneja la lógica central del lado servidor.
type Server struct {
	opts ServerOptions
	up   websocket.Upgrader
}

// NewServer construye el servidor y prepara su Upgrader.
func NewServer(opts ServerOptions) *Server {
	return &Server{
		opts: opts,
		// Habilitamos la compresión nativa del WebSocket si el cliente la pide
		up: websocket.Upgrader{
			EnableCompression: true,
			CheckOrigin: func(r *http.Request) bool {
				return true // Validar Origin en entornos de producción
			},
		},
	}
}

// Start inicia la escucha HTTP para los WebSockets y las métricas.
func (s *Server) Start(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleTunnel)
	mux.HandleFunc("/tunnel", s.handleTunnel)

	srv := &http.Server{
		Addr:    s.opts.Addr,
		Handler: mux,
	}

	go func() {
		<-ctx.Done()
		srv.Shutdown(context.Background())
	}()

	slog.Info("Servidor Bifrost escuchando", "addr", s.opts.Addr)
	return srv.ListenAndServe()
}



// handleTunnel evalúa el handshake HTTP, sube el protocolo a WebSocket
// e inicia la sesión multiplexada.
func (s *Server) handleTunnel(w http.ResponseWriter, r *http.Request) {
	// 1. Verificación de seguridad mediante Shared Token
	// Intentamos obtener del header estándar Authorization o el personalizado X-Tunnel-Token
	clientToken := strings.TrimSpace(r.Header.Get("Authorization"))
	if clientToken == "" {
		clientToken = strings.TrimSpace(r.Header.Get("X-Tunnel-Token"))
	}

	slog.Info("Intento de handshake", "remote", r.RemoteAddr, "path", r.URL.Path, "has_token", clientToken != "")

	if s.opts.Token != "" && clientToken != strings.TrimSpace(s.opts.Token) {
		slog.Warn("Acceso denegado: Los tokens no coinciden", 
			"remote", r.RemoteAddr, 
			"len_recibido", len(clientToken), 
			"len_esperado", len(strings.TrimSpace(s.opts.Token)))
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// 2. Upgrade del request HTTP a WebSocket
	wsConn, err := s.up.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("Fallo Upgrade a WebSocket", "error", err)
		return
	}
	defer wsConn.Close()

	targetAddr := r.Header.Get("X-Tunnel-Target")
	if targetAddr == "" {
		targetAddr = "1.1.1.1:80" // backward compatibility fallback
	}

	slog.Info("Nuevo cliente conectado", "remote", wsConn.RemoteAddr(), "target", targetAddr)

	// 3. Envuelve la conexión gorila/websocket en nuestra interfaz net.Conn
	conn := NewWSConn(wsConn)

	// 4. Inicia la sesión de Yamux sobre el net.Conn del WebSocket
	session, err := yamux.Server(conn, newYamuxConfig())
	if err != nil {
		slog.Error("No se pudo iniciar Yamux", "error", err)
		return
	}
	defer session.Close()

	// 5. Ciclo de escucha para streams multiplexados (Local Forwarding entrante)
	for {
		stream, err := session.AcceptStream()
		if err != nil {
			slog.Error("Fallo aceptando un stream de Yamux", "error", err)
			break
		}

		go func(st *yamux.Stream) {
			s.handleStream(r.Context(), st, targetAddr)
		}(stream)
	}
}

// handleStream maneja cada túnel lógico que llega desde el cliente (Local Forwarding).
func (s *Server) handleStream(ctx context.Context, stream *yamux.Stream, targetAddr string) {
	defer stream.Close()

	// ruteamos el tráfico interno hacia el target indicado por el cliente
	dst, err := net.Dial("tcp", targetAddr)
	if err != nil {
		slog.Error("No se alcanzó el destino", "error", err, "target", targetAddr)
		return
	}
	defer dst.Close()

	slog.Debug("Nuevo stream enlazado", "streamID", stream.StreamID())

	// 6. El núcleo: copiamos los bytes bidireccionalmente.
	if err := Proxy(ctx, stream, dst); err != nil {
		slog.Warn("Túnel finalizado", "motivo", err)
	}
}
