package config

import (
	"djp.chapter42.de/a/data"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

const (
	DefaultPort string = "8080"
)

var Config *data.WavelyConfig

func InitConfig(logger *zap.Logger) {
	v := viper.New()
	v.SetDefault("port", DefaultPort)
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath(".")
	err := v.ReadInConfig()
	if err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			logger.Warn("Konfigurationsdatei nicht gefunden, verwende Standardwerte")
		} else {
			logger.Error("Fehler beim Lesen der Konfigurationsdatei:", zap.Error(err))
		}
	}

	if err := v.Unmarshal(&Config); err != nil {
		logger.Error("Fehler beim Lesen der Konfigurationsdatei:", zap.Error(err))
	}
}
