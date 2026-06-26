#!/bin/bash
# Desarrollo: inyecta la versión de VERSION.md igual que el build de producción.
VERSION=$(tr -d '[:space:]' < VERSION.md)
exec go run -ldflags "-X main.Version=${VERSION}" ./cmd/bifrost "$@"
