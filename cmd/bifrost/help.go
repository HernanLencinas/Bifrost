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

[white]Bifrost[-] es una herramienta de [yellow]port forwarding[-]
escrita en Go. Redirige tráfico TCP a través de una única
conexión [yellow]WebSocket[-] multiplexada, permitiendo
acceder a servicios internos usando tráfico HTTP/S.

Es ideal para entornos donde solo están abiertos los puertos
80/443, o cuando necesita exponer o consumir servicios
detrás de un firewall o proxy inverso.

[orange::b]Versión actual[-]
  %s

[orange::b]Casos de uso típicos[-]
  • Acceder a una base de datos interna desde su PC
  • Conectar a APIs o servicios en una red privada
  • Redirigir puertos locales hacia contenedores remotos
  • Mantener túneles persistentes con reconexión automática

[orange::b]Modos de ejecución[-]
  [yellow]./bifrost[-]
    Abre la interfaz TUI interactiva (modo recomendado).
    Permite gestionar múltiples servidores y túneles.

  [yellow]./bifrost server[-]
    Inicia el demonio servidor en la máquina remota.
    Lee la configuración desde [white]config/server.conf[-].

  [yellow]./bifrost client[-]
    Cliente mínimo por línea de comandos, sin interfaz
    gráfica en terminal.

  [yellow]./bifrost encryptsecret[-]
    Genera un token cifrado para usar en archivos .conf.

[orange::b]Flujo del túnel[-]
  1. Una aplicación local se conecta al puerto [white]listen[-]
  2. El cliente Bifrost acepta la conexión TCP local
  3. El tráfico viaja por WebSocket hasta el servidor
  4. El servidor abre una conexión TCP al [white]target[-]
  5. Los datos fluyen en ambos sentidos de forma transparente

[orange::b]Características principales[-]
  • Múltiples conexiones TCP sobre una sola sesión WebSocket
  • Reconexión automática con backoff exponencial (1s–30s)
  • Autenticación por token compartido
  • Soporte TLS/WSS e ignorar certificados (Insecure)
  • Tokens cifrables en disco con AES-GCM 256
  • Métricas en vivo: tráfico, reconexiones y errores`, version),
		},
		{
			title: "Inicio rápido",
			content: `[orange::b]Guía paso a paso[-]

[orange::b]Paso 1 — Token de autenticación[-]
  Ejecute [yellow]./bifrost encryptsecret[-] y escriba su
  secreto (se muestra enmascarado con *).
  Copie el resultado con prefijo [white]ENC:...[-] para usarlo
  en [white]server.conf[-] y [white]client.conf[-].

[orange::b]Paso 2 — Servidor remoto[-]
  En la máquina donde está el servicio destino, cree
  [white]config/server.conf[-]:

[white]  {
    "server": {
      "address": "0.0.0.0",
      "port": "3000",
      "secret": "ENC:..."
    }
  }[-]

  Inicie el servidor: [yellow]./bifrost server[-]
  El demonio escucha en [white]address:port[-] y acepta
  WebSocket en [white]/[-] y [white]/tunnel[-].

[orange::b]Paso 3 — Primera conexión (TUI)[-]
  Ejecute [yellow]./bifrost[-] sin argumentos.
  Presione [yellow]A[-] para crear una conexión:
    • [white]Nombre[-]: etiqueta descriptiva (ej. Producción)
    • [white]Dirección:Puerto[-]: host del servidor Bifrost
    • [white]Token[-]: el mismo secreto del servidor
    • Marque TLS si usa puerto 443 o WSS

[orange::b]Paso 4 — Crear un túnel[-]
  Seleccione la conexión y presione [yellow]Enter[-].
  Presione [yellow]A[-] para nuevo túnel:
    • [white]Listen[-]: puerto local (ej. 127.0.0.1:5432)
    • [white]Target[-]: destino visto desde el servidor
      (ej. db-interna:5432 o 127.0.0.1:80)

[orange::b]Paso 5 — Conectar[-]
  Seleccione el túnel y presione [yellow]Enter[-].
  Verifique el estado en "Túneles Activos" y los logs
  en la consola. Use [yellow]T[-] para iniciar todos
  los túneles de un servidor a la vez.

[orange::b]Autoconnect[-]
  Active "Conectar al iniciar" para que el túnel arranque
  automáticamente cada vez que abra la TUI.

