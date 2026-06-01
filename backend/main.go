// Command stratum is the Stratum backend API server. It loads configuration,
// opens and migrates the database, wires the HTTP router + middleware, and
// serves until interrupted.
package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/google/uuid"
	"golang.org/x/time/rate"

	"github.com/kaylaehman/stratum/backend/activity"
	"github.com/kaylaehman/stratum/backend/prommetrics"
	"github.com/kaylaehman/stratum/backend/agent"
	"github.com/kaylaehman/stratum/backend/ai"
	"github.com/kaylaehman/stratum/backend/alertpolicy"
	"github.com/kaylaehman/stratum/backend/api"
	"github.com/kaylaehman/stratum/backend/auth"
	"github.com/kaylaehman/stratum/backend/automation"
	"github.com/kaylaehman/stratum/backend/backup"
	"github.com/kaylaehman/stratum/backend/config"
	"github.com/kaylaehman/stratum/backend/configversion"
	"github.com/kaylaehman/stratum/backend/crypto"
	"github.com/kaylaehman/stratum/backend/db"
	"github.com/kaylaehman/stratum/backend/db/sqlite"
	"github.com/kaylaehman/stratum/backend/depgraph"
	"github.com/kaylaehman/stratum/backend/docker"
	"github.com/kaylaehman/stratum/backend/drexport"
	"github.com/kaylaehman/stratum/backend/forecast"
	"github.com/kaylaehman/stratum/backend/fs"
	"github.com/kaylaehman/stratum/backend/hub"
	"github.com/kaylaehman/stratum/backend/inventory"
	"github.com/kaylaehman/stratum/backend/logtail"
	"github.com/kaylaehman/stratum/backend/metrics"
	"github.com/kaylaehman/stratum/backend/mountindex"
	"github.com/kaylaehman/stratum/backend/nodeconn"
	"github.com/kaylaehman/stratum/backend/nodes"
	"github.com/kaylaehman/stratum/backend/orchestration"
	"github.com/kaylaehman/stratum/backend/permissions"
	"github.com/kaylaehman/stratum/backend/proxy"
	"github.com/kaylaehman/stratum/backend/proxmox"
	"github.com/kaylaehman/stratum/backend/recreate"
	"github.com/kaylaehman/stratum/backend/remediation"
	"github.com/kaylaehman/stratum/backend/certs"
	"github.com/kaylaehman/stratum/backend/chatbot"
	"github.com/kaylaehman/stratum/backend/cve"
	dnspkg "github.com/kaylaehman/stratum/backend/dns"
	"github.com/kaylaehman/stratum/backend/features"
	"github.com/kaylaehman/stratum/backend/filewatch"
	"github.com/kaylaehman/stratum/backend/placement"
	"github.com/kaylaehman/stratum/backend/push"
	"github.com/kaylaehman/stratum/backend/sso"
	"github.com/kaylaehman/stratum/backend/scheduler"
	"github.com/kaylaehman/stratum/backend/secrets"
	"github.com/kaylaehman/stratum/backend/security"
	"github.com/kaylaehman/stratum/backend/server"
	"github.com/kaylaehman/stratum/backend/stacks"
	"github.com/kaylaehman/stratum/backend/skills"
	"github.com/kaylaehman/stratum/backend/topology"
	"github.com/kaylaehman/stratum/backend/twofa"
	"github.com/kaylaehman/stratum/backend/updates"
	"github.com/kaylaehman/stratum/backend/uptime"
	"github.com/kaylaehman/stratum/backend/volumes"
	"github.com/kaylaehman/stratum/backend/webhooks"
)

