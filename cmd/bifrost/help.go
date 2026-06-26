package main

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type helpTopic struct {
	title   string
	content string
}

func bifrostHelpTopics(version string) []helpTopic {
	configDir := configDirPath()
	clientPath := clientConfigPath()
	serverPath := serverConfigPath()

	return []helpTopic{
		{
			title: "Introducción",
			content: fmt.Sprintf(`[orange::b]¿Qué es Bifrost?[-]

[white]Bifrost[-] es una herramienta de [yellow]port forwarding[-] que
redirige tráfico TCP a través de una conexión [yellow]WebSocket[-]
multiplexada. Permite acceder a servicios internos
eludiendo restricciones de red usando tráfico HTTP/S.

[orange::b]Versión[-]
  %s

[orange::b]Modos de uso[-]
  • [yellow]./bifrost[-]        Interfaz TUI (recomendado)
  • [yellow]./bifrost server[-]  Demonio servidor
  • [yellow]./bifrost client[-]  Cliente CLI mínimo

[orange::b]Flujo del túnel[-]
  App local → puerto local (listen)
           → WebSocket
           → servidor Bifrost
           → destino remoto (target)`, version),
		},
		{
			title: "Inicio rápido",
			content: `[orange::b]1. Generar token (opcional)[-]
  [yellow]./bifrost encryptsecret[-]
  Copie el valor [white]ENC:...[-] para los archivos de config.

[orange::b]2. Servidor remoto[-]
  Cree [white]config/server.conf[-] y ejecute:
  [yellow]./bifrost server[-]

[orange::b]3. Cliente (esta TUI)[-]
  Presione [yellow]A[-] para agregar una conexión con:
    • Nombre, dirección:puerto y token del servidor
  Entre a la conexión con [yellow]Enter[-] y presione [yellow]A[-]
  para crear un túnel con listen y target.

[orange::b]4. Conectar[-]
  Seleccione un túnel y presione [yellow]Enter[-], o use
  [yellow]T[-] para iniciar todos los túneles del servidor.

[silver]Los túneles con "Conectar al iniciar" arrancan solos
al abrir la aplicación.[-]`,
		},
		{
			title: "Interfaz TUI",
			content: `[orange::b]Paneles de la interfaz[-]

[white]Panel izquierdo — Conexiones[-]
  Lista de servidores remotos configurados. Al entrar en uno,
  muestra sus túneles. Si no hay elementos, aparece una guía
  de bienvenida.

[white]Panel derecho superior — Túneles Activos[-]
  Tabla con túneles en ejecución: conexión, nombre, rutas
  local ↔ destino y estado. Muestra métricas de tráfico,
  reconexiones y errores por cada túnel activo.

[white]Panel derecho inferior — Consola[-]
  Logs en tiempo real de la aplicación y los túneles.

[orange::b]Navegación[-]
  [yellow]Tab[-] alterna el foco entre paneles.
  El panel activo se resalta con borde amarillo.

[orange::b]Barra inferior[-]
  Muestra atajos globales: siguiente panel, ayuda y salir.
  Cada panel tiene su propia barra de atajos contextuales.`,
		},
		{
			title: "Atajos: Conexiones",
			content: `[orange::b]Lista de conexiones (servidores)[-]

  [yellow]A[-]       Nueva conexión
  [yellow]E[-]       Editar conexión seleccionada
  [yellow]D[-]       Eliminar conexión (con confirmación)
  [yellow]Enter[-]   Entrar a los túneles del servidor

[orange::b]Formulario de conexión[-]
  [yellow]Tab[-]     Moverse entre campos y botones
  [yellow]Esc[-]     Cancelar y cerrar
  [yellow]Enter[-]   Confirmar botón o campo activo

[silver]Al eliminar una conexión se detienen todos sus
túneles activos.[-]`,
		},
		{
			title: "Atajos: Túneles",
			content: `[orange::b]Lista de túneles (dentro de un servidor)[-]

  [yellow]A[-]       Nuevo túnel
  [yellow]E[-]       Editar túnel seleccionado
  [yellow]D[-]       Eliminar túnel (con confirmación)
  [yellow]T[-]       Iniciar TODOS los túneles del servidor
  [yellow]Enter[-]   Iniciar túnel / desconectar si está activo
  [yellow]Esc[-]     Volver a la lista de conexiones
  [yellow]←[-]       Volver a la lista de conexiones

[orange::b]Formulario de túnel[-]
  [yellow]Tab[-]     Moverse entre campos y botones
  [yellow]Esc[-]     Cancelar y cerrar

[silver]Si edita un túnel activo, se detiene antes de guardar
los cambios.[-]`,
		},
		{
			title: "Atajos: Activos",
			content: `[orange::b]Tabla de túneles activos[-]

  [yellow]D[-]       Detener el túnel seleccionado
  [yellow]K[-]       Detener TODAS las conexiones activas
  [yellow]↑ ↓[-]     Seleccionar fila en la tabla
  [yellow]Tab[-]     Pasar al siguiente panel

[orange::b]Estados posibles[-]
  [green]Conectado[-]              Túnel operativo
  [yellow]Conectando...[-]        Estableciendo sesión
  [yellow]Iniciando[-]            Arranque del proceso
  [gray]Desconectado[-]           Sin conexión activa
  [red]Error (Reintentando)[-]    Fallo con reintento automático

[silver]La tabla muestra bytes enviados/recibidos, tasas
por segundo, reconexiones y errores acumulados.[-]`,
		},
		{
			title: "Atajos: Consola",
			content: `[orange::b]Panel de consola (logs)[-]

  [yellow]C[-]       Limpiar la consola de logs
  [yellow]Tab[-]     Pasar al siguiente panel
  [yellow]↑ ↓[-]     Desplazarse por el historial
  [yellow]PgUp/PgDn[-]  Scroll rápido

[orange::b]Atajos globales[-]

  [yellow]Tab[-]         Cambiar entre paneles
  [yellow]Ctrl+A[-]      Abrir / cerrar esta ayuda
  [yellow]Ctrl+Q[-]      Salir (con confirmación)
  [yellow]Esc[-]         Cerrar ventanas emergentes

[silver]Los logs usan colores según el nivel: info, warning
y error.[-]`,
		},
		{
			title: "Config. cliente",
			content: fmt.Sprintf(`[orange::b]Archivo de configuración del cliente[-]

  Ruta: [yellow]%s[-]
  Directorio: [yellow]%s[-]

[orange::b]Estructura general[-]
[white]{
  "servers": [ ... ]
}[-]

Cada elemento de [white]servers[-] es una conexión a un
servidor Bifrost remoto con su lista de túneles.

[orange::b]Creación automática[-]
  Si el archivo no existe o es inválido, la TUI crea uno
  vacío al iniciar y lo guarda en disco.

[orange::b]Permisos[-]
  El directorio [white]config/[-] se crea con permisos 0755.
  El archivo se guarda con permisos 0644.

[silver]La ruta se resuelve junto al ejecutable (no al
directorio de trabajo), salvo en builds temporales de go run.[-]`, clientPath, configDir),
		},
		{
			title: "Config. servidor",
			content: fmt.Sprintf(`[orange::b]Archivo de configuración del servidor[-]

  Ruta: [yellow]%s[-]

[orange::b]Estructura[-]
[white]{
  "server": {
    "address": "0.0.0.0",
    "port": "3000",
    "secret": "ENC:..."
  }
}[-]

[orange::b]Campos[-]
  [yellow]address[-]  IP o hostname de escucha
  [yellow]port[-]     Puerto HTTP/WebSocket
  [yellow]secret[-]   Token compartido con los clientes

[orange::b]Ejecución[-]
  [yellow]./bifrost server[-]

El servidor escucha en [white]address:port[-] y acepta
conexiones WebSocket en [white]/[-] y [white]/tunnel[-].

[silver]No acepta flags por línea de comandos: toda la
configuración proviene del archivo.[-]`, serverPath),
		},
		{
			title: "Campos: Conexión",
			content: `[orange::b]Campos del formulario de conexión[-]

  [yellow]Nombre[-]
    Etiqueta visible en la TUI. Obligatorio.

  [yellow]Dirección:Puerto[-]
    Host y puerto del servidor Bifrost.
    Ej: [white]proxy.ejemplo.com:443[-]
    Si no incluye esquema, se agrega automáticamente
    [white]ws://[-] o [white]wss://[-] según TLS.

  [yellow]Token[-]
    Secreto de autenticación. Puede ser texto plano o
    [white]ENC:...[-] (cifrado). Se cifra al guardar si
    está en texto plano.

  [yellow]Usar TLS (WSS)[-]
    Fuerza conexión segura [white]wss://[-] cuando la URL
    no tiene esquema explícito.

  [yellow]Ignorar errores TLS (Insecure)[-]
    Omite la validación del certificado. Útil con
    certificados autofirmados detrás de un proxy.

[orange::b]URL WebSocket final[-]
  Si la URL no termina en [white]/tunnel[-], se agrega
  automáticamente al conectar.`,
		},
		{
			title: "Campos: Túnel",
			content: `[orange::b]Campos del formulario de túnel[-]

  [yellow]Nombre Túnel[-]
    Etiqueta visible en la TUI. Obligatorio.

  [yellow]Dirección:Puerto Local[-]
    Donde el cliente escucha en su máquina.
    Ej: [white]127.0.0.1:5432[-] o [white]:8080[-]

  [yellow]Dirección:Puerto Remoto[-]
    Destino TCP alcanzable [white]desde el servidor[-].
    Ej: [white]db-interna:5432[-] o [white]127.0.0.1:80[-]

  [yellow]Conectar al iniciar[-]
    Si está activo ([white]autoconnect: true[-] en el JSON),
    el túnel se inicia automáticamente al abrir la TUI.

[orange::b]Ejemplo práctico[-]
  Local  [white]127.0.0.1:27017[-]
  Remoto [white]mongodb:27017[-]
  → Su app local se conecta a localhost:27017 y el tráfico
    sale hacia MongoDB en la red del servidor.`,
		},
		{
			title: "Cifrado de tokens",
			content: `[orange::b]Protección de secretos en disco[-]

Bifrost cifra tokens con [yellow]AES-GCM 256[-]. En los
archivos .conf aparecen con el prefijo [white]ENC:[-].

[orange::b]Generar un token cifrado[-]
  [yellow]./bifrost encryptsecret[-]
  Escriba el secreto (enmascarado con *) y copie el resultado.

[orange::b]Comportamiento automático[-]
  • La TUI cifra tokens en texto plano al guardar.
  • Servidor y cliente descifran [white]ENC:...[-] al leer.
  • Puede pegar un token ya cifrado en los formularios.

[silver]El cifrado ofusca secretos en disco; no sustituye
un gestor de claves externo (KMS).[-]`,
		},
		{
			title: "Modo servidor",
			content: `[orange::b]bifrost server[-]

Inicia el demonio que acepta conexiones de clientes.

[orange::b]Autenticación[-]
  El cliente envía el token en el header [white]Authorization[-]
  o [white]X-Tunnel-Token[-]. Debe coincidir con [white]secret[-]
  en [white]server.conf[-].

[orange::b]Destino del túnel[-]
  Cada stream indica su destino con el header
  [white]X-Tunnel-Target[-] (host:puerto). La TUI lo envía
  según el campo [white]target[-] de cada túnel.

[orange::b]Multiplexación[-]
  Multiplexa varias conexiones TCP en una sola sesión
  WebSocket.

[orange::b]Apagado[-]
  [yellow]Ctrl+C[-] o [yellow]SIGTERM[-] para cierre controlado.`,
		},
		{
			title: "Modo cliente CLI",
			content: `[orange::b]bifrost client[-]

Cliente mínimo por línea de comandos (sin TUI).

[orange::b]Flags[-]
  [yellow]-u, --url[-]     URL WebSocket del servidor
                  (default: ws://localhost:8000/tunnel)
  [yellow]-t, --token[-]   Token de autenticación
  [yellow]-l, --listen[-]  Puerto local a abrir
                  (default: 127.0.0.1:9090)

[orange::b]Ejemplo[-]
  [white]./bifrost client \
    -u wss://proxy.com:443/tunnel \
    -t mi_token \
    -l 127.0.0.1:8080[-]

[orange::b]Reconexión[-]
  Reintento automático con backoff exponencial (1s → 30s).

[silver]Para múltiples túneles, destinos configurables y
métricas, use la TUI interactiva.[-]`,
		},
		{
			title: "TLS y seguridad",
			content: `[orange::b]Conexiones seguras[-]

  [yellow]WSS[-]     WebSocket sobre TLS ([white]wss://[-])
  [yellow]TLS[-]     Activar con "Usar TLS" o puerto :443
  [yellow]Insecure[-] Ignorar validación de certificados

[orange::b]Compatibilidad con proxies[-]
  Funciona detrás de Traefik, Nginx y otros proxies
  inversos que soporten WebSocket.

[orange::b]Recomendaciones[-]
  • Use tokens largos y aleatorios.
  • Prefiera [white]ENC:...[-] en archivos de configuración.
  • Use TLS en producción; reserve Insecure para entornos
    de desarrollo o certificados autofirmados.
  • Restrinja el acceso al directorio [white]config/[-].`,
		},
		{
			title: "Docker",
			content: `[orange::b]Ejecución en contenedor[-]

[orange::b]Construir imagen[-]
  [white]docker build -t bifrost:latest .[-]

[orange::b]Ejecutar como servidor[-]
  [white]docker run -p 3000:3000 \
    -v $(pwd)/config:/root/config \
    bifrost:latest server[-]

Monte el volumen en [white]/root/config[-] para persistir
[white]server.conf[-] y [white]client.conf[-].

[orange::b]Puerto[-]
  El Dockerfile expone el puerto [white]3000[-] por defecto.
  Ajústelo según su [white]server.conf[-].`,
		},
		{
			title: "Compilación",
			content: `[orange::b]Build multi-plataforma[-]

  [white]chmod +x build.sh[-]
  [white]./build.sh[-]

Genera en [white]bin/[-]:
  • Ejecutables Linux, Windows y macOS (amd64/arm64)
  • Copia de [white]config/client.conf[-] en cada carpeta
  • ZIPs listos para distribuir

[orange::b]Desarrollo local[-]
  [white]go run ./cmd/bifrost[-]     Abre la TUI
  [white]go run ./cmd/bifrost server[-]

[orange::b]Versión embebida[-]
  Se define en compilación con [white]-ldflags[-].
  Sin ello, la versión aparece como [white]dev[-].`,
		},
	}
}

