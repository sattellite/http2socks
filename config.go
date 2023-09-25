package main

import (
	"fmt"
	"net/netip"

	"github.com/cristalhq/aconfig"
)

type Config struct {
	HTTPAddress        string `default:":8080" usage:"address to listen on"`
	SocksProxy         string `usage:"SOCKS5 proxy to use"`
	SocksProxyUser     string `usage:"SOCKS5 proxy user"`
	SocksProxyPassword string `usage:"SOCKS5 proxy password"`
}

func loadConfig() (*Config, error) {
	cfg := Config{}
	err := aconfig.LoaderFor(&cfg, aconfig.Config{
		SkipFiles:    true,
		SkipDefaults: false,
		SkipEnv:      false,
		SkipFlags:    false,
	}).Load()
	if err != nil {
		return nil, err
	}

	_, httpError := netip.ParseAddrPort(cfg.HTTPAddress)
	if httpError != nil {
		return nil, fmt.Errorf("HTTP address must be a valid IP address and port: %w", httpError)
	}

	if cfg.SocksProxy == "" {
		return nil, fmt.Errorf("SOCKS5 proxy must be set")
	}

	if cfg.SocksProxy != "" {
		if cfg.SocksProxyUser == "" {
			return nil, fmt.Errorf("SOCKS5 proxy user must be set when SOCKS5 proxy is set")
		}
		if cfg.SocksProxyPassword == "" {
			return nil, fmt.Errorf("SOCKS5 proxy password must be set when SOCKS5 proxy is set")
		}
	}
	return &cfg, nil
}
