package config

import (
	"path/filepath"

	"github.com/spf13/viper"
)

type Config struct {
	RPCUrl       string
	DbUser       string
	DbPassword   string
	DbHost       string
	DbPort       string
	DbName       string
	WorkersCount int
	Step         int
}

func LoadConfig() (Config, error) {
	configFile, err := filepath.Abs(".env")

	if err != nil {
		return Config{}, err
	}

	err = read(configFile)

	if err != nil {
		return Config{}, err
	}

	config := Config{
		RPCUrl:       viper.GetString("RPCUrl"),
		DbUser:       viper.GetString("DB_USER"),
		DbPassword:   viper.GetString("DB_PASSWORD"),
		DbHost:       viper.GetString("DB_HOST"),
		DbPort:       viper.GetString("DB_PORT"),
		DbName:       viper.GetString("DB_NAME"),
		WorkersCount: viper.GetInt("WORKERS_COUNT"),
		Step:         viper.GetInt("STEP"),
	}

	config.fillDefaults()

	return config, nil
}

// Read - Reading .env file content, during application start up
func read(file string) error {
	viper.SetConfigFile(file)
	return viper.ReadInConfig()
}

func (cfg *Config) fillDefaults() {
	if cfg.Step == 0 {
		cfg.Step = 1000
	}

	if cfg.WorkersCount == 0 {
		cfg.WorkersCount = 32
	}
}
