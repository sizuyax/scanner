package config

import (
	"github.com/caarlos0/env"
	"github.com/joho/godotenv"
)

type Config struct {
	Environment   string `env:"ENVIRONMENT" envDefault:"release"`
	Port          int    `env:"PORT" envDefault:"50051"`
	AdminPassword string `env:"ADMIN_PASSWORD,required"`
	UserPassword  string `env:"USER_PASSWORD,required"`
}

func MustLoad() Config {
	cfg := Config{}

	err := godotenv.Load(".env")
	if err != nil {
		panic(err)
	}

	if err := env.Parse(&cfg); err != nil {
		panic(err)
	}

	return cfg
}
