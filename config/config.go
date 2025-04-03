package config

import (
	"time"

	"github.com/spf13/viper"
	"go.uber.org/zap"
)

const (
	DefaultPort          = "8080"
	DefaultCheckInterval = 5 * time.Second
)

var Config *viper.Viper

func InitConfig(logger *zap.Logger) {
	Config = viper.New()
	Config.SetDefault("port", DefaultPort)
	Config.SetDefault("check_interval", DefaultCheckInterval)
	Config.SetDefault("target_system_url", "")
	Config.SetConfigName("config")
	Config.SetConfigType("yaml")
	Config.AddConfigPath(".")
	err := Config.ReadInConfig()
	if err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			logger.Warn("Konfigurationsdatei nicht gefunden, verwende Standardwerte")
		} else {
			logger.Error("Fehler beim Lesen der Konfigurationsdatei:", zap.Error(err))
		}
	}
}
