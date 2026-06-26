package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/user/bifrost/internal/crypto"
	"github.com/user/bifrost/internal/tunnel"
)

func formatBytes(n int64) string {
	const (
		KB = 1024
		MB = 1024 * KB
		GB = 1024 * MB
		TB = 1024 * GB
	)
	switch {
	case n >= TB:
		return fmt.Sprintf("%.2f TB", float64(n)/TB)
	case n >= GB:
		return fmt.Sprintf("%.2f GB", float64(n)/GB)
	case n >= MB:
		return fmt.Sprintf("%.2f MB", float64(n)/MB)
	case n >= KB:
		return fmt.Sprintf("%.2f KB", float64(n)/KB)
	default:
		return fmt.Sprintf("%d B", n)
	}
}

var (
	uiModalBg      = tcell.GetColor("#1a2332")
	uiModalBorder  = tcell.GetColor("#ff8c00")
	uiModalAccent  = tcell.GetColor("#ffb347")
	uiOverlayBg    = tcell.GetColor("#0a0a0a")
	uiFormBg       = tcell.GetColor("#1e2430")
	uiFormFieldBg  = tcell.GetColor("#2a3444")
	uiFormButtonBg = tcell.GetColor("#2a3444")
)

func uiButtonStyles() (tcell.Style, tcell.Style) {
	normal := tcell.StyleDefault.
		Background(uiFormButtonBg).
		Foreground(tcell.ColorWhite)
	activated := tcell.StyleDefault.
		Background(uiModalBorder).
		Foreground(tcell.ColorBlack).
		Attributes(tcell.AttrBold)
	return normal, activated
}

func styleForm(form *tview.Form) {
	normalBtn, activatedBtn := uiButtonStyles()
	form.SetBackgroundColor(uiFormBg)
	form.SetFieldBackgroundColor(uiFormFieldBg)
	form.SetFieldTextColor(tcell.ColorWhite)
	form.SetLabelColor(uiModalAccent)
	form.SetButtonTextColor(tcell.ColorWhite)
	form.SetButtonBackgroundColor(uiFormButtonBg)
	form.SetButtonStyle(normalBtn)
	form.SetButtonActivatedStyle(activatedBtn)
	form.SetBorderColor(uiModalBorder)
	form.SetTitleColor(uiModalAccent)
	form.SetBorderPadding(1, 1, 2, 2)
}

func newStyledModal(text string, buttons []string) *tview.Modal {
	normalBtn, activatedBtn := uiButtonStyles()
	modal := tview.NewModal()
	modal.SetText(text)
	modal.AddButtons(buttons)
	modal.SetBackgroundColor(uiModalBg)
	modal.SetTextColor(tcell.ColorWhite)
	modal.SetBorderColor(uiModalBorder)
	modal.SetButtonTextColor(tcell.ColorWhite)
	modal.SetButtonBackgroundColor(uiFormButtonBg)
	modal.SetButtonStyle(normalBtn)
	modal.SetButtonActivatedStyle(activatedBtn)
	return modal
}

func centeredOverlayPage(content tview.Primitive, width, height int) *tview.Grid {
	grid := tview.NewGrid().SetBorders(false)
	grid.SetRows(0, height, 0).SetColumns(0, width, 0)
	grid.AddItem(tview.NewBox().SetBackgroundColor(uiOverlayBg), 0, 0, 3, 3, 0, 0, false)
	grid.AddItem(content, 1, 1, 1, 1, 0, 0, true)
	return grid
}

func newConfirmModal(title, message string, buttons []string, done func(int, string)) *tview.Grid {
	text := fmt.Sprintf("[#ffb347::b]%s[-]\n\n[white]%s[-]", title, message)
	modal := newStyledModal(text, buttons).SetDoneFunc(done)
	return centeredOverlayPage(modal, 72, 11)
}

func newFormPage(form *tview.Form, title string, formHeight int) *tview.Grid {
	styleForm(form)
	form.SetBorder(true).SetTitle(" "+title+" ").SetTitleAlign(tview.AlignCenter)

	accentBar := tview.NewBox().SetBackgroundColor(uiModalBorder)
	card := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(form, 0, 1, true).
		AddItem(accentBar, 1, 0, false)

	return centeredOverlayPage(card, 72, formHeight+1)
}

type tunnelDetail struct {
	tx         atomic.Int64
	rx         atomic.Int64
	reconnects atomic.Int64
	errors     atomic.Int64
	lastErr    atomic.Value // string

	lastSampleTx atomic.Int64
	lastSampleRx atomic.Int64
	lastSampleAt atomic.Int64 // unix nano
}

func (d *tunnelDetail) setErr(err error) {
	if err == nil {
		return
	}
	d.lastErr.Store(err.Error())
}

func (d *tunnelDetail) errString() string {
	v := d.lastErr.Load()
	if s, ok := v.(string); ok && s != "" {
		return s
	}
	return "-"
}

// tunnelConfig modela la conexión (origen y destino) para el cliente.
type tunnelConfig struct {
	Name        string `json:"name"`
	Listen      string `json:"listen"`
	Target      string `json:"target"`
	Autoconnect bool   `json:"autoconnect"`
}

// serverConfig modela un servidor remoto al que se conectará el cliente.
type serverConfig struct {
	Name        string         `json:"name"`
	URL         string         `json:"url"`
	Token       string         `json:"token"`
	UseTLS      bool           `json:"use_tls"`
	Insecure    bool           `json:"insecure"`
	Connections []tunnelConfig `json:"connections"`
}

type clientConfigFile struct {
	Servers []serverConfig `json:"servers"`
}

