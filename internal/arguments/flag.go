package arguments

import (
	"flag"
	"fmt"

	"github.com/caarlos0/env/v6"
)

var PostgresPWD string
var NatsURL string

type ServerEnvConfig struct {
	PostgresPWD string `env:"POSTGRES_PWD"`
	NatsURL     string `env:"NATS_URL"`
}

func ParseArgsServer() error {
	var cfg ServerEnvConfig
	err := env.Parse(&cfg)
	if err != nil {
		return fmt.Errorf("Problem with parsing of env variables: %w", err)
	}
	p := flag.String("p", "", "password of postgres db")
	n := flag.String("n", "0.0.0.0:4222", "Nats <host>:<port> to connect")
	flag.Parse()
	if p != nil {
		PostgresPWD = *p
	}
	if n != nil {
		NatsURL = *n
	}
	if cfg.PostgresPWD != "" {
		PostgresPWD = cfg.PostgresPWD
	}
	if cfg.NatsURL != "" {
		NatsURL = cfg.NatsURL
	}
	return nil
}
