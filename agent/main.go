// Command stratum-agent is the per-host Stratum agent. It loads its config
// file, starts a mTLS-secured gRPC server, and serves WatchFiles and
// DetectInit RPCs. If TLS cert files are absent the agent refuses to start in
// insecure mode — the channel must be encrypted and mutually authenticated.
//
// TODO(install-script): the install script must issue a node-specific
// cert/key pair signed by the backend CA before starting the agent. Until
// that provisioning step is implemented, set AGENT_CERT, AGENT_KEY, and
// AGENT_CA env vars (or populate the config file) to point to valid files.
package main

import (
	"flag"
	"fmt"
	"log/slog"
	"net"
	"os"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"gopkg.in/yaml.v3"

	"github.com/kaylaehman/stratum/agent/agentsrv"
)

// Config mirrors /etc/stratum-agent/config.yaml.
type Config struct {
	ServerURL  string   `yaml:"server_url"`
	Token      string   `yaml:"token"`
	WatchPaths []string `yaml:"watch_paths"`
	ListenAddr string   `yaml:"listen_addr"`
	TLS        tlsCfg   `yaml:"tls"`
}

type tlsCfg struct {
	CertFile string `yaml:"cert_file"`
	KeyFile  string `yaml:"key_file"`
	CAFile   string `yaml:"ca_file"`
}

func main() {
	configPath := flag.String("config", "/etc/stratum-agent/config.yaml", "path to agent config file")
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	if err := run(*configPath, logger); err != nil {
		logger.Error("agent: fatal", "error", err)
		os.Exit(1)
	}
}

func run(configPath string, logger *slog.Logger) error {
	cfg, err := loadConfig(configPath)
	if err != nil {
		logger.Warn("no usable config; agent cannot start without configuration",
			"path", configPath, "error", err)
		return fmt.Errorf("agent: load config: %w", err)
	}

	logger.Info("stratum-agent starting",
		"server_url", cfg.ServerURL,
		"listen_addr", cfg.ListenAddr,
		"watch_paths", cfg.WatchPaths,
		"token_present", cfg.Token != "",
		"tls_cert", cfg.TLS.CertFile,
	)

	addr := cfg.ListenAddr
	if addr == "" {
		addr = ":7750"
	}

	tlsCredential, tlsErr := agentsrv.BuildServerTLS(agentsrv.TLSConfig{
		CertFile: cfg.TLS.CertFile,
		KeyFile:  cfg.TLS.KeyFile,
		CAFile:   cfg.TLS.CAFile,
	})
	if tlsErr != nil {
		// Certificate files not yet provisioned (install-script step pending).
		// Log clearly and refuse to start without mTLS — never fall back to
		// plaintext.
		logger.Error("agent: TLS setup failed — cert files are required; "+
			"run the install script to provision a node certificate before starting the agent",
			"error", tlsErr,
			"cert_file", cfg.TLS.CertFile,
			"key_file", cfg.TLS.KeyFile,
			"ca_file", cfg.TLS.CAFile,
		)
		return fmt.Errorf("agent: tls: %w", tlsErr)
	}

	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("agent: listen %s: %w", addr, err)
	}

	grpcSrv := grpc.NewServer(grpc.Creds(credentials.NewTLS(tlsCredential)))
	svc := agentsrv.New(cfg.WatchPaths, logger)
	svc.Register(grpcSrv)

	logger.Info("agent: gRPC server listening", "addr", addr)
	return grpcSrv.Serve(lis)
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
