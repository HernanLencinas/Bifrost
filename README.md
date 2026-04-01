<img width="1359" height="269" alt="1" src="https://github.com/user-attachments/assets/112a4d11-7749-402c-9c8f-6f7287b5b971" />

# Bifrost - Multi-Platform Tunneling

Bifrost es una herramienta robusta y de alto rendimiento escrita en Go diseñada para crear túneles TCP sobre una única conexión segura de WebSockets (multiplexada). Es ideal para realizar *Port Forwarding* y eludir restricciones de red mediante tráfico HTTP/S, facilitando el acceso a servicios internos de forma segura.

## ✨ Características Principales

- **Stream Multiplexing:** Utiliza `hashicorp/yamux` sobre WebSockets para manejar cientos de conexiones concurrentes en una sola sesión real.
- **TUI Interactiva Premium:** Interfaz de usuario por consola con soporte para múltiples servidores, túneles, logs en vivo y atajos rápidos.
- **Resiliencia Extrema:** Reconexión automática con *Exponential Backoff*. Si la conexión cae, el cliente intentará restablecer el túnel transparentemente.
- **Seguridad Integrada:** Autenticación por **Shared Token** con soporte para cifrado **AES-GCM 256** (tokens ofuscados en archivos de configuración).
- **Soporte TLS/Proxy:** Compatible con Traefik, Nginx y otros proxies. Opción para ignorar validación de certificados (Ideal para entornos con self-signed certs).
- **Multi-plataforma:** Binarios nativos para Linux, Windows (amd64) y macOS (Intel/Apple Silicon).

---

## 🚀 Compilación y Distribución

Bifrost incluye un script de automatización que compila, organiza y empaqueta la aplicación para todas las plataformas soportadas.

```bash
# Otorgar permisos de ejecución
chmod +x build.sh

# Ejecutar el build completo
./build.sh
```

Esto generará una carpeta `bin/` con:
- Ejecutables para cada OS/Arch.
- Una copia de la configuración base en cada carpeta.
- Archivos **.zip** listos para ser distribuidos.

---

## 🛠️ Configuración y Uso

### 1. Generar un Token de Autenticación
Para mayor seguridad, nunca guardes tus tokens en texto plano. Encripta tu clave secreta:

```bash
./bifrost encryptsecret
```
Copia el resultado (ej: `ENC:hash...`) para usarlo en tus archivos `.conf`.

### 2. Levantar el Servidor (Server)
Crea `config/server.conf` y define el puerto y secreto:

```json
{
    "server": {
        "port": "3000",
        "address": "0.0.0.0",
        "secret": "ENC:su_token_encriptado_aqui"
    }
}
```
Ejecuta con: `./bifrost server`. El servidor acepta conexiones en `/` y `/tunnel`.

### 3. Interfaz del Cliente (TUI)
Ejecuta `./bifrost` sin argumentos para abrir la consola interactiva. 

**Atajos Rápidos:**
- `Tab`: Cambiar entre lista de servidores, tabla de conexiones y logs.
- `A`: Agregar Servidor / Agregar Túnel.
- `E`: Editar elemento seleccionado.
- `D`: Eliminar elemento (con confirmación).
- `Enter`: (En lista) Conectar/Desconectar túnel.
- `T`: Conectar TODOS los túneles del servidor seleccionado.
- `k`: (En tabla activas) Detener TODAS las conexiones del cliente.
- `c`: (En logs) Limpiar consola de logs.
- `Ctrl+Q`: Salir de la aplicación.

**Estructura de `config/client.conf`:**
```json
{
  "servers": [
    {
      "name": "Producción",
      "url": "mi-proxy.com:443",
      "token": "ENC:...",
      "use_tls": true,
      "insecure": true,
      "connections": [
        {
          "name": "DB Interna",
          "listen": "127.0.0.1:5432",
          "target": "db-prod-container:5432"
        }
      ]
    }
  ]
}
```

---

## 🐳 Docker Support

Si prefieres usar contenedores, Bifrost incluye un `Dockerfile` multi-stage optimizado:

```bash
# Construir la imagen
docker build -t bifrost:latest .

# Ejecutar como servidor
docker run -p 3000:3000 -v $(pwd)/config:/app/config bifrost:latest server
```
---
*Desarrollado con ❤️ en Go por Hernan Lencinas*
