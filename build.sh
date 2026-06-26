#!/bin/bash

# Bifrost Multi-Platform Build Script
# ----------------------------------

# Obtener versión desde VERSION.md (solo en tiempo de compilación)
VERSION=$(tr -d '[:space:]' < VERSION.md)
LDFLAGS="-X main.Version=${VERSION}"
BINARY_NAME="bifrost"
OUTPUT_DIR="bin"

echo "🚀 Iniciando proceso de compilación para la versión: $VERSION"

# Limpiar carpeta bin
rm -rf $OUTPUT_DIR
mkdir -p $OUTPUT_DIR

# Definir las plataformas: "GOOS/GOARCH"
PLATFORMS=(
    "linux/amd64"
    "windows/amd64"
    "darwin/amd64"
    "darwin/arm64"
)

for PLATFORM in "${PLATFORMS[@]}"; do
    # Separar OS y Arch
    IFS="/" read -r OS ARCH <<< "$PLATFORM"
    
    # Definir nombre del ejecutable (con .exe para Windows)
    EXEC_NAME=$BINARY_NAME
    if [ "$OS" == "windows" ]; then
        EXEC_NAME="${BINARY_NAME}.exe"
    fi

    # Definir ruta de salida: bin/os_arch/binary
    PLATFORM_DIR="${OUTPUT_DIR}/${OS}_${ARCH}"
    mkdir -p "$PLATFORM_DIR"
    
    OUTPUT_PATH="${PLATFORM_DIR}/${EXEC_NAME}"

    echo "📦 Compilando para $OS ($ARCH)..."
    
    # Ejecutar compilación de Go (versión embebida en el binario)
    GOOS=$OS GOARCH=$ARCH go build -ldflags "${LDFLAGS}" -o "$OUTPUT_PATH" ./cmd/bifrost

    if [ $? -eq 0 ]; then
        echo "✅ Generado: $OUTPUT_PATH"
        
        # Copiar archivo de configuración del cliente
        mkdir -p "${PLATFORM_DIR}/config"
        if [ -f "config/client.conf" ]; then
            cp "config/client.conf" "${PLATFORM_DIR}/config/client.conf"
            echo "📄 Configuración copiada a ${PLATFORM_DIR}/config/"
        fi

        # Comprimir la distribución en formato ZIP
        ZIP_NAME="bifrost_${VERSION}_${OS}_${ARCH}.zip"
        echo "📦 Comprimiendo en $ZIP_NAME..."
        # Entramos a la carpeta bin para que el zip no tenga rutas absolutas innecesarias
        (cd "$OUTPUT_DIR" && zip -q -r "$ZIP_NAME" "${OS}_${ARCH}")
        echo "🎁 Zip creado: ${OUTPUT_DIR}/${ZIP_NAME}"
    else
        echo "❌ Error compilando para $OS ($ARCH)"
    fi
done

echo "---------------------------------------"
echo "✨ Proceso de construcción finalizado. Revisa la carpeta /$OUTPUT_DIR"