// tviewLogger nos permite desviar los logs del túnel hacia un widget de TextView
// en pantalla en vez del os.Stderr habitual.
type tviewLogger struct {
	tv  *tview.TextView
	app *tview.Application
	msg chan string
}

func newTviewLogger(app *tview.Application, tv *tview.TextView) *tviewLogger {
	l := &tviewLogger{
		app: app,
		tv:  tv,
		msg: make(chan string, 100),
	}

	// Goroutine que escucha mensajes para no bloquear slog
	go func() {
		for text := range l.msg {
			// Solicitamos a Tview que corra el render de los logs
			// de manera segura con el hilo UI.
			l.app.QueueUpdateDraw(func() {
				fmt.Fprint(l.tv, text)
				l.tv.ScrollToEnd()
			})
		}
	}()

	return l
}

func (l *tviewLogger) Write(p []byte) (n int, err error) {
	// Intentamos meter el log al canal. Si está súper atascado lo tira
	// (evitando deadlocks).
	select {
	case l.msg <- string(p):
	default:
	}
	return len(p), nil
}

func loadConfig() clientConfigFile {
	var file clientConfigFile
	// Intenta leer el archivo (junto al ejecutable real, no al cwd)
	data, err := os.ReadFile(clientConfigPath())
	if err == nil && len(data) > 3 {
		if marshalErr := json.Unmarshal(data, &file); marshalErr == nil {
			return file
		}
	}
	// Si no existe, es inválido o está en blanco, crea uno vacío
	cfg := fallbackFile()
	saveConfig(&cfg)
	return cfg
}

func saveConfig(file *clientConfigFile) {
	// Aseguramos que todos los tokens estén encriptados antes de persistir
	for i := range file.Servers {
		t := file.Servers[i].Token
		if t != "" && !strings.HasPrefix(t, "ENC:") {
			if enc, err := crypto.Encrypt(t); err == nil {
				file.Servers[i].Token = "ENC:" + enc
			}
		}
	}

	data, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		slog.Error("No se pudo estructurar el JSON para guardar", "error", err)
		return
	}
	path := clientConfigPath()
	_ = os.MkdirAll(configDirPath(), 0755)
	if err := os.WriteFile(path, data, 0644); err != nil {
		slog.Error("No se pudo guardar la configuración", "error", err)
	} else {
		slog.Info("Configuración guardada correctamente", "file", path)
	}
}

