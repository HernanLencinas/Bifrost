package tunnel

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/hashicorp/yamux"
)

// yamuxSlog envía los logs internos de Yamux a slog en vez de os.Stderr.
// Escribir en stderr mientras corre la TUI corrompe el buffer de tcell
// (texto fantasma, bordes rotos, paneles duplicados) hasta un Sync().
type yamuxSlog struct{}

func (yamuxSlog) Print(v ...interface{}) {
	logYamux(fmt.Sprint(v...))
}

func (yamuxSlog) Printf(format string, v ...interface{}) {
	logYamux(fmt.Sprintf(format, v...))
}

func (yamuxSlog) Println(v ...interface{}) {
	logYamux(fmt.Sprintln(v...))
}

func logYamux(msg string) {
	msg = strings.TrimSpace(msg)
	if msg == "" {
		return
	}
	switch {
	case strings.Contains(msg, "[ERR]"):
		slog.Error(stripYamuxPrefix(msg))
	case strings.Contains(msg, "[WARN]"):
		slog.Warn(stripYamuxPrefix(msg))
	default:
		slog.Debug(stripYamuxPrefix(msg))
	}
}

func stripYamuxPrefix(msg string) string {
	for _, prefix := range []string{"[ERR] ", "[WARN] ", "[DEBUG] ", "[INFO] "} {
		if strings.HasPrefix(msg, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(msg, prefix))
		}
	}
	return msg
}

func newYamuxConfig() *yamux.Config {
	cfg := yamux.DefaultConfig()
	cfg.LogOutput = nil
	cfg.Logger = yamuxSlog{}
	return cfg
}
