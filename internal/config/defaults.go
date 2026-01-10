package config

import "github.com/spf13/viper"

// Default configuration values.
const (
	LogLevel = "info"
	LogFile  = "~/.config/memorizer/memorizer.log"
)

// setDefaults registers all default configuration values with viper.
// Called during Init() before reading config files.
func setDefaults() {
	viper.SetDefault("log_level", LogLevel)
	viper.SetDefault("log_file", LogFile)
}