// StartInteractiveUI arranca la interfaz puramente terminal para seleccionar el túnel.
func StartInteractiveUI() error {
	version := getAppVersion()
	file := loadConfig()

	// Ajustamos el "Fondo gris" general a un gris verdaderamente oscuro (casi negro)
	tview.Styles.PrimitiveBackgroundColor = tcell.GetColor("#1a1a1a")

	app := tview.NewApplication()
	app.EnableMouse(true)

	// Cerrar la TUI con gracia al cerrar la ventana/pestaña del terminal o con Ctrl+C.
	go func() {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
		<-ch
		app.Stop()
	}()

	// Título principal arriba
	titleBar := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft).
		SetText(fmt.Sprintf("  [white::b]Bifrost - v%s[-:-:-]", version))
	titleBar.SetBackgroundColor(tcell.ColorDarkOrange)

	// Footer con opciones
	footerBar := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter)
	footerBar.SetBackgroundColor(tcell.ColorDarkGreen)

	// Área de Logs (Derecha abajo)
	logView := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true).
		SetChangedFunc(func() {
			app.Draw()
		})
	logView.SetBorder(false)
	logView.SetBorderPadding(0, 0, 1, 1)

	fmt.Fprintf(logView, `[orange]
  ██████╗ ██╗███████╗██████╗  ██████╗ ███████╗████████╗
  ██╔══██╗██║██╔════╝██╔══██╗██╔═══██╗██╔════╝╚══██╔══╝
  ██████╔╝██║█████╗  ██████╔╝██║   ██║███████╗   ██║   
  ██╔══██╗██║██╔══╝  ██╔══██╗██║   ██║╚════██║   ██║   
  ██████╔╝██║██║     ██║  ██║╚██████╔╝███████║   ██║   
  ╚═════╝ ╚═╝╚═╝     ╚═╝  ╚═╝ ╚═════╝ ╚══════╝   ╚═╝   

   [white]Version %s - Desarrollador: Hernan Lencinas[-]
   
`, version)

	// Instanciamos el interceptor de logs asíncrono
	tLogger := newTviewLogger(app, logView)
	handler := newCustomHandler(tLogger, slog.HandlerOptions{Level: slog.LevelInfo}, true)
	slog.SetDefault(slog.New(handler))

	// Menú de Selección (Izquierda)
	list := tview.NewList().ShowSecondaryText(true)
	list.SetBorder(false)
	list.SetBorderPadding(0, 0, 1, 1)

	var currentScope string
	var selectedServer int
	var populateServers func()
	var populateTunnels func(srvIdx int)
	var updateActiveTable func()
	var updateFooter func()
	var updateLeftShortcuts func()

	// Panel izquierdo: Usamos páginas para alternar entre la lista y un mensaje de ayuda
	leftPane := tview.NewPages()
	var leftPanelContainer *tview.Flex

	leftShortcutsBar := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter).
		SetWrap(true)
	leftShortcutsBar.SetBackgroundColor(tcell.GetColor("#252525"))
	leftShortcutsBar.SetBorderPadding(0, 0, 1, 0)

	helpView := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter).
		SetWrap(true).
		SetTextColor(tcell.ColorWhite)
	helpView.SetBorder(false)
	helpView.SetBorderPadding(1, 1, 1, 1)

	setHelpContent := func(scope string, serverName string) {
		if scope == "server" {
			leftPanelContainer.SetTitle(" Comenzar ")
			helpView.SetText(`
[orange]¡Bienvenido a Bifrost![-]

Parece que aún no tienes
conexiones configuradas.

Para empezar, presiona la tecla [yellow::b]A[-]
en tu teclado para crear una nueva.

[yellow]Necesitarás:[-]
• Nombre de la conexión
• Dirección:Puerto del servidor
• Token de acceso

[silver]Usa TAB para navegar y
Ctrl+Q para salir.[-]`)
		} else {
			leftPanelContainer.SetTitle(" Configurar Túnel ")
			helpView.SetText(fmt.Sprintf(`
[orange]Servidor: %s[-]

Aún no has definido túneles para
este servidor.

Presiona la tecla [yellow::b]A[-] para crear un
nuevo túnel y comenzar a redirigir tráfico.

[yellow]Necesitarás:[-]
• Nombre del túnel
• Puerto Local (ej: :8080)
• Destino Remoto (ej: 127.0.0.1:80)

[silver]Presiona Esc o Flecha Izquierda
para volver a la lista.[-]`, serverName))
		}
	}

	// Diccionario para manejar los procesos en vivo (guardamos el Context Cancel)
	activeTunnels := make(map[string]context.CancelFunc)
	activeStatuses := make(map[string]string)
	var activeDetails sync.Map // map[tunnelID]*tunnelDetail

	startTunnel := func(srvIdx, connIdx int) {
		tunnelID := fmt.Sprintf("%d-%d", srvIdx, connIdx)
		if _, exists := activeTunnels[tunnelID]; exists {
			return // Ya está activo
		}

		srv := file.Servers[srvIdx]
		c := srv.Connections[connIdx]

		// Proceso de desencriptado
		secret := srv.Token
		if strings.HasPrefix(secret, "ENC:") {
			decrypted, err := crypto.Decrypt(strings.TrimPrefix(secret, "ENC:"))
			if err != nil {
				slog.Error("Fallo intentando desencriptar", "error", err)
				return
			}
			secret = decrypted
		}

		// Auto-completar URL
		fullURL := srv.URL
		if !strings.HasPrefix(fullURL, "ws://") && !strings.HasPrefix(fullURL, "wss://") {
			if srv.UseTLS || strings.HasSuffix(fullURL, ":443") {
				fullURL = "wss://" + fullURL
			} else {
				fullURL = "ws://" + fullURL
			}
		}
		if !strings.HasSuffix(fullURL, "/tunnel") {
			fullURL = strings.TrimSuffix(fullURL, "/") + "/tunnel"
		}

		client := tunnel.NewClient(tunnel.ClientOptions{
			ServerURL:          fullURL,
			Token:              secret,
			ListenAddr:         c.Listen,
			TargetAddr:         c.Target,
			InsecureSkipVerify: srv.Insecure,
			OnStatusChange: func(status string) {
				app.QueueUpdateDraw(func() {
					if _, active := activeTunnels[tunnelID]; active {
						activeStatuses[tunnelID] = status
						updateActiveTable()
					}
				})
			},
			OnTraffic: func(txDelta, rxDelta int64) {
				if v, ok := activeDetails.Load(tunnelID); ok {
					d := v.(*tunnelDetail)
					if txDelta != 0 {
						d.tx.Add(txDelta)
					}
					if rxDelta != 0 {
						d.rx.Add(rxDelta)
					}
				}
				app.QueueUpdateDraw(func() {
					if _, active := activeTunnels[tunnelID]; active {
						updateActiveTable()
					}
				})
			},
			OnReconnect: func(totalReconnects int64) {
				if v, ok := activeDetails.Load(tunnelID); ok {
					v.(*tunnelDetail).reconnects.Store(totalReconnects)
				}
				app.QueueUpdateDraw(func() {
					if _, active := activeTunnels[tunnelID]; active {
						updateActiveTable()
					}
				})
			},
			OnError: func(err error) {
				if v, ok := activeDetails.Load(tunnelID); ok {
					d := v.(*tunnelDetail)
					d.errors.Add(1)
					d.setErr(err)
				}
				app.QueueUpdateDraw(func() {
					if _, active := activeTunnels[tunnelID]; active {
						updateActiveTable()
					}
				})
			},
		})

		ctx, cancel := context.WithCancel(context.Background())
		activeTunnels[tunnelID] = cancel
		activeStatuses[tunnelID] = "Iniciando"
		activeDetails.Store(tunnelID, &tunnelDetail{})

		go func(id string, tName string) {
			err := client.Start(ctx)
			app.QueueUpdateDraw(func() {
				if _, active := activeTunnels[id]; active && err != nil && err != context.Canceled {
					slog.Error("El túnel cliente falló", "tunel", tName, "error", err)
					delete(activeTunnels, id)
					delete(activeStatuses, id)
					activeDetails.Delete(id)
					updateActiveTable()
					if currentScope == "tunnel" && selectedServer == srvIdx {
						populateTunnels(srvIdx)
					}
				}
			})
		}(tunnelID, c.Name)
	}

	// Panel de conexiones activas (Derecha arriba)
	activeTable := tview.NewTable().
		SetBorders(false).
		SetSelectable(true, false).
		SetFixed(1, 0).
		SetSelectedStyle(tcell.StyleDefault.
			Background(tcell.ColorWhite).
			Foreground(tcell.ColorBlack))
	activeTable.SetBorder(false)
	activeTable.SetBorderPadding(0, 0, 1, 1)

	activeTableShortcutsBar := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter).
		SetWrap(false)
	activeTableShortcutsBar.SetBackgroundColor(tcell.GetColor("#252525"))
	activeTableShortcutsBar.SetText(fmt.Sprintf("%s  %s",
		fmt.Sprintf("[white]%s[-:-:-] Desconectar", tview.Escape("[D]")),
		fmt.Sprintf("[white]%s[-:-:-] Detener Todos", tview.Escape("[K]"))))

	activeTablePanel := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(activeTable, 0, 1, false).
		AddItem(activeTableShortcutsBar, 1, 0, false)
	activeTablePanel.SetBorder(true).SetTitle(" Túneles Activos ")

	activeTableHeaderBg := tcell.GetColor("#2f3d52")
	newActiveHeaderCell := func(text string, indent bool) *tview.TableCell {
		label := text + " "
		if indent {
			label = " " + label
		}
		return tview.NewTableCell(label).
			SetTextColor(tcell.ColorWhite).
			SetAttributes(tcell.AttrBold).
			SetBackgroundColor(activeTableHeaderBg).
			SetExpansion(1).
			SetSelectable(false)
	}

	rowToTID := make(map[int]string)

	formatShortcut := func(key, label string) string {
		return fmt.Sprintf("[white]%s[-:-:-] %s", tview.Escape("["+key+"]"), label)
	}

	logShortcutsBar := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter).
		SetWrap(false)
	logShortcutsBar.SetBackgroundColor(tcell.GetColor("#252525"))
	logShortcutsBar.SetText(formatShortcut("C", "Limpiar"))

	logPanel := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(logView, 0, 1, false).
		AddItem(logShortcutsBar, 1, 0, false)
	logPanel.SetBorder(true).SetTitle(" Consola ")

	updateLeftShortcuts = func() {
		switch currentScope {
		case "server":
			leftShortcutsBar.SetText(fmt.Sprintf("%s  %s  %s  %s",
				formatShortcut("A", "Nueva"),
				formatShortcut("E", "Editar"),
				formatShortcut("D", "Eliminar"),
				formatShortcut("Enter", "Túneles")))
		case "tunnel":
			leftShortcutsBar.SetText(fmt.Sprintf("%s  %s  %s  %s\n%s  %s",
				formatShortcut("A", "Nuevo"),
				formatShortcut("E", "Editar"),
				formatShortcut("D", "Eliminar"),
				formatShortcut("T", "Iniciar Todos"),
				formatShortcut("Enter", "Iniciar/Detener"),
				formatShortcut("Esc", "Volver")))
		}
	}

	updateFooter = func() {
		updateLeftShortcuts()

		focus := app.GetFocus()

		// Reset de bordes
		leftPanelContainer.SetBorderColor(tcell.ColorWhite)
		activeTablePanel.SetBorderColor(tcell.ColorWhite)
		logPanel.SetBorderColor(tcell.ColorWhite)

		var text string
		if focus == list || focus == helpView {
			leftPanelContainer.SetBorderColor(tcell.ColorYellow)
			text = fmt.Sprintf("[white]%s[-] Siguiente panel", tview.Escape("[Tab]"))
		} else if focus == activeTable {
			activeTablePanel.SetBorderColor(tcell.ColorYellow)
			text = fmt.Sprintf("[white]%s[-] Siguiente panel", tview.Escape("[Tab]"))
		} else if focus == logView {
			logPanel.SetBorderColor(tcell.ColorYellow)
			text = fmt.Sprintf("[white]%s[-] Siguiente panel", tview.Escape("[Tab]"))
		}
		footerBar.SetText(text + "  [white]" + tview.Escape("[Ctrl+A]") + "[-] Ayuda  [white]" + tview.Escape("[Ctrl+Q]") + "[-] Salir")
	}

	list.SetFocusFunc(func() { updateFooter() })
	helpView.SetFocusFunc(func() { updateFooter() })
	activeTable.SetFocusFunc(func() { updateFooter() })
	logView.SetFocusFunc(func() { updateFooter() })

	app.SetInputCapture(func(e *tcell.EventKey) *tcell.EventKey {
		updateFooter()
		return e
	})

	updateActiveTable = func() {
		activeTable.Clear()
		activeTable.SetCell(0, 0, newActiveHeaderCell("Conexión", true))
		activeTable.SetCell(0, 1, newActiveHeaderCell("Túnel", false))
		activeTable.SetCell(0, 2, newActiveHeaderCell("Local/Destino", false))
		activeTable.SetCell(0, 3, newActiveHeaderCell("Estado", false))

		rowToTID = make(map[int]string)

		row := 1
		for i, srv := range file.Servers {
			for j, conn := range srv.Connections {
				tID := fmt.Sprintf("%d-%d", i, j)
				if _, ok := activeTunnels[tID]; ok {
					mainRow := row
					rowToTID[mainRow] = tID
					activeTable.SetCell(mainRow, 0, tview.NewTableCell(" "+srv.Name).SetTextColor(tcell.ColorWhite))
					activeTable.SetCell(mainRow, 1, tview.NewTableCell(conn.Name).SetTextColor(tcell.ColorWhite))
					activeTable.SetCell(mainRow, 2, tview.NewTableCell(conn.Listen+" ↔ "+conn.Target).SetTextColor(tcell.ColorGreen))

					status := activeStatuses[tID]
					if status == "" {
						status = "Iniciando"
					}

					color := tcell.ColorWhite
					switch {
					case strings.Contains(status, "Error") || strings.Contains(status, "Fallo"):
						color = tcell.ColorRed
					case status == "Conectado":
						color = tcell.ColorGreen
					case status == "Conectando...":
						color = tcell.ColorYellow
					case status == "Desconectado":
						color = tcell.ColorGray
					}

					activeTable.SetCell(mainRow, 3, tview.NewTableCell(status).SetTextColor(color))

					var (
						tx      int64
						rx      int64
						txRate  int64
						rxRate  int64
						reconns int64
						errs    int64
					)

					if v, ok := activeDetails.Load(tID); ok {
						d := v.(*tunnelDetail)
						tx = d.tx.Load()
						rx = d.rx.Load()
						reconns = d.reconnects.Load()
						errs = d.errors.Load()

						now := time.Now().UnixNano()
						prevAt := d.lastSampleAt.Load()
						prevTx := d.lastSampleTx.Load()
						prevRx := d.lastSampleRx.Load()
						if prevAt != 0 && now > prevAt {
							dt := float64(now-prevAt) / float64(time.Second)
							if dt > 0 {
								txRate = int64(float64(tx-prevTx) / dt)
								rxRate = int64(float64(rx-prevRx) / dt)
							}
						}
						d.lastSampleAt.Store(now)
						d.lastSampleTx.Store(tx)
						d.lastSampleRx.Store(rx)
					}

					detailRow := mainRow + 1
					localDestinoDetail := fmt.Sprintf(
						" ↳ Enviado: %s (%s/s) • Recibido: %s (%s/s)",
						formatBytes(tx),
						formatBytes(txRate),
						formatBytes(rx),
						formatBytes(rxRate),
					)
					estadoDetail := fmt.Sprintf(" ↳ Reconexiones: %d • Errores: %d", reconns, errs)

					activeTable.SetCell(detailRow, 0, tview.NewTableCell("").SetSelectable(false))
					activeTable.SetCell(detailRow, 1, tview.NewTableCell("").SetSelectable(false))
					activeTable.SetCell(detailRow, 2, tview.NewTableCell(localDestinoDetail).SetTextColor(tcell.ColorGray).SetSelectable(false))
					activeTable.SetCell(detailRow, 3, tview.NewTableCell("").SetSelectable(false))

					detailRow2 := mainRow + 2
					activeTable.SetCell(detailRow2, 0, tview.NewTableCell("").SetSelectable(false))
					activeTable.SetCell(detailRow2, 1, tview.NewTableCell("").SetSelectable(false))
					activeTable.SetCell(detailRow2, 2, tview.NewTableCell(estadoDetail).SetTextColor(tcell.ColorGray).SetSelectable(false))
					activeTable.SetCell(detailRow2, 3, tview.NewTableCell("").SetSelectable(false))

					row += 3
				}
			}
		}
	}
	updateActiveTable() // Call initial empty state

	pages := tview.NewPages()

	var focusBeforeHelp tview.Primitive
	var helpTopicList *tview.List
	var helpContentView *tview.TextView

	toggleHelp := func() {
		if pages.HasPage("help") {
			pages.RemovePage("help")
			if focusBeforeHelp != nil {
				app.SetFocus(focusBeforeHelp)
			}
			updateFooter()
			return
		}
		focusBeforeHelp = app.GetFocus()
		helpPage, topicList, contentView := newHelpPage(version)
		helpTopicList = topicList
		helpContentView = contentView
		pages.AddPage("help", helpPage, true, true)
		app.SetFocus(helpTopicList)
	}

	populateServers = func() {
		currentScope = "server"
		list.Clear()
		leftPanelContainer.SetTitle(" Conexiones ")
		list.ShowSecondaryText(false)

		if len(file.Servers) == 0 {
			setHelpContent("server", "")
			leftPane.SwitchToPage("help")
			app.SetFocus(helpView)
		} else {
			leftPane.SwitchToPage("list")
		}

		for i, srv := range file.Servers {
			idx := i
			s := srv // loop copy

			displayURL := s.URL
			if u, err := url.Parse(s.URL); err == nil && u.Host != "" {
				displayURL = u.Host
			} else {
				displayURL = strings.TrimPrefix(displayURL, "ws://")
				displayURL = strings.TrimPrefix(displayURL, "wss://")
				displayURL = strings.TrimSuffix(displayURL, "/tunnel")
			}

			list.AddItem(" » "+s.Name+" ", "", 0, func() {
				populateTunnels(idx)
			})
		}
		updateFooter()
	}

	populateTunnels = func(srvIdx int) {
		currentScope = "tunnel"
		selectedServer = srvIdx
		srv := file.Servers[srvIdx]
		list.ShowSecondaryText(true)

		if len(srv.Connections) == 0 {
			setHelpContent("tunnel", srv.Name)
			leftPane.SwitchToPage("help")
			app.SetFocus(helpView)
		} else {
			leftPane.SwitchToPage("list")
		}
		list.Clear()
		leftPanelContainer.SetTitle(fmt.Sprintf(" Túneles en %s ", srv.Name))

		for i, conn := range srv.Connections {
			c := conn // local loop copy
			idx := i

			tunnelID := fmt.Sprintf("%d-%d", selectedServer, idx)
			status := "[yellow][ Inactivo ]"
			if _, exists := activeTunnels[tunnelID]; exists {
				status = "[yellow][ Activo ]"
			}

			desc := fmt.Sprintf("   ↳ %s ↔ %s %s", c.Listen, c.Target, status)
			if c.Autoconnect {
				desc += " [silver][Auto][-]"
			}

			list.AddItem(" » "+c.Name+" ", desc, 0, func() {
				// Acción al apretar Enter sobre un ítem.
				// Si ya está activo, lo cancelamos.
				if cancel, exists := activeTunnels[tunnelID]; exists {
					pages.AddPage("modal", newConfirmModal(
						"Desconectar túnel",
						fmt.Sprintf("¿Desea desconectar el túnel [%s]?", c.Name),
						[]string{"Si", "No"},
						func(buttonIndex int, buttonLabel string) {
							if buttonLabel == "Si" {
								// No llamar cancel() en el hilo UI: los callbacks del túnel
								// usan QueueUpdateDraw, que bloquea hasta que el bucle principal
								// ejecute la cola (deadlock si seguimos dentro del modal).
								go func(cancel context.CancelFunc, tid, tunnelName string) {
									app.QueueUpdateDraw(func() {
										delete(activeTunnels, tid)
										delete(activeStatuses, tid)
										activeDetails.Delete(tid)
										slog.Info("Túnel detenido manualmente", "tunel", tunnelName)
										updateActiveTable()
										populateTunnels(selectedServer)
									})
									cancel()
								}(cancel, tunnelID, c.Name)
							}
							pages.RemovePage("modal")
							app.SetFocus(list)
						}), true, true)
					return
				}

				startTunnel(selectedServer, idx)
				updateActiveTable()
				slog.Info("Proceso de túnel encolado", "tunel", c.Name)
				populateTunnels(selectedServer)
			})
		}
		updateFooter()
	}

	// Formularios
	showServerForm := func(editIdx int) {
		form := tview.NewForm()
		s := serverConfig{}
		isEdit := editIdx >= 0
		if isEdit {
			s = file.Servers[editIdx]
		}

		tokenToShow := s.Token
		if strings.HasPrefix(tokenToShow, "ENC:") {
			if decrypted, err := crypto.Decrypt(strings.TrimPrefix(tokenToShow, "ENC:")); err == nil {
				tokenToShow = decrypted
			}
		}

		form.AddInputField("Nombre", s.Name, 0, nil, func(text string) { s.Name = text }).
			AddInputField("Dirección:Puerto", s.URL, 0, nil, func(text string) { s.URL = text }).
			AddPasswordField("Token", tokenToShow, 0, '*', func(text string) { s.Token = text }).
			AddCheckbox("Usar TLS (WSS)", s.UseTLS, func(checked bool) { s.UseTLS = checked }).
			AddCheckbox("Ignorar errores TLS (Insecure)", s.Insecure, func(checked bool) { s.Insecure = checked }).
			SetButtonsAlign(tview.AlignCenter).
			AddButton("Cancelar", func() {
				pages.RemovePage("form")
				app.SetFocus(list)
				updateFooter()
			}).
			AddButton("Guardar", func() {
				if s.Name == "" || s.URL == "" {
					return
				}

				// Encriptamos el token antes de guardar
				if s.Token != "" && !strings.HasPrefix(s.Token, "ENC:") {
					if encrypted, err := crypto.Encrypt(s.Token); err == nil {
						s.Token = "ENC:" + encrypted
					}
				}
				if isEdit {
					s.Connections = file.Servers[editIdx].Connections
					file.Servers[editIdx] = s
				} else {
					file.Servers = append(file.Servers, s)
				}
				saveConfig(&file)
				populateServers()
				pages.RemovePage("form")
				app.SetFocus(list)
				updateFooter()
			})

		title := "Nueva Conexión"
		if isEdit {
			title = "Editar Conexión"
		}

		pages.AddPage("form", newFormPage(form, title, 16), true, true)
	}

	showTunnelForm := func(editIdx int) {
		form := tview.NewForm()
		c := tunnelConfig{}
		isEdit := editIdx >= 0
		if isEdit {
			c = file.Servers[selectedServer].Connections[editIdx]
		}

		form.AddInputField("Nombre Túnel", c.Name, 0, nil, func(text string) { c.Name = text }).
			AddInputField("Direccion:Puerto Local", c.Listen, 0, nil, func(text string) { c.Listen = text }).
			AddInputField("Direccion:Puerto Remoto", c.Target, 0, nil, func(text string) { c.Target = text }).
			AddCheckbox("Conectar al iniciar", c.Autoconnect, func(checked bool) { c.Autoconnect = checked }).
			SetButtonsAlign(tview.AlignCenter).
			AddButton("Cancelar", func() {
				pages.RemovePage("form")
				app.SetFocus(list)
				updateFooter()
			}).
			AddButton("Guardar", func() {
				if c.Name == "" || c.Listen == "" || c.Target == "" {
					return
				}
				if isEdit {
					tID := fmt.Sprintf("%d-%d", selectedServer, editIdx)
					if cancel, ok := activeTunnels[tID]; ok {
						go func(cancel context.CancelFunc, id string, cfg tunnelConfig) {
							app.QueueUpdateDraw(func() {
								delete(activeTunnels, id)
								delete(activeStatuses, id)
								activeDetails.Delete(id)
							})
							cancel()
							app.QueueUpdateDraw(func() {
								file.Servers[selectedServer].Connections[editIdx] = cfg
								saveConfig(&file)
								populateTunnels(selectedServer)
								pages.RemovePage("form")
								app.SetFocus(list)
								updateFooter()
							})
						}(cancel, tID, c)
						return
					}
					file.Servers[selectedServer].Connections[editIdx] = c
				} else {
					file.Servers[selectedServer].Connections = append(file.Servers[selectedServer].Connections, c)
				}
				saveConfig(&file)
				populateTunnels(selectedServer)
				pages.RemovePage("form")
				app.SetFocus(list)
				updateFooter()
			})

		title := "Nuevo Túnel"
		if isEdit {
			title = "Editar Túnel"
		}

		pages.AddPage("form", newFormPage(form, title, 14), true, true)
	}

	rightFlex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(activeTablePanel, 0, 3, false).
		AddItem(logPanel, 0, 2, false)

	leftPane.AddPage("list", list, true, true)
	leftPane.AddPage("help", helpView, true, false)

	leftPanelContainer = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(leftPane, 0, 1, true).
		AddItem(leftShortcutsBar, 2, 0, false)
	leftPanelContainer.SetBorder(true).SetTitle(" Conexiones ")

	// Layout principal dividido con márgenes (Padding lateral)
	contentFlex := tview.NewFlex().
		AddItem(nil, 1, 0, false). // Margen izquierdo
		AddItem(leftPanelContainer, 0, 1, true).
		AddItem(nil, 1, 0, false). // Espacio central
		AddItem(rightFlex, 0, 2, false).
		AddItem(nil, 1, 0, false) // Margen derecho

	// Layout con título arriba y footer abajo
	mainFlex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(titleBar, 1, 0, false).
		AddItem(nil, 1, 0, false). // Margen superior
		AddItem(contentFlex, 0, 1, true).
		AddItem(nil, 1, 0, false). // Margen inferior
		AddItem(footerBar, 1, 0, false)

	pages.AddPage("main", mainFlex, true, true)

	// Ejecutamos la carga inicial ahora que todo está configurado
	populateServers()

	for srvIdx, srv := range file.Servers {
		for connIdx, conn := range srv.Connections {
			if conn.Autoconnect {
				startTunnel(srvIdx, connIdx)
			}
		}
	}
	updateActiveTable()

	// Manejo de teclas globales
	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyTab {
			cur := app.GetFocus()
			if cur == list {
				app.SetFocus(activeTable)
			} else if cur == activeTable {
				app.SetFocus(logView)
			} else {
				app.SetFocus(list)
			}
			updateFooter()
			return nil
		}

		if event.Key() == tcell.KeyCtrlA {
			if pages.HasPage("form") || pages.HasPage("modal") {
				return event
			}
			toggleHelp()
			return nil
		}

		if pages.HasPage("help") {
			if event.Key() == tcell.KeyEsc {
				toggleHelp()
				return nil
			}
			if event.Key() == tcell.KeyTab {
				cur := app.GetFocus()
				if cur == helpTopicList {
					app.SetFocus(helpContentView)
				} else {
					app.SetFocus(helpTopicList)
				}
				return nil
			}
			return event
		}

		if pages.HasPage("form") || pages.HasPage("modal") {
			if event.Key() == tcell.KeyEsc {
				pages.RemovePage("form")
				pages.RemovePage("modal")
				app.SetFocus(list)
				updateFooter()
				return nil
			}
			return event
		}

		// Atajos exclusivos de la tabla de Conexiones Activas
		if app.GetFocus() == activeTable {
			if event.Rune() == 'd' || event.Rune() == 'D' {
				row, _ := activeTable.GetSelection()
				if tID, ok := rowToTID[row]; ok {
					pages.AddPage("modal", newConfirmModal(
						"Detener túnel",
						fmt.Sprintf("¿Desea detener el túnel [%s]?", tID),
						[]string{"Si", "No"},
						func(buttonIndex int, buttonLabel string) {
							if buttonLabel == "Si" {
								if cancel, ok := activeTunnels[tID]; ok {
									go func(cancel context.CancelFunc, id string) {
										app.QueueUpdateDraw(func() {
											delete(activeTunnels, id)
											delete(activeStatuses, id)
											activeDetails.Delete(id)
											updateActiveTable()
											if currentScope == "tunnel" {
												populateTunnels(selectedServer)
											}
										})
										cancel()
									}(cancel, tID)
								}
							}
							pages.RemovePage("modal")
							app.SetFocus(activeTable)
							updateFooter()
						}), true, true)
					return nil
				}
			} else if event.Rune() == 'k' || event.Rune() == 'K' {
				if len(activeTunnels) == 0 {
					return nil
				}
				pages.AddPage("modal", newConfirmModal(
					"Detener todos",
					fmt.Sprintf("¿Desea detener TODAS las conexiones activas (%d)?", len(activeTunnels)),
					[]string{"Si", "No"},
					func(buttonIndex int, buttonLabel string) {
						if buttonLabel == "Si" {
							snapshot := make(map[string]context.CancelFunc, len(activeTunnels))
							for id, cancel := range activeTunnels {
								snapshot[id] = cancel
							}
							go func(m map[string]context.CancelFunc) {
								app.QueueUpdateDraw(func() {
									for id := range m {
										delete(activeTunnels, id)
										delete(activeStatuses, id)
										activeDetails.Delete(id)
									}
									updateActiveTable()
									if currentScope == "tunnel" {
										populateTunnels(selectedServer)
									}
								})
								for _, cancel := range m {
									cancel()
								}
							}(snapshot)
						}
						pages.RemovePage("modal")
						app.SetFocus(activeTable)
						updateFooter()
					}), true, true)
				return nil
			}
		}

		// Atajos exclusivos de la ventana de Logs
		if app.GetFocus() == logView && (event.Rune() == 'c' || event.Rune() == 'C') {
			logView.Clear()
			return nil
		}

		// Atajos exclusivos de la ventana de Ayuda (cuando no hay conexiones o túneles)
		if app.GetFocus() == helpView {
			if event.Rune() == 'a' {
				if currentScope == "server" {
					showServerForm(-1)
				} else {
					showTunnelForm(-1)
				}
				return nil
			}
			if currentScope == "tunnel" && (event.Key() == tcell.KeyEsc || event.Key() == tcell.KeyLeft) {
				populateServers()
				return nil
			}
		}

		// Atajos exclusivos de la Lista de Servidores/Túneles (Izquierda)
		if app.GetFocus() == list {
			idx := list.GetCurrentItem()
			if currentScope == "server" {
				validItem := idx >= 0 && idx < len(file.Servers)
				if event.Rune() == 'a' {
					showServerForm(-1)
					return nil
				} else if validItem && event.Rune() == 'e' {
					showServerForm(idx)
					return nil
				} else if validItem && event.Rune() == 'd' {
					serverName := file.Servers[idx].Name
					pages.AddPage("modal", newConfirmModal(
						"Eliminar conexión",
						fmt.Sprintf("¿Está seguro que desea eliminar la conexión [%s]?\nEsto detendrá todos sus túneles activos.", serverName),
						[]string{"Si", "No"},
						func(buttonIndex int, buttonLabel string) {
							if buttonLabel == "Si" {
								srvIdx := idx
								go func() {
									var tIDs []string
									var cancels []context.CancelFunc
									for i := range file.Servers[srvIdx].Connections {
										tID := fmt.Sprintf("%d-%d", srvIdx, i)
										if cancel, ok := activeTunnels[tID]; ok {
											tIDs = append(tIDs, tID)
											cancels = append(cancels, cancel)
										}
									}
									app.QueueUpdateDraw(func() {
										for _, tID := range tIDs {
											delete(activeTunnels, tID)
											delete(activeStatuses, tID)
										}
										updateActiveTable()
										file.Servers = append(file.Servers[:srvIdx], file.Servers[srvIdx+1:]...)
										saveConfig(&file)
										populateServers()
									})
									for _, cancel := range cancels {
										cancel()
									}
								}()
							}
							pages.RemovePage("modal")
							app.SetFocus(list)
							updateFooter()
						}), true, true)
					return nil
				}
			} else if currentScope == "tunnel" {
				if event.Key() == tcell.KeyEsc || event.Key() == tcell.KeyLeft {
					populateServers()
					return nil
				}
				validItem := idx >= 0 && idx < len(file.Servers[selectedServer].Connections)
				if event.Rune() == 'a' {
					showTunnelForm(-1)
					return nil
				} else if validItem && event.Rune() == 'e' {
					showTunnelForm(idx)
					return nil
				} else if validItem && event.Rune() == 'd' {
					tunnelName := file.Servers[selectedServer].Connections[idx].Name
					pages.AddPage("modal", newConfirmModal(
						"Eliminar túnel",
						fmt.Sprintf("¿Está seguro que desea eliminar el túnel [%s]?", tunnelName),
						[]string{"Si", "No"},
						func(buttonIndex int, buttonLabel string) {
							if buttonLabel == "Si" {
								tID := fmt.Sprintf("%d-%d", selectedServer, idx)
								connIdx := idx
								if cancel, ok := activeTunnels[tID]; ok {
									go func(cancel context.CancelFunc, id string) {
										app.QueueUpdateDraw(func() {
											delete(activeTunnels, id)
											delete(activeStatuses, id)
											activeDetails.Delete(id)
											updateActiveTable()
											conns := file.Servers[selectedServer].Connections
											file.Servers[selectedServer].Connections = append(conns[:connIdx], conns[connIdx+1:]...)
											saveConfig(&file)
											populateTunnels(selectedServer)
										})
										cancel()
									}(cancel, tID)
								} else {
									updateActiveTable()
									conns := file.Servers[selectedServer].Connections
									file.Servers[selectedServer].Connections = append(conns[:connIdx], conns[connIdx+1:]...)
									saveConfig(&file)
									populateTunnels(selectedServer)
								}
							}
							pages.RemovePage("modal")
							app.SetFocus(list)
							updateFooter()
						}), true, true)
					return nil
				} else if event.Rune() == 't' || event.Rune() == 'T' {
					// Conectar todos los túneles
					srv := file.Servers[selectedServer]
					for i := range srv.Connections {
						startTunnel(selectedServer, i)
					}
					updateActiveTable()
					populateTunnels(selectedServer)
					slog.Info("Activando todos los túneles", "servidor", srv.Name)
					return nil
				}
			}
		}

		if event.Key() == tcell.KeyCtrlQ {
			pages.AddPage("modal", newConfirmModal(
				"Salir",
				"¿Está seguro que desea salir de Bifrost?",
				[]string{"Si", "No"},
				func(buttonIndex int, buttonLabel string) {
					if buttonLabel == "Si" {
						app.Stop()
					}
					pages.RemovePage("modal")
					app.SetFocus(list)
					updateFooter()
				}), true, true)
			return nil
		}
		return event
	})

	// Para salir amablemente, cancelamos todo al cerrar la UI.
	defer func() {
		for _, cancel := range activeTunnels {
			cancel()
		}
		time.Sleep(200 * time.Millisecond)
	}()

	slog.Info("Bienvenido a Bifrost")
	slog.Info("Modo multi-servidor habilitado.")

	if err := app.SetRoot(pages, true).EnableMouse(true).Run(); err != nil {
		return err
	}

	// En algunas terminales de macOS el cursor puede quedar oculto tras salir de la pantalla alternativa.
	fmt.Fprint(os.Stderr, "\033[?25h")

	return nil
}

func fallbackFile() clientConfigFile {
	return clientConfigFile{
		Servers: []serverConfig{},
	}
}
