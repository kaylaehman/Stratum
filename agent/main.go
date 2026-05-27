// Command stratum-agent is the per-host Stratum agent. In SP0 it is a
// compiling placeholder: it loads its config file, logs what it found, and
// exits cleanly. Real capabilities (file ops, permissions, cron, inotify watch,
// SSH-key audit) land in later sub-projects over gRPC + mTLS.
package main

import (
	"flag"
	"log/slog"
	"os"

	"gopkg.in/yaml.v3"
)

// Config mirrors /etc/stratum-agent/config.yaml.
type Config struct {
	ServerURL  string   `yaml:"server_url"`
	Token      string   `yaml:"token"`
	WatchPaths []string `yaml:"watch_paths"`
}

func main() {
	configPath := flag.String("config", "/etc/stratum-agent/config.yaml", "path to agent config file")
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	cfg, err := loadConfig(*configPath)
	if err != nil {
		logger.Warn("no usable config; agent is a placeholder in SP0", "path", *configPath, "error", err)
		return
	}

	logger.Info("stratum-agent placeholder started",
		"server_url", cfg.ServerURL,
		"watch_paths", cfg.WatchPaths,
		"token_present", cfg.Token != "",
	)
	logger.Info("SP0 placeholder: no capabilities implemented yet; exiting cleanly")
}

func loadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
