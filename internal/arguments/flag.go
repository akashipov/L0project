package arguments

import (
	"flag"
	"fmt"

	"github.com/caarlos0/env/v6"
)

var PostgresPWD string
var NatsURL string
var HPServer string
var CacheSize int
var CacheTimeLimitSecs int

type ServerEnvConfig struct {
	PostgresPWD        string `env:"POSTGRES_PWD"`
	NatsURL            string `env:"NATS_URL"`
	HPServer           string `env:"HTTP_URL"`
	CacheSize          int    `env:"CACHE_SIZE"`
	CacheTimeLimitSecs int    `env:"CACHE_LIMIT_SECS"`
}

func ParseArgsServer() error {
	var cfg ServerEnvConfig
	err := env.Parse(&cfg)
	if err != nil {
		return fmt.Errorf("Problem with parsing of env variables: %w", err)
	}
	p := flag.String("p", "", "password of postgres db")
	n := flag.String("n", "0.0.0.0:4222", "Nats <host>:<port> to connect")
	cs := flag.Int("cs", 5, "Cache max capacity")
	ctl := flag.Int("ctl", 5, "Cache time limit on value in the table")
	s := flag.String("s", "0.0.0.0:8000", "Nats <host>:<port> to connect")
	flag.Parse()
	if p != nil {
		PostgresPWD = *p
	}
	if cs != nil {
		CacheSize = *cs
	}
	if ctl != nil {
		CacheTimeLimitSecs = *ctl
	}
	if n != nil {
		NatsURL = *n
	}
	if s != nil {
		HPServer = *s
	}
	if cfg.HPServer != "" {
		HPServer = cfg.HPServer
	}
	if cfg.CacheSize != 0 {
		CacheSize = cfg.CacheSize
	}
	if cfg.CacheTimeLimitSecs != 0 {
		CacheTimeLimitSecs = cfg.CacheTimeLimitSecs
	}
	if cfg.PostgresPWD != "" {
		PostgresPWD = cfg.PostgresPWD
	}
	if cfg.NatsURL != "" {
		NatsURL = cfg.NatsURL
	}
	fmt.Println("Http host:", HPServer)
	fmt.Println("Nats host:", NatsURL)
	fmt.Printf("Cache max size: %d\n", CacheSize)
	fmt.Printf("Cache limit on time in seconds: %d\n", CacheTimeLimitSecs)
	return nil
}