[silver]Tip: el destino (target) siempre se resuelve desde
la red del servidor, no desde su máquina local.[-]`,
		},
		{
			title: "Interfaz TUI",
			content: `[orange::b]Descripción general[-]

La interfaz se divide en tres zonas principales más
barras de atajos contextuales y un pie de página global.

[orange::b]Panel izquierdo — Conexiones[-]
  Muestra la lista de servidores remotos configurados.
  Cada entrada representa una conexión guardada en
  [white]client.conf[-].

  Al presionar [yellow]Enter[-] sobre un servidor, entra
  a la vista de túneles de ese servidor. Si no hay
  servidores o túneles configurados, aparece una pantalla
  de ayuda con instrucciones para comenzar.

  Incluye una barra inferior con atajos: Nueva, Editar,
  Eliminar y Enter (Túneles).

[orange::b]Panel derecho superior — Túneles Activos[-]
  Tabla con todos los túneles en ejecución. Columnas:
    • [white]Conexión[-] — servidor al que pertenece
    • [white]Túnel[-] — nombre del túnel
    • [white]Local/Destino[-] — rutas listen ↔ target
    • [white]Estado[-] — situación actual de la sesión

  Por cada túnel activo se muestran filas de detalle:
    • Bytes enviados y recibidos (total y tasa B/s)
    • Cantidad de reconexiones acumuladas
    • Cantidad de errores registrados

  Atajos: [yellow]D[-] detener seleccionado, [yellow]K[-]
  detener todos.

[orange::b]Panel derecho inferior — Consola[-]
  Logs en tiempo real de la aplicación y de cada túnel.
  Los mensajes se colorean según nivel (info, warning,
  error). Atajo [yellow]C[-] limpia el historial.

[orange::b]Navegación entre paneles[-]
  [yellow]Tab[-] cicla el foco en este orden:
    Lista/ayuda → Túneles activos → Consola → Lista

  El panel con foco se resalta con borde amarillo.
  El pie de página muestra atajos globales según el
  panel activo.

[orange::b]Estados del túnel[-]
  [green]Conectado[-]
    Sesión WebSocket activa y operativa.
  [yellow]Conectando...[-]
    Intentando establecer la conexión al servidor.
  [yellow]Iniciando[-]
    El proceso del túnel acaba de arrancar.
  [gray]Desconectado[-]
    Sin sesión activa (detenido manualmente o cerrado).
  [red]Error (Reintentando)[-]
    Falló la conexión; reintento automático en curso
    con backoff exponencial (1s hasta 30s máximo).

[orange::b]Ventanas emergentes[-]
  Formularios para crear/editar conexiones y túneles.
  Diálogos de confirmación para eliminar, desconectar
  o salir. La ayuda ([yellow]Ctrl+A[-]) y el cierre
  ([yellow]Ctrl+Q[-]) también usan ventanas modales.

[orange::b]Atajos globales[-]
  [yellow]Tab[-]         Cambiar entre paneles
  [yellow]Ctrl+A[-]      Abrir / cerrar documentación
  [yellow]Ctrl+Q[-]      Salir (con confirmación)
  [yellow]Esc[-]         Cerrar ventana emergente activa`,
		},
		{
			title: "Config. cliente",
			content: fmt.Sprintf(`[orange::b]Archivo de configuración del cliente[-]

  Ruta completa: [yellow]%s[-]
  Directorio:    [yellow]%s[-]

[orange::b]Resolución de rutas[-]
  Por defecto, la configuración se busca junto al
  ejecutable (symlinks resueltos). En builds temporales
  de [white]go run[-] se usa el directorio de trabajo
  actual. La TUI crea [white]config/[-] automáticamente
  si no existe.

[orange::b]Estructura completa del JSON[-]

[white]{
  "servers": [
    {
      "name": "Producción",
      "url": "proxy.ejemplo.com:443",
      "token": "ENC:...",
      "use_tls": true,
      "insecure": false,
      "connections": [
        {
          "name": "Base de datos",
          "listen": "127.0.0.1:5432",
          "target": "db-interna:5432",
          "autoconnect": true
        }
      ]
    }
  ]
}[-]

[orange::b]Campos del servidor (servers[])[-]
  [yellow]name[-]         Nombre visible en la TUI
  [yellow]url[-]          Host:puerto o URL WebSocket
  [yellow]token[-]        Token de autenticación (ENC:...)
  [yellow]use_tls[-]      Forzar WSS si no hay esquema
  [yellow]insecure[-]     Ignorar validación TLS
  [yellow]connections[-] Lista de túneles del servidor

