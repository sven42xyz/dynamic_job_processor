package data

type WavelyConfig struct {
	Port            string          `mapstructure:"port"`
	TargetSystemURL string          `mapstructure:"target_system_url"`
	Debug           bool            `mapstructure:"debug"`
	Currents        []CurrentConfig `mapstructure:"apis"`
}

type CurrentConfig struct {
	Name            string         `mapstructure:"name"`
	BaseURL         string         `mapstructure:"base_url"`
	Endpoints       EndpointConfig `mapstructure:"endpoints"`
	PayloadTemplate string         `mapstructure:"payload_template"`
	Auth            AuthConfig     `mapstructure:"auth"`
	Repititions     int            `mapstructure:"repetitions"`
	MinWorkers      int            `mapstructure:"min_workers"`
	MaxWorkers      int            `mapstructure:"min_workers"`
}

type EndpointConfig struct {
	CheckWritable string `mapstructure:"check_writable"`
	Write         string `mapstructure:"write"`
}

type AuthConfig struct {
	Type         string `mapstructure:"type"` // basic, bearer, oauth2
	Username     string `mapstructure:"username,omitempty"`
	Password     string `mapstructure:"password,omitempty"`
	Token        string `mapstructure:"token,omitempty"`
	ClientID     string `mapstructure:"client_id,omitempty"`
	ClientSecret string `mapstructure:"client_secret,omitempty"`
	TokenURL     string `mapstructure:"token_url,omitempty"`
	RefreshToken string `mapstructure:"refresh_token,omitempty"`
}