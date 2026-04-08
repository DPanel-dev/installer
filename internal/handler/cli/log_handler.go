package cli

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"
)

// ========== Progress mode constants ==========

const (
	ProgressPlain = "plain" // 标准文本输出 (default)
	ProgressQuiet = "quiet" // no console output
)

// ========== Multi-handler for slog ==========

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

// ========== Console handler (slog text + 本地时间) ==========

type consoleHandler struct {
	w    io.Writer
	opts *slog.HandlerOptions
}

func newConsoleHandler(w io.Writer, opts *slog.HandlerOptions) *consoleHandler {
	if opts == nil {
		opts = &slog.HandlerOptions{}
	}
	return &consoleHandler{w: w, opts: opts}
}

func (h *consoleHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.opts.Level.Level()
}

func (h *consoleHandler) Handle(_ context.Context, r slog.Record) error {
	ts := time.Now().Format("15:04:05")
	level := strings.TrimSpace(r.Level.String())

	var attrs strings.Builder
	r.Attrs(func(a slog.Attr) bool {
		if attrs.Len() > 0 {
			attrs.WriteString(" ")
		}
		attrs.WriteString(fmt.Sprintf("%s=%s", a.Key, a.Value))
		return true
	})

	if attrs.Len() > 0 {
		fmt.Fprintf(h.w, "%s %s %s %s\n", ts, level, r.Message, attrs.String())
	} else {
		fmt.Fprintf(h.w, "%s %s %s\n", ts, level, r.Message)
	}
	return nil
}

func (h *consoleHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return h
}

func (h *consoleHandler) WithGroup(name string) slog.Handler {
	return h
}

// ========== setupConsoleLogger ==========

func setupConsoleLogger(progress string) {
	fileHandler := slog.Default().Handler()

	if progress == ProgressQuiet {
		return
	}

	consoleHandler := newConsoleHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	slog.SetDefault(slog.New(&multiHandler{
		handlers: []slog.Handler{fileHandler, consoleHandler},
	}))
}

// ========== Progress ==========

// ShowProgress 在终端原地刷新显示下载进度
func ShowProgress(complete, total int64) {
	if complete <= 0 {
		return
	}
	// 用足够宽度覆盖残留字符
	fmt.Fprintf(os.Stdout, "\r%s INFO Pull %-15s",
		time.Now().Format("15:04:05"),
		formatBytes(complete),
	)
}

// FinishProgress 固定进度行（换行）
func FinishProgress() {
	fmt.Fprintln(os.Stdout)
}

func formatBytes(b int64) string {
	const mb = 1024 * 1024
	if b >= mb {
		return fmt.Sprintf("%.1fMB", float64(b)/float64(mb))
	}
	const kb = 1024
	if b >= kb {
		return fmt.Sprintf("%.1fKB", float64(b)/float64(kb))
	}
	return fmt.Sprintf("%dB", b)
}