[orange::b]Persistencia[-]
  Los cambios hechos en la TUI se guardan al crear,
  editar o eliminar conexiones y túneles. Los tokens
  en texto plano se cifran automáticamente al guardar.

[orange::b]Permisos de archivos[-]
  Directorio [white]config/[-]: 0755
  Archivo [white]client.conf[-]: 0644

[orange::b]Atajos en la TUI (lista de conexiones)[-]
  [yellow]A[-]       Nueva conexión
  [yellow]E[-]       Editar conexión seleccionada
  [yellow]D[-]       Eliminar conexión (con confirmación)
  [yellow]Enter[-]   Entrar a los túneles del servidor

[silver]Al eliminar una conexión se detienen todos sus
túneles activos y se borra del archivo de configuración.[-]`, clientPath, configDir),
		},
		{
			title: "Config. servidor",
			content: fmt.Sprintf(`[orange::b]Archivo de configuración del servidor[-]

  Ruta completa: [yellow]%s[-]

[orange::b]Estructura del JSON[-]

[white]{
  "server": {
    "address": "0.0.0.0",
    "port": "3000",
    "secret": "ENC:su_token_aqui"
  }
}[-]

[orange::b]Descripción de campos[-]

  [yellow]address[-]  (obligatorio)
    IP o hostname donde escucha el demonio.
    Use [white]0.0.0.0[-] para todas las interfaces, o
    [white]127.0.0.1[-] solo para conexiones locales.

  [yellow]port[-]  (obligatorio)
    Puerto TCP para HTTP y WebSocket.
    Debe coincidir con el expuesto en firewall/proxy.

  [yellow]secret[-]  (obligatorio)
    Token compartido con todos los clientes. Debe ser
    idéntico al token configurado en cada conexión del
    cliente. Recomendado usar [white]ENC:...[-].

[orange::b]Ejecución[-]
  [yellow]./bifrost server[-]

  No acepta flags por línea de comandos: toda la
  configuración proviene exclusivamente del archivo.

[orange::b]Endpoints HTTP[-]
  El servidor acepta conexiones WebSocket en:
    • [white]/[-]
    • [white]/tunnel[-]

  Ambas rutas usan el mismo handler. La TUI agrega
  [white]/tunnel[-] automáticamente si no está en la URL.

[orange::b]Detrás de un proxy (Traefik, Nginx)[-]
  Configure el proxy para reenviar WebSocket al puerto
  del servidor Bifrost. El cliente puede usar TLS
  terminado en el proxy con [white]use_tls: true[-].

[orange::b]Apagado[-]
  [yellow]Ctrl+C[-] o [yellow]SIGTERM[-] para cierre
  controlado del demonio.`, serverPath),
		},
		{
			title: "Campos: Conexión",
			content: `[orange::b]Formulario Nueva / Editar Conexión[-]

Define cómo el cliente se conecta a un servidor Bifrost
remoto. Cada conexión guardada aparece en el panel
izquierdo de la TUI.

[orange::b]Nombre[-]  (obligatorio)
  Etiqueta descriptiva visible en la lista de conexiones.
  Ejemplos: [white]Producción[-], [white]Staging[-],
  [white]VPN Oficina[-].

[orange::b]Dirección:Puerto[-]  (obligatorio)
  Host y puerto del servidor Bifrost, sin ruta WebSocket.
  Ejemplos:
    [white]proxy.ejemplo.com:443[-]
    [white]192.168.1.50:3000[-]
    [white]tunel.midominio.com:8080[-]

  La TUI construye la URL WebSocket final así:
    • Si no tiene esquema: agrega [white]ws://[-] o
      [white]wss://[-] según TLS o puerto :443
    • Si no termina en [white]/tunnel[-], lo agrega

[orange::b]Token[-]  (obligatorio)
  Secreto compartido con el servidor. Debe coincidir
  exactamente con [white]secret[-] en [white]server.conf[-].
  Puede ingresarse en texto plano o como [white]ENC:...[-].
  Al guardar, la TUI cifra automáticamente tokens en
  texto plano.

[orange::b]Usar TLS (WSS)[-]
  Fuerza conexión segura [white]wss://[-] cuando la URL
  no incluye esquema explícito. Actívelo si el servidor
  está detrás de HTTPS o en puerto 443.

