package data

import (
	"html/template"

	"djp.chapter42.de/a/internal/auth"
)

type WavelyConfig struct {
	Port    string        `mapstructure:"port"`
	Debug   bool          `mapstructure:"debug"`
	Current CurrentConfig `mapstructure:"current"`
}

type CurrentConfig struct {
	Name        string          `mapstructure:"name"`
	BaseURL     string          `mapstructure:"base_url"`
	Endpoints   EndpointConfig  `mapstructure:"endpoints"`
	ContentType string          `mapstructure:"content_type"`
	Auth        auth.AuthConfig `mapstructure:"auth"`
	Repititions int             `mapstructure:"repetitions"`
	MinWorkers  int             `mapstructure:"min_workers"`
	MaxWorkers  int             `mapstructure:"min_workers"`

	// Caching vorbereiteter Templates
	ParsedCheckTpl    *template.Template
	ParsedRevisionTpl *template.Template
	ParsedWriteTpl    *template.Template

	// Authentication provider
	AuthProvider auth.AuthProvider
}

type EndpointConfig struct {
	Check    string `mapstructure:"check"`
	Revision string `mapstructure:"revision"`
	Write    string `mapstructure:"write"`
}
