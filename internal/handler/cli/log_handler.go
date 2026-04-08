package cli

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"

	"golang.org/x/term"
)

// ========== Progress mode constants ==========

const (
	ProgressAuto  = "auto"  // auto-detect TTY
	ProgressTTY   = "tty"   // TTY-style (overwrite lines)
	ProgressPlain = "plain" // standard line-by-line
	ProgressQuiet = "quiet" // no console output
)

// ========== Multi-handler for slog ==========

// multiHandler 将日志记录扇出到多个 slog.Handler
type multiHandler struct {
	handlers []slog.Handler
}

func (h *multiHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, hh := range h.handlers {
		if hh.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (h *multiHandler) Handle(ctx context.Context, r slog.Record) error {
	for _, hh := range h.handlers {
		if hh.Enabled(ctx, r.Level) {
			if err := hh.Handle(ctx, r); err != nil {
				return err
			}
		}
	}
	return nil
}

func (h *multiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	handlers := make([]slog.Handler, len(h.handlers))
	for i, hh := range h.handlers {
		handlers[i] = hh.WithAttrs(attrs)
	}
	return &multiHandler{handlers: handlers}
}

func (h *multiHandler) WithGroup(name string) slog.Handler {
	handlers := make([]slog.Handler, len(h.handlers))
	for i, hh := range h.handlers {
		handlers[i] = hh.WithGroup(name)
	}
	return &multiHandler{handlers: handlers}
}

// ========== TTY console handler ==========

// ttyHandler 在终端中原地刷新同一行输出
type ttyHandler struct {
	w       io.Writer
	opts    *slog.HandlerOptions
	lastLen int
}

func newTTYHandler(w io.Writer, opts *slog.HandlerOptions) *ttyHandler {
	if opts == nil {
		opts = &slog.HandlerOptions{}
	}
	return &ttyHandler{w: w, opts: opts}
}

func (h *ttyHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.opts.Level.Level()
}

func (h *ttyHandler) Handle(_ context.Context, r slog.Record) error {
	// 构建日志行（不含换行）
	var attrs strings.Builder
	r.Attrs(func(a slog.Attr) bool {
		attrs.WriteString(fmt.Sprintf(" %s=%s", a.Key, a.Value.String()))
		return true
	})

	icon := levelIcon(r.Level)
	line := fmt.Sprintf("%s %s%s", icon, r.Message, attrs.String())

	// 清除上一行并写入新内容
	if h.lastLen > 0 {
		fmt.Fprintf(h.w, "\r%s\r%s", strings.Repeat(" ", h.lastLen), line)
	} else {
		fmt.Fprintf(h.w, "\r%s", line)
	}
	h.lastLen = len(line)

	// Error/Warn 固定换行，Info 原地刷新
	if r.Level >= slog.LevelWarn {
		fmt.Fprintln(h.w)
		h.lastLen = 0
	}

	return nil
}

// Finalize 刷新最后一行（换行固定）
func (h *ttyHandler) Finalize() {
	if h.lastLen > 0 {
		fmt.Fprintln(h.w)
		h.lastLen = 0
	}
}

func (h *ttyHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return h // 简化实现
}

func (h *ttyHandler) WithGroup(name string) slog.Handler {
	return h
}

func levelIcon(level slog.Level) string {
	switch {
	case level >= slog.LevelError:
		return "✗"
	case level >= slog.LevelWarn:
		return "⚠"
	default:
		return time.Now().Format("15:04:05")
	}
}

// ========== setupConsoleLogger ==========

// setupConsoleLogger 根据 progress 模式设置控制台日志
func setupConsoleLogger(progress string) {
	fileHandler := slog.Default().Handler()

	// quiet 模式：只保留文件日志
	if progress == ProgressQuiet {
		return
	}

	// 确定实际模式
	mode := progress
	if mode == ProgressAuto {
		if term.IsTerminal(int(os.Stdout.Fd())) {
			mode = ProgressTTY
		} else {
			mode = ProgressPlain
		}
	}

	var consoleHandler slog.Handler
	switch mode {
	case ProgressTTY:
		consoleHandler = newTTYHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		})
	default: // plain
		consoleHandler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		})
	}

	slog.SetDefault(slog.New(&multiHandler{
		handlers: []slog.Handler{fileHandler, consoleHandler},
	}))
}

// finalizeTTYLogger 刷新 TTY handler 最后一行
func finalizeTTYLogger() {
	h := slog.Default().Handler()
	if m, ok := h.(*multiHandler); ok {
		for _, hh := range m.handlers {
			if tty, ok := hh.(*ttyHandler); ok {
				tty.Finalize()
			}
		}
	}
}