[orange::b]Ignorar errores TLS (Insecure)[-]
  Omite la validación del certificado SSL/TLS.
  Útil con certificados autofirmados o proxies internos.
  [red]No recomendado en producción[-] salvo que sea
  estrictamente necesario.

[orange::b]Reglas de construcción de URL[-]
  Entrada: [white]mi-host.com:443[-] + TLS activo
  Resultado: [white]wss://mi-host.com:443/tunnel[-]

  Entrada: [white]ws://192.168.1.10:3000/tunnel[-]
  Resultado: se usa tal cual (sin modificaciones)

[orange::b]Atajos del formulario[-]
  [yellow]Tab[-]     Siguiente campo o botón
  [yellow]Shift+Tab[-]  Campo anterior
  [yellow]Esc[-]     Cancelar sin guardar
  [yellow]Enter[-]   Activar botón o confirmar campo`,
		},
		{
			title: "Campos: Túnel",
			content: `[orange::b]Formulario Nuevo / Editar Túnel[-]

Define un mapeo de puertos: escucha localmente y
reenvía el tráfico al destino remoto a través del
servidor Bifrost seleccionado.

[orange::b]Nombre Túnel[-]  (obligatorio)
  Identificador visible en la lista y en la tabla de
  túneles activos. Ej: [white]PostgreSQL[-],
  [white]API Interna[-], [white]SSH Jump[-].

[orange::b]Dirección:Puerto Local (listen)[-]  (obligatorio)
  Dirección TCP donde el cliente escucha en SU máquina.
  Ejemplos:
    [white]127.0.0.1:5432[-]  — solo localhost
    [white]:8080[-]           — todas las interfaces
    [white]0.0.0.0:9090[-]   — explícito en todas

  Su aplicación se conectará a esta dirección como si
  el servicio estuviera local.

[orange::b]Dirección:Puerto Remoto (target)[-]  (obligatorio)
  Destino TCP resuelto desde la RED DEL SERVIDOR, no
  desde su PC. Ejemplos:
    [white]db-prod:5432[-]       — hostname Docker/red
    [white]127.0.0.1:80[-]       — localhost del servidor
    [white]10.0.0.5:27017[-]     — IP interna de la red

[orange::b]Conectar al iniciar (autoconnect)[-]
  Si está activo, el túnel se inicia automáticamente
  al abrir la TUI, sin necesidad de presionar Enter.
  En el JSON se guarda como [white]"autoconnect": true[-].
  Los túneles con autoconnect muestran la etiqueta
  [silver][Auto][-] en la lista.

[orange::b]Ejemplos de uso[-]

  [white]Base de datos[-]
    Listen: [white]127.0.0.1:5432[-]
    Target: [white]postgres:5432[-]
    → psql o su ORM conecta a localhost:5432

  [white]Servicio web interno[-]
    Listen: [white]127.0.0.1:8080[-]
    Target: [white]127.0.0.1:80[-]
    → Navegador en http://localhost:8080

  [white]MongoDB en contenedor[-]
    Listen: [white]127.0.0.1:27017[-]
    Target: [white]mongodb:27017[-]

[orange::b]Edición con túnel activo[-]
  Si modifica un túnel que está en ejecución, Bifrost
  lo detiene primero, guarda los cambios y usted debe
  volver a iniciarlo manualmente.

[orange::b]Atajos — lista de túneles[-]
  [yellow]A[-]       Nuevo túnel
  [yellow]E[-]       Editar túnel seleccionado
  [yellow]D[-]       Eliminar túnel (con confirmación)
  [yellow]T[-]       Iniciar TODOS los túneles
  [yellow]Enter[-]   Iniciar / desconectar túnel
  [yellow]Esc / ←[-]  Volver a lista de conexiones

[orange::b]Atajos — formulario[-]
  [yellow]Tab[-]     Siguiente campo
  [yellow]Esc[-]     Cancelar sin guardar`,
		},
		{
			title: "Cifrado de tokens",
			content: `[orange::b]Protección de secretos en disco[-]

Bifrost puede almacenar tokens cifrados en los archivos
de configuración para evitar que queden en texto plano.

[orange::b]Algoritmo[-]
  [yellow]AES-GCM 256[-] — cifrado simétrico autenticado.
  Los valores cifrados se guardan con el prefijo
  [white]ENC:[-] seguido del payload en Base64.

[orange::b]Generar un token cifrado manualmente[-]
  [yellow]./bifrost encryptsecret[-]

  1. Escriba el secreto (caracteres enmascarados con *)
  2. Presione Enter
  3. Copie el valor mostrado: [white]ENC:...[-]
  4. Péguelo en [white]token[-] (cliente) o [white]secret[-]
     (servidor) del archivo .conf

