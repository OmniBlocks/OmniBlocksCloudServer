package server

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

// LogCategory identifies a distinct part of the system that can be logged.
// Each category can be enabled or disabled independently via LoggingConfig.
type LogCategory string

const (
	LogServer     LogCategory = "server"     // process startup/shutdown
	LogConfig     LogCategory = "config"     // config loading
	LogConnection LogCategory = "connection" // websocket upgrade/accept/close
	LogHandshake  LogCategory = "handshake"  // handshake accept/reject
	LogMessages   LogCategory = "messages"   // inbound message dispatch
	LogVariables  LogCategory = "variables"  // cloud variable set/rename/delete
	LogRooms      LogCategory = "rooms"      // room lifecycle
	LogBroadcast  LogCategory = "broadcast"  // fan-out to room clients
	LogErrors     LogCategory = "errors"     // protocol errors / forced closes
)

const colorReset = "\x1b[0m"

var categoryColors = map[LogCategory]string{
	LogServer:     "\x1b[34m",   // blue
	LogConfig:     "\x1b[36m",   // cyan
	LogConnection: "\x1b[94m",   // bright blue
	LogHandshake:  "\x1b[35m",   // magenta
	LogMessages:   "\x1b[37m",   // white
	LogVariables:  "\x1b[32m",   // green
	LogRooms:      "\x1b[33m",   // yellow
	LogBroadcast:  "\x1b[96m",   // bright cyan
	LogErrors:     "\x1b[31;1m", // bold red
}

// Logger writes categorized diagnostic messages to the console and/or logfile
type Logger struct {
	mu      sync.Mutex
	enabled map[LogCategory]bool
	color   bool
	console io.Writer
	file    io.WriteCloser
}

// NewLogger builds a Logger from a LoggingConfig. When cfg.File.Enabled, log
// lines are also appended to a rotating, gzip-compressed file under
// cfg.File.Directory.
func NewLogger(cfg LoggingConfig) (*Logger, error) {
	l := &Logger{
		enabled: map[LogCategory]bool{
			LogServer:     cfg.Categories.Server,
			LogConfig:     cfg.Categories.Config,
			LogConnection: cfg.Categories.Connection,
			LogHandshake:  cfg.Categories.Handshake,
			LogMessages:   cfg.Categories.Messages,
			LogVariables:  cfg.Categories.Variables,
			LogRooms:      cfg.Categories.Rooms,
			LogBroadcast:  cfg.Categories.Broadcast,
			LogErrors:     cfg.Categories.Errors,
		},
		color: cfg.Color,
	}

	if !cfg.Enabled {
		return l, nil
	}
	l.console = os.Stdout

	if cfg.File.Enabled {
		rf, err := newRotatingFile(cfg.File.Directory, cfg.File.MaxSizeMB, cfg.File.MaxBackups)
		if err != nil {
			return nil, fmt.Errorf("initialize log file: %w", err)
		}
		l.file = rf
	}

	return l, nil
}

// NewDiscardLogger returns a Logger with every category disabled. Handy for
// tests and other contexts that don't want diagnostic output.
func NewDiscardLogger() *Logger {
	return &Logger{enabled: map[LogCategory]bool{}}
}

// Close releases the underlying log file, if any.
func (l *Logger) Close() error {
	if l == nil || l.file == nil {
		return nil
	}
	return l.file.Close()
}

func (l *Logger) log(cat LogCategory, format string, args ...any) {
	if l == nil || !l.enabled[cat] {
		return
	}

	ts := time.Now().Format("2006-01-02 15:04:05.000")
	line := fmt.Sprintf("[%s] [%s] %s", ts, cat, fmt.Sprintf(format, args...))

	l.mu.Lock()
	defer l.mu.Unlock()

	if l.console != nil {
		if l.color {
			fmt.Fprintf(l.console, "%s%s%s\n", categoryColors[cat], line, colorReset)
		} else {
			fmt.Fprintln(l.console, line)
		}
	}
	if l.file != nil {
		fmt.Fprintln(l.file, line)
	}
}

func (l *Logger) Server(format string, args ...any)     { l.log(LogServer, format, args...) }
func (l *Logger) Config(format string, args ...any)     { l.log(LogConfig, format, args...) }
func (l *Logger) Connection(format string, args ...any) { l.log(LogConnection, format, args...) }
func (l *Logger) Handshake(format string, args ...any)  { l.log(LogHandshake, format, args...) }
func (l *Logger) Messages(format string, args ...any)   { l.log(LogMessages, format, args...) }
func (l *Logger) Variables(format string, args ...any)  { l.log(LogVariables, format, args...) }
func (l *Logger) Rooms(format string, args ...any)      { l.log(LogRooms, format, args...) }
func (l *Logger) Broadcast(format string, args ...any)  { l.log(LogBroadcast, format, args...) }
func (l *Logger) Errors(format string, args ...any)     { l.log(LogErrors, format, args...) }
