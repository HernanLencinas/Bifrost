package tunnel

import "strings"

// NormalizeServerURL completa el esquema WebSocket y la ruta /tunnel
// cuando faltan, igual que la TUI al armar la URL del cliente.
//
// preferTLS fuerza wss:// cuando no hay esquema; si es false, se usa wss://
// solo cuando el host termina en :443.
func NormalizeServerURL(raw string, preferTLS bool) string {
	fullURL := strings.TrimSpace(raw)
	if fullURL == "" {
		return fullURL
	}

	if !strings.HasPrefix(fullURL, "ws://") && !strings.HasPrefix(fullURL, "wss://") {
		if preferTLS || strings.HasSuffix(fullURL, ":443") {
			fullURL = "wss://" + fullURL
		} else {
			fullURL = "ws://" + fullURL
		}
	}
	if !strings.HasSuffix(fullURL, "/tunnel") {
		fullURL = strings.TrimSuffix(fullURL, "/") + "/tunnel"
	}
	return fullURL
}