[orange::b]Comportamiento en la TUI[-]
  • Al guardar una conexión, si el token está en texto
    plano, se cifra automáticamente antes de escribir
    el archivo.
  • Si ya tiene prefijo [white]ENC:[-], se conserva tal cual.
  • Al editar, el token se muestra descifrado en el
    formulario para facilitar la modificación.

[orange::b]Comportamiento en el servidor[-]
  Al iniciar [yellow]./bifrost server[-], si [white]secret[-]
  tiene prefijo [white]ENC:[-], se descifra en memoria
  antes de validar las conexiones entrantes.

[orange::b]Recomendaciones de seguridad[-]
  • Use tokens largos y aleatorios (32+ caracteres)
  • Prefiera siempre [white]ENC:...[-] en archivos de config
  • Restrinja permisos del directorio [white]config/[-]
  • No comparta archivos .conf por canales inseguros
  • Use TLS ([white]use_tls[-]) en entornos de producción

[silver]El cifrado ofusca secretos en disco; protege
frente a lectura casual del archivo pero no reemplaza
un gestor de claves externo (KMS/Vault).[-]`,
		},
		{
			title: "Modo servidor",
			content: `[orange::b]bifrost server[-]

Inicia el demonio que acepta conexiones de clientes
Bifrost y reenvía el tráfico TCP a los destinos indicados.

[orange::b]Requisitos[-]
  • Archivo [white]config/server.conf[-] válido
  • Puerto [white]port[-] accesible desde los clientes
  • Token [white]secret[-] configurado e idéntico en clientes

[orange::b]Proceso de conexión[-]
  1. El cliente inicia handshake WebSocket
  2. El servidor valida el token en headers HTTP
  3. Se establece la sesión multiplexada
  4. Por cada conexión TCP local del cliente, se abre
     un stream hacia el destino indicado

[orange::b]Autenticación[-]
  El cliente envía el token en uno de estos headers:
    • [white]Authorization[-]
    • [white]X-Tunnel-Token[-]

  Debe coincidir exactamente con [white]secret[-] del
  servidor. Conexiones sin token válido son rechazadas.

[orange::b]Destino del túnel[-]
  Cada stream TCP indica su destino con el header:
    • [white]X-Tunnel-Target: host:puerto[-]

  La TUI lo envía automáticamente según el campo
  [white]target[-] de cada túnel configurado.

[orange::b]Multiplexación[-]
  Múltiples conexiones TCP simultáneas comparten una
  sola sesión WebSocket, reduciendo overhead de red
  y permitiendo cientos de conexiones concurrentes.

[orange::b]Compatibilidad[-]
  • Compresión WebSocket habilitada
  • [white]CheckOrigin[-] permite cualquier origen
    (adecuado detrás de proxy inverso)
  • Funciona con Traefik, Nginx y otros proxies WS

[orange::b]Apagado y señales[-]
  [yellow]Ctrl+C[-] — interrupción desde terminal
  [yellow]SIGTERM[-] — apagado controlado (systemd, Docker)
  Al cerrar, las conexiones activas se terminan.`,
		},
		{
			title: "Modo cliente CLI",
			content: `[orange::b]bifrost client[-]

Cliente mínimo por línea de comandos, sin interfaz TUI.
Útil para scripts, pruebas rápidas o entornos sin terminal
interactiva.

[orange::b]Sintaxis[-]
  [white]./bifrost client [flags][-]

[orange::b]Flags disponibles[-]

  [yellow]-u, --url[-]
    URL WebSocket completa del servidor.
    Default: [white]ws://localhost:8000/tunnel[-]

  [yellow]-t, --token[-]
    Token de autenticación compartido con el servidor.
    Default: [white]super_secreto[-]

  [yellow]-l, --listen[-]
    Dirección local donde escucha el proxy TCP.
    Default: [white]127.0.0.1:9090[-]

[orange::b]Ejemplo básico[-]
  [white]./bifrost client \
    -u wss://proxy.ejemplo.com:443/tunnel \
    -t mi_token_secreto \
    -l 127.0.0.1:8080[-]

[orange::b]Estados reportados[-]
  [yellow]Iniciando Local[-]  — abriendo puerto local
  [yellow]Conectando...[-]     — handshake WebSocket
  [green]Conectado[-]         — túnel operativo
  [red]Error (Reintentando)[-] — fallo con reintento
  [gray]Desconectado[-]        — sesión cerrada

