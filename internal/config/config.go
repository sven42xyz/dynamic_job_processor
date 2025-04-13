package config

import (
	"log"

	"djp.chapter42.de/a/internal/auth"
	"djp.chapter42.de/a/internal/data"
	"djp.chapter42.de/a/internal/tmpl"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

const (
	DefaultPort string = "4224"
)

var Config *data.WavelyConfig

func InitConfig(logger *zap.Logger) {
	v := viper.New()
	v.SetDefault("port", DefaultPort)
	v.SetConfigName("wavely.cfg")
	v.SetConfigType("yaml")
	// v.AddConfigPath("./config")
	v.AddConfigPath("/app/config")
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

	if err := tmpl.PrepareTemplates(Config); err != nil {
		logger.Error("Fehler beim Parsen der Templates:", zap.Error(err))
	}

	auth, err := auth.BuildAuthProvider(Config.Current.Auth)
	if err != nil {
		log.Fatalf("Fehler beim Erzeugen des AuthProviders für %s: %v", Config.Current.Name, err)
	}
	Config.Current.AuthProvider = auth
}
