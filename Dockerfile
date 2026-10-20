# Build stage
FROM golang:alpine AS builder

# Instalar dependencias necesarias para el build
RUN apk add --no-cache git ca-certificates

# Permitir que Go descargue la versión necesaria si go.mod lo requiere
ENV GOTOOLCHAIN=auto

WORKDIR /app

# Copiar archivos de dependencia primero para aprovechar el cache de Docker
COPY go.mod go.sum ./
RUN go mod download

# Copiar el resto del código fuente
COPY . .

# Compilar el binario (versión embebida desde VERSION.md)
RUN VERSION=$(tr -d '[:space:]' < VERSION.md) && \
    CGO_ENABLED=0 GOOS=linux go build -ldflags "-X main.Version=${VERSION}" -o bifrost ./cmd/bifrost

# Run stage
FROM alpine:latest

# Instalar certificados CA para poder hacer peticiones HTTPS (túneles wss://)
RUN apk --no-cache add ca-certificates tzdata

# Usar /app en lugar de /root/ para que sea accesible por cualquier UID.
# OpenShift ejecuta los contenedores con un UID arbitrario (no root) que
# pertenece al grupo root (GID 0), por lo que /root/ no es accesible.
WORKDIR /app

# Copiar el binario desde la etapa de build
COPY --from=builder /app/bifrost .

# Crear directorio de configuración (el usuario deberá montar su config aquí)
RUN mkdir -p /app/config

# Ajustar permisos para compatibilidad con OpenShift:
#  - El binario debe ser ejecutable por el grupo root (GID 0).
#  - Los directorios deben ser accesibles/escribibles por el grupo root.
RUN chmod g+rwX /app /app/config && \
    chmod 0755 /app/bifrost && \
    chgrp -R 0 /app && \
    chmod -R g=u /app

# Ejecutar como usuario no-root por defecto (buena práctica; OpenShift
# igualmente sobreescribe el UID, pero esto mantiene la imagen portable).
USER 1001

# Definir el volumen para la persistencia de configuraciones
VOLUME ["/app/config"]

# Exponer el puerto por defecto del servidor (si se usa en modo server)
EXPOSE 3000

# El entrypoint apunta al binario de Bifrost
# Se puede usar como:
# docker run bifrost server
# docker run bifrost client
ENTRYPOINT ["./bifrost"]
