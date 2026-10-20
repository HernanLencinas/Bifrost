package tunnel

import "testing"

func TestNormalizeServerURL(t *testing.T) {
	tests := []struct {
		name      string
		raw       string
		preferTLS bool
		want      string
	}{
		{
			name: "completa sin cambios",
			raw:  "wss://mi-host.com:443/tunnel",
			want: "wss://mi-host.com:443/tunnel",
		},
		{
			name: "agrega /tunnel",
			raw:  "wss://mi-host.com:443",
			want: "wss://mi-host.com:443/tunnel",
		},
		{
			name: "puerto 443 implica wss",
			raw:  "mi-host.com:443",
			want: "wss://mi-host.com:443/tunnel",
		},
		{
			name: "sin esquema usa ws",
			raw:  "192.168.1.10:3000",
			want: "ws://192.168.1.10:3000/tunnel",
		},
		{
			name:      "preferTLS fuerza wss",
			raw:       "proxy.ejemplo.com:8080",
			preferTLS: true,
			want:      "wss://proxy.ejemplo.com:8080/tunnel",
		},
		{
			name: "recorta barra final antes de /tunnel",
			raw:  "ws://localhost:8000/",
			want: "ws://localhost:8000/tunnel",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeServerURL(tt.raw, tt.preferTLS)
			if got != tt.want {
				t.Fatalf("NormalizeServerURL(%q, %v) = %q, want %q", tt.raw, tt.preferTLS, got, tt.want)
			}
		})
	}
}