// accessTokenTTL is the lifetime of issued access tokens.
const accessTokenTTL = 15 * time.Minute

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	if err := run(logger); err != nil {
		logger.Error("fatal", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	sqldb, err := db.Open(cfg.DatabaseURL)
	if err != nil {
		return err
	}
	if err := db.Migrate(sqldb); err != nil {
		return err
	}
	store := sqlite.New(sqldb)
	defer store.Close()

	maybeSeedAdmin(context.Background(), store, cfg, logger)

	cipher, err := crypto.New(cfg.EncryptionKey)
	if err != nil {
		return err
	}

	jwt := auth.NewJWT(cfg.JWTSecret, accessTokenTTL)
	h := hub.New()
	conn := nodeconn.NewManager(store, cipher)
	nodesSvc := nodes.NewService(store, cipher)
	poller := inventory.NewPoller(store, conn, h, logger)
	// Let the poller confirm SSH reachability (with the pinned host key) so
	// SSH-only nodes — and nodes whose Docker/Proxmox transport is down but SSH
	// is up — are marked "ok" instead of permanently "unreachable".
	poller.SetReachability(nodesSvc.ProbeReachability)
	containerUsers := permissions.NewContainerCache(func(ctx context.Context, nodeID, containerID, p string) ([]byte, error) {
		clients, err := conn.Get(ctx, nodeID)
		if err != nil {
			return nil, err
		}
		if clients.Docker == nil {
			return nil, fmt.Errorf("node %s has no docker client", nodeID)
		}
		return clients.Docker.CopyFromContainer(ctx, containerID, p)
	}, 5*time.Minute)

	dockerForNode := func(ctx context.Context, nodeID string) (*docker.Client, error) {
		clients, err := conn.Get(ctx, nodeID)
		if err != nil {
			return nil, err
		}
		if clients.Docker == nil {
			return nil, fmt.Errorf("node %s has no docker client", nodeID)
		}
		return clients.Docker, nil
	}
	// MVP single-admin: any authenticated user may read any node's logs; RBAC
	// (feature 30) will replace this with a per-node read-access check.
	logsMgr := logtail.NewManager(dockerForNode, h, func(context.Context, string, string) (bool, error) { return true, nil })
	mountIdx := mountindex.New(store, dockerForNode, 30*time.Second)
	securityScanner := security.NewScanner(store, security.ClientProvider(dockerForNode), containerUsers, 30*time.Second)
	// pushSvc is declared here so the securityScanner.SetNotify closure below can
	// capture it by reference; it is assigned after VAPID init completes.
	var pushSvc *push.Service
	webhookDispatcher := webhooks.New(store)
	securityScanner.SetNotify(func(ctx context.Context, trigger, title, text string) {
		webhookDispatcher.Notify(ctx, trigger, webhooks.Message{Title: title, Text: text})
		if pushSvc != nil && isCriticalTrigger(trigger) {
			_ = pushSvc.SendToAll(ctx, push.Payload{
				Title: title,
				Body:  text,
				Tag:   trigger,
				URL:   "/security",
				Actions: []push.Action{
					{Action: "ack", Title: "Acknowledge"},
				},
			})
		}
	})
	poller.SetNotify(func(ctx context.Context, trigger, title, text string) {
		webhookDispatcher.Notify(ctx, trigger, webhooks.Message{Title: title, Text: text})
	})
	volumeSvc := volumes.New(store, volumes.ClientProvider(dockerForNode), mountIdx, volumeAlertBytes())
	volumeSvc.SetNotify(func(ctx context.Context, trigger, title, text string) {
		webhookDispatcher.Notify(ctx, trigger, webhooks.Message{Title: title, Text: text})
	})
	metricsSampler := metrics.NewSampler(store, metrics.ClientProvider(dockerForNode), 15*time.Second, 7*24*time.Hour)
	topologySvc := topology.New(store, topology.ClientProvider(dockerForNode))
	depGraphSvc := depgraph.New(store, depgraph.ClientProvider(dockerForNode), mountIdx)
	updateSvc := updates.New(store, updates.ClientProvider(dockerForNode), 6*time.Hour)
	updateSvc.SetNotify(func(ctx context.Context, trigger, title, text string) {
		webhookDispatcher.Notify(ctx, trigger, webhooks.Message{Title: title, Text: text})
	})
	secretSvc := secrets.New(store, cipher)
	filesSvc := fs.NewService(store, cipher, uploadMaxBytes())
	schedulerSvc := scheduler.New(filesSvc.Exec)
	cveSvc := cve.New(store, cve.ClientProvider(dockerForNode), cve.NewScanner())
	backupSvc := backup.New(store, filesSvc.Exec)
	backupSvc.SetNotify(func(ctx context.Context, trigger, title, text string) {
		webhookDispatcher.Notify(ctx, trigger, webhooks.Message{Title: title, Text: text})
	})
	backupSvc.SetProxmox(func(ctx context.Context, nodeID string) (*proxmox.Client, error) {
		clients, err := conn.Get(ctx, nodeID)
		if err != nil {
			return nil, err
		}
		if clients.Proxmox == nil {
			return nil, fmt.Errorf("node %s has no proxmox client", nodeID)
		}
		return clients.Proxmox, nil
	})
	// Wire verify store into backup service.
	backupSvc.SetVerifyStore(store)

	twoFASvc := twofa.New(store, cipher)
	recreateSvc := recreate.New(store, recreate.ClientProvider(dockerForNode))
	stacksSvc := stacks.New(store, filesSvc, cipher)
	aiSvc := ai.New(store, cipher, cfg.AnthropicKey, cfg.OllamaBaseURL)
	proxySvc := proxy.New(store, cipher)
	proxySvc.WithFiles(filesSvc)
	dnsSvc := dnspkg.New(store, cipher)
	featureSvc := features.New(store)
	chatSvc := chatbot.New(store, cipher, logger, func(ctx context.Context) bool {
		return featureSvc.Enabled(ctx, "feature.chat_integration")
	}, aiSvc)
	certSvc := certs.New(store, filesSvc.Exec, 6*time.Hour)
	certSvc.SetNotify(func(ctx context.Context, trigger, title, text string) {
		webhookDispatcher.Notify(ctx, trigger, webhooks.Message{Title: title, Text: text})
	})

	// Agent mTLS transport. The CA cert path is required; the client cert/key
	// are optional until the install-script provisioning step is implemented.
	// A missing or invalid CA is non-fatal: agent capabilities remain disabled
	// and SSH-only nodes continue to function unchanged.
	var agentTLSCfg *tls.Config
	if cfg.AgentCACertPath != "" {
		var tlsErr error
		agentTLSCfg, tlsErr = agent.ClientTLSConfig("", "", cfg.AgentCACertPath)
		if tlsErr != nil {
			logger.Warn("agent: TLS config build failed; agent streaming disabled",
				"ca_cert", cfg.AgentCACertPath, "error", tlsErr)
		}
	} else {
		logger.Info("agent: AGENT_CA_CERT_PATH not set; agent streaming disabled")
	}
	agentMgr := agent.NewManager(store, agentTLSCfg, logger)
	agentOrch := agent.NewOrchestrator(store, agentMgr, agentTLSCfg, logger)

	fileWatchSvc := filewatch.New(store, filesSvc.Exec)
	fileWatchSvc.SetNotify(func(ctx context.Context, trigger, title, text string) {
		webhookDispatcher.Notify(ctx, trigger, webhooks.Message{Title: title, Text: text})
	})
	ssoSvc := sso.New(store, cipher)
	remediationSvc := remediation.New(store, filesSvc.Exec)

	// Orchestration service.
	orchestrationSvc := orchestration.NewService(store, conn, depGraphSvc, logger)

	// Config versioning service.
	configVersionReadFn := func(ctx context.Context, nodeID, path string) ([]byte, error) {
		content, tooLarge, _, err := filesSvc.Preview(ctx, nodeID, path)
		if err != nil {
			return nil, err
		}
		if tooLarge {
			return nil, configversion.ErrContentTooLarge
		}
		return content, nil
	}
	configVersionWriteFn := func(ctx context.Context, nodeID, path string, content []byte) error {
		return filesSvc.Write(ctx, nodeID, path, content, nil)
	}
	configVersionSvc := configversion.New(store, configVersionReadFn, configVersionWriteFn, false)

	// Capacity forecast service (7-day lookback).
	forecastSvc := forecast.New(store, 7*24*time.Hour)

	// Secret expiry + scanner sub-services.
	secretExpiry := secrets.NewExpiry(store)
	secretExpiry.SetNotify(func(ctx context.Context, trigger, title, text string) {
		webhookDispatcher.Notify(ctx, trigger, webhooks.Message{Title: title, Text: text})
	})
	secretScanner := secrets.NewScannerService(&fsFileReader{filesSvc})
	secretSvc.SetExpiry(secretExpiry)
	secretSvc.SetScanner(secretScanner)

	// Alert policy service.
	alertPolicySvc := alertpolicy.New(store)

	// DR export service.
	drExportSvc := drexport.New(store, dnsSvc, proxySvc, volumeSvc)

	// Placement service (C8): ranks docker-capable nodes by available headroom.
	// No disk prober yet (nil = disk scores omitted); wire a DiskProber later.
	placementSvc := placement.New(store, store, nil)

	// Web Push service (C9): VAPID keypair managed in DB; non-fatal on failure.
	// The VAPID subject is a mailto: derived from BASE_URL or a stable constant.
	pushSubject := "mailto:stratum@" + sanitisePushHost(cfg.BaseURL)
	if svc, pushErr := push.New(context.Background(), store, pushSubject, logger); pushErr != nil {
		logger.Warn("push: service init failed; push notifications disabled", "err", pushErr)
	} else {
		pushSvc = svc
	}

	// Container-troubleshooting skill library (reference data). Graceful: a
	// missing/empty SKILLS_DIR yields an empty library, not a startup failure.
	skillLib, err := skills.Load(cfg.SkillsDir)
	if err != nil {
		return err
	}
	// Merge user-authored (custom) skills from the DB on top of the shipped
	// library so they survive restarts and participate in matching like any
	// built-in skill. A malformed stored skill is logged and skipped, never fatal.
	if customSkills, err := store.ListCustomSkills(context.Background()); err != nil {
		logger.Warn("skills: load custom skills", "error", err)
	} else {
		for _, cs := range customSkills {
			s, err := skills.Parse([]byte(cs.YAML))
			if err != nil {
				logger.Warn("skills: skip malformed custom skill", "id", cs.ID, "error", err)
				continue
			}
			s.Source = skills.SourceCustom
			skillLib.Upsert(s)
		}
	}
	logger.Info("skills library loaded", "count", skillLib.Len(), "dir", cfg.SkillsDir)

	uptimeSvc := uptime.New(store, logger)
	uptimeSvc.SetNotify(func(ctx context.Context, trigger string, msg webhooks.Message) {
		webhookDispatcher.Notify(ctx, trigger, msg)
	})

	// Import the uptime triggers package so its init() registers the trigger.
	// The import happens via the uptime package itself (triggers.go init).

	// Automations engine: build handler map and wire the engine.
	automationHandlers := automation.BuildHandlers(store, automation.Deps{
		Store:       store,
		Conn:        conn,
		Security:    securityScanner,
		CVE:         cveSvc,
		Volumes:     volumeSvc,
		Recreate:    recreateSvc,
		Backups:     backupSvc,
		Remediation: remediationSvc,
		Files:       filesSvc,
		Forecast:    forecastSvc,
		Notify: func(ctx context.Context, trigger, title, text string) {
			webhookDispatcher.Notify(ctx, trigger, webhooks.Message{Title: title, Text: text})
		},
	})
	automationEngine := automation.New(store, activity.NewStore(store), webhookDispatcher, automationHandlers, logger)

	handlers := &api.Handlers{
		Store:          store,
		Activity:       activity.NewStore(store),
		JWT:            jwt,
		Hub:            h,
		Nodes:          nodesSvc,
		Poller:         poller,
		Files:          filesSvc,
		Conn:           conn,
		ContainerUsers: containerUsers,
		Logs:           logsMgr,
		Mounts:         mountIdx,
		Security:       securityScanner,
		Volumes:        volumeSvc,
		Topology:       topologySvc,
		DepGraph:       depGraphSvc,
		Webhooks:       webhookDispatcher,
		Updater:        updateSvc,
		Secrets:        secretSvc,
		Scheduler:      schedulerSvc,
		CVE:            cveSvc,
		Backups:        backupSvc,
		TwoFA:          twoFASvc,
		Recreate:       recreateSvc,
		Stacks:         stacksSvc,
		AI:             aiSvc,
		Remediation:    remediationSvc,
		Certs:          certSvc,
		Proxy:          proxySvc,
		DNS:            dnsSvc,
		Features:       featureSvc,
		Chat:           chatSvc,
		FileWatch:      fileWatchSvc,
		SSO:            ssoSvc,
		Skills:         skillLib,
		Uptime:         uptimeSvc,
		Automation:     automationEngine,
		Orchestration:  orchestrationSvc,
		ConfigVersions: configVersionSvc,
		Forecast:       forecastSvc,
		AlertPolicy:    alertPolicySvc,
		DRExportSvc:    drExportSvc,
		Placement:      placementSvc,
		Push:           pushSvc,
		Logger:         logger,
		StartedAt:      time.Now(),
		SecureCookies:  strings.HasPrefix(cfg.BaseURL, "https"),
		PreviewLimiter: rate.NewLimiter(rate.Every(2*time.Second), 5),
	}

	promReg := prommetrics.Registry(store, h)
	router := server.NewRouter(&server.Deps{Handlers: handlers, JWT: jwt, Store: store, PromRegistry: promReg})
	srv := server.New(fmt.Sprintf(":%d", cfg.Port), router, logger)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go poller.Run(ctx)                              // inventory poller; stops on shutdown
	go volumeSvc.RunDailySampler(ctx, 24*time.Hour) // volume size-trend sampler
	go metricsSampler.Run(ctx)                      // 15s resource-timeline sampler
	go chatSvc.Run(ctx)                             // inbound chat-command poller (no-op until enabled+configured)
	go uptimeSvc.Run(ctx)                           // uptime monitor checker loop
	go automationEngine.Run(ctx)                    // automation engine tick loop (60s); stops on shutdown
	go func() {
		t := time.NewTicker(24 * time.Hour)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				_ = secretExpiry.Check(ctx)
			}
		}
	}()
	go uptimeSvc.RunPrune(ctx, 90*24*time.Hour)     // prune results older than 90 days
	// Agent streaming: probe existing nodes for agent reachability, then start
	// the watch-file orchestrator. Both are no-ops when agentTLSCfg is nil
	// (AGENT_CA_CERT_PATH not configured), so SSH-only nodes are unaffected.
	go func() {
		agent.UpdateAgentCapabilities(ctx, store, agentTLSCfg, logger)
		agentOrch.Run(ctx)
	}()

	// Best-effort warm the Trivy vulnerability DB so the first user scan isn't
	// slow. Non-fatal and time-bounded: offline deploys (no egress to the
	// trivy-db mirror) still boot normally.
	if cveSvc.Available() {
		go func() {
			if err := cveSvc.WarmDB(ctx); err != nil {
				logger.Info("trivy db warm skipped", "error", err)
			} else {
				logger.Info("trivy vulnerability db ready")
			}
		}()
	}

	go cveSvc.RunSchedules(ctx) // scheduled CVE scans; stops on shutdown

	return srv.Run(ctx)
}

