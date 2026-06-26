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

WORKDIR /root/

# Copiar el binario desde la etapa de build
COPY --from=builder /app/bifrost .

# Crear directorio de configuración (el usuario deberá montar su config aquí)
RUN mkdir -p /root/config

# Definir el volumen para la persistencia de configuraciones
VOLUME ["/root/config"]

# Exponer el puerto por defecto del servidor (si se usa en modo server)
EXPOSE 3000

# El entrypoint apunta al binario de Bifrost
# Se puede usar como:
# docker run bifrost server
# docker run bifrost client
ENTRYPOINT ["./bifrost"]
