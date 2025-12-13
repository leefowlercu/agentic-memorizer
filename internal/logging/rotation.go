package logging

import "gopkg.in/natefinch/lumberjack.v2"

const (
	// DefaultMaxSize is the default maximum size in megabytes before rotation
	DefaultMaxSize = 10

	// DefaultMaxBackups is the default number of old log files to retain
	DefaultMaxBackups = 3

	// DefaultMaxAge is the default maximum days to retain old log files
	DefaultMaxAge = 28

	// DefaultCompress enables compression of rotated files
	DefaultCompress = true
)

// RotationConfig holds lumberjack rotation configuration
type RotationConfig struct {
	MaxSize    int  // megabytes
	MaxBackups int  // number of backups
	MaxAge     int  // days
	Compress   bool // compress rotated files
}

// DefaultRotationConfig returns default rotation settings
func DefaultRotationConfig() RotationConfig {
	return RotationConfig{
		MaxSize:    DefaultMaxSize,
		MaxBackups: DefaultMaxBackups,
		MaxAge:     DefaultMaxAge,
		Compress:   DefaultCompress,
	}
}

// NewRotatingWriter creates a lumberjack.Logger with default rotation settings
func NewRotatingWriter(filename string) *lumberjack.Logger {
	cfg := DefaultRotationConfig()
	return &lumberjack.Logger{
		Filename:   filename,
		MaxSize:    cfg.MaxSize,
		MaxBackups: cfg.MaxBackups,
		MaxAge:     cfg.MaxAge,
		Compress:   cfg.Compress,
	}
}

// NewRotatingWriterWithConfig creates a lumberjack.Logger with custom rotation settings
func NewRotatingWriterWithConfig(filename string, cfg RotationConfig) *lumberjack.Logger {
	return &lumberjack.Logger{
		Filename:   filename,
		MaxSize:    cfg.MaxSize,
		MaxBackups: cfg.MaxBackups,
		MaxAge:     cfg.MaxAge,
		Compress:   cfg.Compress,
	}
}