// volumeAlertBytes reads STRATUM_VOLUME_ALERT_BYTES (0/unset disables the flag).
func volumeAlertBytes() int64 {
	if v := os.Getenv("STRATUM_VOLUME_ALERT_BYTES"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 {
			return n
		}
	}
	return 0
}

// uploadMaxBytes reads STRATUM_UPLOAD_MAX_BYTES (default fs.DefaultUploadMax).
func uploadMaxBytes() int64 {
	if v := os.Getenv("STRATUM_UPLOAD_MAX_BYTES"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 {
			return n
		}
	}
	return fs.DefaultUploadMax
}

// fsFileReader adapts *fs.Service to the secrets.FileReader interface so the
// scanner can read files without importing the full fs.Service type.
type fsFileReader struct{ svc *fs.Service }

func (a *fsFileReader) ReadFile(ctx context.Context, nodeID, filePath string) (string, error) {
	content, tooLarge, _, err := a.svc.Preview(ctx, nodeID, filePath)
	if err != nil {
		return "", err
	}
	if tooLarge {
		return "", fmt.Errorf("file too large: %s", filePath)
	}
	return string(content), nil
}

func (a *fsFileReader) ListDir(ctx context.Context, nodeID, dirPath string) ([]string, error) {
	entries, _, err := a.svc.List(ctx, nodeID, dirPath)
	if err != nil {
		return nil, err
	}
	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = e.Name
	}
	return names, nil
}

