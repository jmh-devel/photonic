package logging

import (
	"context"
	"fmt"
	"io"
	"log"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"photonic/internal/config"
)

// New returns a slog.Logger with the provided level string (info, debug, warn, error).
// format may be "json" or "text".
func New(level string, format string) *slog.Logger {
	opts := &slog.HandlerOptions{Level: parseLevel(level)}
	var handler slog.Handler
	if strings.ToLower(format) == "json" {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}
	return slog.New(handler)
}

// Setup configures global logging with file output and rotation
func Setup(cfg *config.Config) (*slog.Logger, error) {
	// Parse log level
	level := parseLevel(cfg.Logging.Level)

	// Create log directory
	if cfg.Logging.FileOutput {
		if err := os.MkdirAll(cfg.Logging.LogDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create log directory: %v", err)
		}
	}

	// Configure output writers
	var writers []io.Writer

	// Always include stdout for immediate feedback
	writers = append(writers, os.Stdout)

	// Add file output if enabled
	if cfg.Logging.FileOutput {
		logFile := filepath.Join(cfg.Logging.LogDir, fmt.Sprintf("photonic-%s.log",
			time.Now().Format("2006-01-02")))

		file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return nil, fmt.Errorf("failed to open log file: %v", err)
		}

		writers = append(writers, file)

		// Create a symlink for the current log
		currentLogPath := filepath.Join(cfg.Logging.LogDir, "photonic-current.log")
		os.Remove(currentLogPath) // Remove existing symlink
		if err := os.Symlink(filepath.Base(logFile), currentLogPath); err != nil {
			// Symlink failed, but continue - it's not critical
		}
	}

	// Combine all writers
	multiWriter := io.MultiWriter(writers...)

	// Create a standard logger that uses traditional format
	logger := log.New(multiWriter, "", log.LstdFlags)

	// Create a wrapper that implements slog.Handler interface but uses traditional format
	handler := &TraditionalHandler{
		logger: logger,
		level:  level,
	}

	slogLogger := slog.New(handler)

	// Set as default logger
	slog.SetDefault(slogLogger)

	// Log startup information
	slogLogger.Info("photonic logging initialized",
		"level", cfg.Logging.Level,
		"format", cfg.Logging.Format,
		"file_output", cfg.Logging.FileOutput,
		"log_dir", cfg.Logging.LogDir,
	)

	return slogLogger, nil
}

// TraditionalHandler implements slog.Handler with traditional log formatting
type TraditionalHandler struct {
	logger *log.Logger
	level  slog.Level
}

func (h *TraditionalHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return level >= h.level
}

func (h *TraditionalHandler) Handle(ctx context.Context, r slog.Record) error {
	level := r.Level.String()

	// Build message with attributes
	msg := r.Message
	attrs := make([]string, 0)

	r.Attrs(func(a slog.Attr) bool {
		attrs = append(attrs, fmt.Sprintf("%s=%v", a.Key, a.Value))
		return true
	})

	if len(attrs) > 0 {
		msg = fmt.Sprintf("%s [%s]", msg, strings.Join(attrs, " "))
	}

	// Use traditional format: [LEVEL] message
	h.logger.Printf("[%s] %s", strings.ToUpper(level), msg)

	return nil
}

func (h *TraditionalHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	// For simplicity, return the same handler
	return h
}

func (h *TraditionalHandler) WithGroup(name string) slog.Handler {
	// For simplicity, return the same handler
	return h
}

func parseLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// LogJobStart logs the beginning of a processing job
func LogJobStart(logger *slog.Logger, jobType, jobID, inputPath, outputPath string, options map[string]any) {
	logger.Info("job started",
		"type", jobType,
		"id", jobID,
		"input", inputPath,
		"output", outputPath,
		"options", options,
	)
}

// LogJobComplete logs successful job completion
func LogJobComplete(logger *slog.Logger, jobType, jobID string, duration time.Duration, resultInfo map[string]any) {
	logger.Info("job completed successfully",
		"type", jobType,
		"id", jobID,
		"duration_ms", duration.Milliseconds(),
		"duration_human", duration.String(),
		"result", resultInfo,
	)
}

// LogJobError logs job failures
func LogJobError(logger *slog.Logger, jobType, jobID string, duration time.Duration, err error, context map[string]any) {
	logger.Error("job failed",
		"type", jobType,
		"id", jobID,
		"duration_ms", duration.Milliseconds(),
		"error", err.Error(),
		"context", context,
	)
}

// LogToolStatus logs tool detection and status
func LogToolStatus(logger *slog.Logger, tool string, available bool, version, path string, err error) {
	if available {
		logger.Debug("tool detected",
			"tool", tool,
			"version", version,
			"path", path,
		)
	} else {
		logger.Debug("tool not available",
			"tool", tool,
			"error", err,
		)
	}
}

// LogProcessingStep logs individual processing steps within a job
func LogProcessingStep(logger *slog.Logger, jobID, step, status string, details map[string]any) {
	logger.Info("processing step",
		"job_id", jobID,
		"step", step,
		"status", status,
		"details", details,
	)
}
