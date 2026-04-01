package tunnel

import (
	"io"
	"net"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	// Tiempo permitido para escribir un mensaje al peer.
	writeWait = 10 * time.Second

	// Tiempo permitido para leer el próximo pong del peer.
	pongWait = 60 * time.Second

	// Intervalo para enviar pings al peer. Debe ser menor que pongWait.
	pingPeriod = (pongWait * 9) / 10
)

// wsConn adapta el *websocket.Conn para implementar la interfaz net.Conn.
type wsConn struct {
	*websocket.Conn
	r      io.Reader
	mu     sync.Mutex
	done   chan struct{}
}

// NewWSConn recibe una conexión websocket y la envuelve como net.Conn
// permitiendo la integración nativa con hashicorp/yamux.
func NewWSConn(c *websocket.Conn) net.Conn {
	c.SetReadDeadline(time.Now().Add(pongWait))
	c.SetPongHandler(func(string) error {
		c.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	ws := &wsConn{
		Conn: c,
		done: make(chan struct{}),
	}
	
	// Iniciamos el ticker de pings para mantener la conexión viva
	go ws.pingLoop()

	return ws
}

func (c *wsConn) pingLoop() {
	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			c.mu.Lock()
			// Solo enviamos ping si la conexión no está cerrada
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				c.mu.Unlock()
				return
			}
			c.mu.Unlock()
		case <-c.done:
			return
		}
	}
}

func (c *wsConn) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	select {
	case <-c.done:
		// Ya cerrado
	default:
		close(c.done)
	}
	return c.Conn.Close()
}

// Read implementa lectura fluida de mensajes binarios ignorando bordes de frame.
func (c *wsConn) Read(b []byte) (int, error) {
	for {
		if c.r == nil {
			// Bloquea hasta que haya un nuevo mensaje
			msgType, r, err := c.Conn.NextReader()
			if err != nil {
				return 0, err
			}
			
			// Actualizamos el deadline de lectura al recibir cualquier mensaje (incluyendo control frames que NextReader procesa)
			c.Conn.SetReadDeadline(time.Now().Add(pongWait))

			// Ignoramos mensajes que no sean binarios
			if msgType != websocket.BinaryMessage {
				continue
			}
			c.r = r
		}
		
		n, err := c.r.Read(b)
		if err == io.EOF {
			c.r = nil
			if n > 0 {
				return n, nil
			}
			continue
		}
		return n, err
	}
}

// Write envía el buffer como un mensaje binario.
// Protegido por mutex para permitir uso concurrente por Yamux.
func (c *wsConn) Write(b []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
	err := c.Conn.WriteMessage(websocket.BinaryMessage, b)
	if err != nil {
		return 0, err
	}
	return len(b), nil
}

// SetDeadline aplica los deadlines a las lecturas y escrituras del websocket subyacente.
func (c *wsConn) SetDeadline(t time.Time) error {
	if err := c.Conn.SetReadDeadline(t); err != nil {
		return err
	}
	return c.Conn.SetWriteDeadline(t)
}