// maybeSeedAdmin creates the first admin from STRATUM_ADMIN_USER/PASSWORD when
// the users table is empty (a CI/automation escape hatch). The password is
// cleared from the in-memory config afterward; operators should unset the env
// vars after first boot.
func maybeSeedAdmin(ctx context.Context, store db.Store, cfg *config.Config, logger *slog.Logger) {
	if cfg.AdminUser == "" || cfg.AdminPassword == "" {
		return
	}
	n, err := store.CountUsers(ctx)
	if err != nil || n > 0 {
		cfg.AdminPassword = ""
		return
	}
	hash, err := auth.HashPassword(cfg.AdminPassword)
	cfg.AdminPassword = ""
	if err != nil {
		logger.Error("seed admin: hash", "error", err)
		return
	}
	if err := store.CreateUser(ctx, db.User{
		ID:           uuid.NewString(),
		Username:     cfg.AdminUser,
		PasswordHash: hash,
		Role:         "admin",
	}); err != nil {
		logger.Error("seed admin: create", "error", err)
		return
	}
	logger.Info("seeded admin user from environment", "username", cfg.AdminUser)
}

// sanitisePushHost extracts a hostname from BASE_URL for the VAPID mailto
// subject. Falls back to "stratum.local" when BASE_URL is unset or malformed.
func sanitisePushHost(baseURL string) string {
	for _, prefix := range []string{"https://", "http://"} {
		if strings.HasPrefix(baseURL, prefix) {
			host := strings.TrimPrefix(baseURL, prefix)
			if i := strings.IndexByte(host, '/'); i >= 0 {
				host = host[:i]
			}
			if host != "" {
				return host
			}
		}
	}
	return "stratum.local"
}

// criticalTriggers is the set of webhook trigger names that warrant an
// immediate push notification to all subscribed browsers.
var criticalTriggers = map[string]bool{
	"cve.critical":      true,
	"security.critical": true,
	"container.crash":   true,
	"port.new":          true,
	"sshkey.added":      true,
}

// isCriticalTrigger returns true when the webhook trigger string should also
// be sent as a Web Push notification.
func isCriticalTrigger(trigger string) bool {
	return criticalTriggers[trigger]
}