func newHelpPage(version string) (tview.Primitive, *tview.List, *tview.TextView) {
	topics := bifrostHelpTopics(version)

	contentView := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true).
		SetWrap(true)
	contentView.SetBackgroundColor(uiFormBg)
	contentView.SetBorderPadding(1, 1, 2, 1)
	contentView.SetText(topics[0].content)

	topicList := tview.NewList().
		ShowSecondaryText(false).
		SetHighlightFullLine(true).
		SetSelectedStyle(tcell.StyleDefault.
			Background(uiModalBorder).
			Foreground(tcell.ColorBlack).
			Attributes(tcell.AttrBold))
	topicList.SetBackgroundColor(uiFormFieldBg)
	topicList.SetBorderPadding(0, 0, 1, 1)
	topicList.SetMainTextColor(tcell.ColorWhite)
	topicList.SetSecondaryTextColor(tcell.ColorGray)

	showTopic := func(index int) {
		if index >= 0 && index < len(topics) {
			contentView.SetText(topics[index].content)
			contentView.ScrollToBeginning()
		}
	}

	for i, topic := range topics {
		idx := i
		topicList.AddItem(topic.title, "", 0, func() {
			showTopic(idx)
		})
	}
	topicList.SetChangedFunc(func(index int, mainText, secondary string, shortcut rune) {
		showTopic(index)
	})

	shortcutsBar := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter).
		SetText("[white][Esc][-] Cerrar   [white][Ctrl+A][-] Cerrar   [white][↑↓][-] Temas   [white][Tab][-] Panel")
	shortcutsBar.SetBackgroundColor(tcell.GetColor("#252525"))

	body := tview.NewFlex().
		AddItem(topicList, 30, 0, true).
		AddItem(contentView, 0, 2, true)

	card := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(body, 0, 1, true).
		AddItem(shortcutsBar, 1, 0, false)
	card.SetBorder(true).
		SetTitle(" Ayuda de Bifrost ").
		SetTitleColor(uiModalAccent).
		SetBorderColor(uiModalBorder)

	return centeredOverlayPage(card, 100, 28), topicList, contentView
}
