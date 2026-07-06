package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config is the top-level Yoshi Pi-hole configuration, loaded from config.yaml.
type Config struct {
	DNS      DNSConfig      `yaml:"dns"`
	Web      WebConfig      `yaml:"web"`
	Storage  StorageConfig  `yaml:"storage"`
	Gravity  GravityConfig  `yaml:"gravity"`
	Blocking BlockingConfig `yaml:"blocking"`
}

type DNSConfig struct {
	// Listen is host:port the DNS server binds to, e.g. 127.0.0.1:53 (prod) or 127.0.0.1:5353 (dev).
	Listen string `yaml:"listen"`
	// Upstreams are plain DNS servers (host:port) queried for non-blocked domains, tried in order.
	Upstreams []string `yaml:"upstreams"`
	// UpstreamTimeoutMS is the per-upstream query timeout in milliseconds.
	UpstreamTimeoutMS int `yaml:"upstream_timeout_ms"`
}

type WebConfig struct {
	// Listen is host:port the dashboard/API HTTP server binds to.
	Listen string `yaml:"listen"`
}

type StorageConfig struct {
	// DataDir holds gravity.db and queries.db.
	DataDir string `yaml:"data_dir"`
}

type GravityConfig struct {
	// DefaultAdlists are blocklist URLs seeded on first run.
	DefaultAdlists []string `yaml:"default_adlists"`
	// UpdateIntervalHours controls how often gravity auto-refreshes (0 disables auto-refresh).
	UpdateIntervalHours int `yaml:"update_interval_hours"`
}

type BlockingConfig struct {
	// Mode is the default reply for blocked domains: "nxdomain" or "null" (0.0.0.0/::).
	Mode string `yaml:"mode"`
}

func Default() Config {
	return Config{
		DNS: DNSConfig{
			Listen:            "127.0.0.1:53",
			Upstreams:         []string{"1.1.1.1:53", "9.9.9.9:53"},
			UpstreamTimeoutMS: 2000,
		},
		Web: WebConfig{
			Listen: "127.0.0.1:8080",
		},
		Storage: StorageConfig{
			DataDir: "./data",
		},
		Gravity: GravityConfig{
			DefaultAdlists: []string{
				"https://raw.githubusercontent.com/StevenBlack/hosts/master/hosts",
			},
			UpdateIntervalHours: 24,
		},
		Blocking: BlockingConfig{
			Mode: "null",
		},
	}
}

// Load reads a YAML config file, filling in defaults for any unset fields.
// If path does not exist, the defaults are returned unmodified.
func Load(path string) (Config, error) {
	cfg := Default()

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return cfg, nil
	}
	if err != nil {
		return cfg, fmt.Errorf("reading config %s: %w", path, err)
	}

	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("parsing config %s: %w", path, err)
	}

	if cfg.Blocking.Mode != "nxdomain" && cfg.Blocking.Mode != "null" {
		return cfg, fmt.Errorf("invalid blocking.mode %q: must be \"nxdomain\" or \"null\"", cfg.Blocking.Mode)
	}
	if len(cfg.DNS.Upstreams) == 0 {
		return cfg, fmt.Errorf("dns.upstreams must contain at least one server")
	}

	return cfg, nil
}