[orange::b]Reconexión automática[-]
  Si la conexión al servidor se pierde, el cliente
  reintenta con backoff exponencial:
    1s → 2s → 4s → ... → máximo 30s entre intentos

[orange::b]Limitaciones respecto a la TUI[-]
  • Un solo túnel por proceso
  • Sin configuración persistente en JSON
  • Sin métricas visuales de tráfico
  • Sin gestión de múltiples servidores
  • El destino remoto no es configurable vía flags

[silver]Para uso habitual, se recomienda la TUI
([yellow]./bifrost[-]) que ofrece gestión completa
de conexiones, túneles y monitoreo en vivo.[-]`,
		},
	}
}

func newHelpPage(app *tview.Application, version string) (tview.Primitive, *tview.List, *tview.TextView) {
	topics := bifrostHelpTopics(version)

	contentView := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true).
		SetWrap(true).
		SetChangedFunc(func() {
			app.Draw()
		})
	contentView.SetBackgroundColor(uiFormBg)
	contentView.SetBorderPadding(1, 1, 2, 2)
	contentView.SetText(topics[0].content)

	contentPanel := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(contentView, 0, 1, true)
	contentPanel.SetBorder(true).
		SetTitle(" "+topics[0].title+" ").
		SetTitleAlign(tview.AlignLeft).
		SetTitleColor(uiModalAccent).
		SetBorderColor(uiModalBorder).
		SetBackgroundColor(uiFormBg)

	topicList := tview.NewList().
		ShowSecondaryText(false).
		SetHighlightFullLine(true).
		SetMainTextStyle(tcell.StyleDefault.
			Foreground(tcell.ColorWhite).
			Background(uiFormFieldBg)).
		SetSelectedStyle(tcell.StyleDefault.
			Background(uiModalBorder).
			Foreground(tcell.ColorBlack).
			Attributes(tcell.AttrBold))
	topicList.SetBackgroundColor(uiFormFieldBg)
	topicList.SetBorderPadding(1, 0, 1, 1)

	topicsPanel := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(topicList, 0, 1, true)
	topicsPanel.SetBorder(true).
		SetTitle(" Temas ").
		SetTitleAlign(tview.AlignLeft).
		SetTitleColor(uiModalAccent).
		SetBorderColor(uiModalBorder).
		SetBackgroundColor(uiFormFieldBg)

	showTopic := func(index int) {
		if index >= 0 && index < len(topics) {
			contentView.SetText(topics[index].content)
			contentView.ScrollToBeginning()
			contentPanel.SetTitle(" " + topics[index].title + " ")
		}
	}

	for i, topic := range topics {
		idx := i
		topicList.AddItem("  "+topic.title, "", 0, func() {
			showTopic(idx)
		})
	}
	topicList.SetChangedFunc(func(index int, mainText, secondary string, shortcut rune) {
		showTopic(index)
	})

	panelDivider := tview.NewBox().SetBackgroundColor(tcell.GetColor("#3a4558"))

	shortcutsBar := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter).
		SetText(fmt.Sprintf("%s  %s  %s",
			fmt.Sprintf("[white]%s[-:-:-] Temas", tview.Escape("[↑↓]")),
			fmt.Sprintf("[white]%s[-:-:-] Desplazar", tview.Escape("[PgUp/PgDn]")),
			fmt.Sprintf("[white]%s[-:-:-] Cerrar", tview.Escape("[Ctrl+A]"))))
	shortcutsBar.SetBackgroundColor(tcell.GetColor("#252525"))

	accentBar := tview.NewBox().SetBackgroundColor(uiModalBorder)

	body := tview.NewFlex().
		AddItem(topicsPanel, 32, 0, true).
		AddItem(panelDivider, 1, 0, false).
		AddItem(contentPanel, 0, 1, true)

	card := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(body, 0, 1, true).
		AddItem(shortcutsBar, 1, 0, false).
		AddItem(accentBar, 1, 0, false)
	card.SetBorder(true).
		SetTitle(" Documentación ").
		SetTitleAlign(tview.AlignCenter).
		SetTitleColor(uiModalAccent).
		SetBorderColor(uiModalBorder).
		SetBackgroundColor(uiModalBg)

	return centeredOverlayPage(card, 104, 30), topicList, contentView
}
