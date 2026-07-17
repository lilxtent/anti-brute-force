package config

import (
	"fmt"
	"time"

	"github.com/go-playground/validator"
	"github.com/spf13/viper"
)

type Config struct {
	Server   ServerConfig   `mapstructure:"server" validate:"required"`
	Limits   LimitsConfig   `mapstructure:"limits" validate:"required"`
	Redis    RedisConfig    `mapstructure:"redis" validate:"required"`
	Postgres PostgresConfig `mapstructure:"postgres" validate:"required"`
}

type ServerConfig struct {
	HttpPort int `mapstructure:"http_port" validate:"required"`
}

type LimitsConfig struct {
	LoginAttempts    int           `mapstructure:"login_attempts" validate:"required"`
	PasswordAttempts int           `mapstructure:"password_attempts" validate:"required"`
	IPAttempts       int           `mapstructure:"ip_attempts" validate:"required"`
	WindowTime       time.Duration `mapstructure:"window_time" validate:"required"`
}

type RedisConfig struct {
	Address  string `mapstructure:"address" validate:"required"`
	Password string `mapstructure:"password" validate:"required"`
}

type PostgresConfig struct {
	Dsn string `mapstructure:"dsn" validate:"required"`
}

func Load(path string) (*Config, error) {
	v := viper.New()
	v.SetConfigFile(path)

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("read config error %q: %v", path, err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config error: %v", err)
	}

	validator := validator.New()
	if err := validator.Struct(cfg); err != nil {
		return nil, fmt.Errorf("missing required fileds in config: %v", err)
	}

	return &cfg, nil
}
