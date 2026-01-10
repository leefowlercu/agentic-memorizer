package config

import "github.com/spf13/viper"

// Default configuration values.
const (
	LogLevel = "info"
	LogFile  = "~/.config/memorizer/memorizer.log"

	// Daemon configuration defaults.
	DaemonHTTPPort        = 7600
	DaemonHTTPBind        = "127.0.0.1"
	DaemonShutdownTimeout = 30 // seconds
	DaemonPIDFile         = "~/.config/memorizer/daemon.pid"
)

// setDefaults registers all default configuration values with viper.
// Called during Init() before reading config files.
func setDefaults() {
	viper.SetDefault("log_level", LogLevel)
	viper.SetDefault("log_file", LogFile)

	// Daemon defaults
	viper.SetDefault("daemon.http_port", DaemonHTTPPort)
	viper.SetDefault("daemon.http_bind", DaemonHTTPBind)
	viper.SetDefault("daemon.shutdown_timeout", DaemonShutdownTimeout)
	viper.SetDefault("daemon.pid_file", DaemonPIDFile)
}
