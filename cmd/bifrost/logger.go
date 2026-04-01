package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
)

type customHandler struct {
	mu      *sync.Mutex
	out     io.Writer
	opts    slog.HandlerOptions
	isTview bool
}

func newCustomHandler(out io.Writer, opts slog.HandlerOptions, isTview bool) *customHandler {
	return &customHandler{
		mu:      &sync.Mutex{},
		out:     out,
		opts:    opts,
		isTview: isTview,
	}
}

func (h *customHandler) Enabled(ctx context.Context, level slog.Level) bool {
	minLevel := slog.LevelInfo
	if h.opts.Level != nil {
		minLevel = h.opts.Level.Level()
	}
	return level >= minLevel
}

func (h *customHandler) Handle(ctx context.Context, r slog.Record) error {
	levelStr := "INFO"
	switch r.Level {
	case slog.LevelDebug:
		levelStr = "DBUG"
	case slog.LevelInfo:
		levelStr = "INFO"
	case slog.LevelWarn:
		levelStr = "WARN"
	case slog.LevelError:
		levelStr = "FAIL"
	}

	msgLower := strings.ToLower(r.Message)
	if r.Level == slog.LevelInfo && (strings.Contains(msgLower, "exitosamente") || strings.Contains(msgLower, "ok")) {
		levelStr = " OK "
	}

	// Formato DD-MM-YYYY HH:MM:SS
	timeStr := r.Time.Format("02-01-2006 15:04:05")

	h.mu.Lock()
	defer h.mu.Unlock()

	if h.isTview {
		// En Tview, damos color al nivel y escapamos los corchetes para que no desaparezcan.
		color := "white"
		switch r.Level {
		case slog.LevelWarn:
			color = "yellow"
		case slog.LevelError:
			color = "red"
		}
		if levelStr == " OK " {
			color = "green"
		}
		// Formato: [[ [COLOR]LEVEL[-] ]
		fmt.Fprintf(h.out, "%s [[%s]%s[-]] %s", timeStr, color, levelStr, r.Message)
	} else {
		// Consola normal
		fmt.Fprintf(h.out, "%s [%s] %s", timeStr, levelStr, r.Message)
	}

	r.Attrs(func(a slog.Attr) bool {
		fmt.Fprintf(h.out, " %s=%v", a.Key, a.Value.Any())
		return true
	})

	fmt.Fprint(h.out, "\n")
	return nil
}

func (h *customHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return h
}

func (h *customHandler) WithGroup(name string) slog.Handler {
	return h
}
