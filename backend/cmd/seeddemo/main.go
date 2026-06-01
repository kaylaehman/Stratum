// Command seeddemo populates a throwaway SQLite database with realistic-looking
// fake data so the Stratum UI can be demoed without real infrastructure.
//
// NEVER connects to real hosts. All IP addresses are documentation ranges
// (192.0.2.0/24, 198.51.100.0/24). All data is synthetic.
//
// Usage:
//
//	DATABASE_URL=sqlite:///../.demo/stratum-demo.db \
//	ENCRYPTION_KEY=<64-hex-char key> \
//	go run ./cmd/seeddemo
//
// Expects a FRESH database. Re-running against an existing database may
// produce duplicate-key errors — delete the .db file and rerun.
package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/kaylaehman/stratum/backend/auth"
	"github.com/kaylaehman/stratum/backend/crypto"
	appdb "github.com/kaylaehman/stratum/backend/db"
	"github.com/kaylaehman/stratum/backend/db/sqlite"
	"github.com/kaylaehman/stratum/backend/nodes"
)

// demoTOTPSecret is a well-known base32 TOTP seed used only in demo DBs.
const demoTOTPSecret = "JBSWY3DPEHPK3PXP"

func main() {
	if err := run(); err != nil {
		log.Fatalf("seeddemo: %v", err)
	}
}

func run() error {
	ctx := context.Background()

	// --- open + migrate -------------------------------------------------
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "sqlite:///../.demo/stratum-demo.db"
	}
	sqldb, err := appdb.Open(dbURL)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer sqldb.Close()

	if err := appdb.Migrate(sqldb); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}

	store := sqlite.New(sqldb)

	// --- status-only mode -----------------------------------------------
	// SEED_MODE=status flips every demo node back to "ok" (the live reachability
	// poller marks the fake TEST-NET nodes unreachable). Used right before a
	// screenshot pass so the hero shots show healthy nodes. Idempotent.
	if os.Getenv("SEED_MODE") == "status" {
		nodes, err := store.ListNodes(ctx)
		if err != nil {
			return fmt.Errorf("list nodes: %w", err)
		}
		for _, n := range nodes {
			n.Status = "ok"
			n.LastError = ""
			now := time.Now()
			n.LastSeen = &now
			if err := store.UpdateNode(ctx, n); err != nil {
				return fmt.Errorf("update node status: %w", err)
			}
		}
		fmt.Printf("seeddemo: set %d node(s) to status=ok\n", len(nodes))
		return nil
	}

	// --- cipher ---------------------------------------------------------
	encKeyHex := os.Getenv("ENCRYPTION_KEY")
	if encKeyHex == "" {
		// fixed demo key (32 bytes / 64 hex chars) — never use in production
		encKeyHex = "09ac40ae1f3604f38e843215752593e099fd71e6b9a488b99887651176909c76"
	}
	keyBytes, err := hex.DecodeString(encKeyHex)
	if err != nil {
		return fmt.Errorf("decode ENCRYPTION_KEY: %w", err)
	}
	cipher, err := crypto.New(keyBytes)
	if err != nil {
		return fmt.Errorf("new cipher: %w", err)
	}

	// --- lab-node mode --------------------------------------------------
	// SEED_MODE=labnode registers a REAL throwaway standalone node backed by a
	// local docker-in-docker sandbox (SSH on localhost:12222, Docker on
	// tcp://localhost:12375) so the live "why is this broken?" diagnostic and
	// UID/GID visualizer have a real container to inspect. Idempotent: replaces
	// any existing demo-lab node. Demo-only credentials (never real infra).
	if os.Getenv("SEED_MODE") == "labnode" {
		creds := nodes.NodeCredentials{
			Method:         nodes.MethodSSHPassword,
			SSHUser:        "root",
			SSHPassword:    "demopass123",
			DockerEndpoint: "tcp://localhost:12375",
		}
		blob, err := creds.Seal(cipher)
		if err != nil {
			return fmt.Errorf("seal lab creds: %w", err)
		}
		existing, _ := store.ListNodes(ctx)
		for _, n := range existing {
			if n.Name == "demo-lab" {
				_ = store.DeleteNode(ctx, n.ID)
			}
		}
		node := appdb.Node{
			ID:                   uuid.NewString(),
			Name:                 "demo-lab",
			Type:                 "standalone",
			Host:                 "localhost",
			Port:                 12222,
			AuthMethod:           nodes.MethodSSHPassword,
			OSType:               "alpine",
			CapabilitiesJSON:     `{"proxmox":false,"docker":true,"agent":false,"systemd":false,"cron":false}`,
			CredentialsEncrypted: blob,
			CredentialsVersion:   1,
			DockerEndpoint:       creds.DockerEndpoint,
			Status:               "ok",
		}
		if err := store.CreateNode(ctx, node); err != nil {
			return fmt.Errorf("create lab node: %w", err)
		}
		fmt.Printf("seeddemo: registered demo-lab (standalone) host=localhost:12222 docker=%s\n", creds.DockerEndpoint)
		return nil
	}

	// --- IDs (stable across one seed run) --------------------------------
	proxNodeID := uuid.NewString()
	dockerNodeID := uuid.NewString()
	sshNodeID := uuid.NewString()

	jellyfinID := uuid.NewString()
	plexID := uuid.NewString()
	postgresID := uuid.NewString()
	mariadbID := uuid.NewString()
	prometheusID := uuid.NewString()
	grafanaID := uuid.NewString()
	piholeID := uuid.NewString()
	nginxID := uuid.NewString()

	adminID := uuid.NewString()

	// ====================================================================
	// 1. Nodes
	// ====================================================================
	// credentials_encrypted is NOT NULL in the schema; seed with an empty sealed blob.
	emptySealed, err := cipher.Seal([]byte("{}"))
	if err != nil {
		return fmt.Errorf("seal empty creds: %w", err)
	}

	nodes := []appdb.Node{
		{
			ID:                   proxNodeID,
			Name:                 "demo-prox",
			Type:                 "proxmox",
			Host:                 "192.0.2.10",
			Port:                 8006,
			AuthMethod:           "token",
			OSType:               "debian",
			CapabilitiesJSON:     `{"proxmox":true,"docker":true,"agent":false,"systemd":true,"cron":true}`,
			Status:               "ok",
			CredentialsEncrypted: emptySealed,
		},
		{
			ID:                   dockerNodeID,
			Name:                 "demo-docker",
			Type:                 "standalone",
			Host:                 "192.0.2.20",
			Port:                 22,
			AuthMethod:           "password",
			OSType:               "ubuntu",
			CapabilitiesJSON:     `{"proxmox":false,"docker":true,"agent":false,"systemd":true,"cron":true}`,
			Status:               "ok",
			CredentialsEncrypted: emptySealed,
		},
		{
			ID:                   sshNodeID,
			Name:                 "edge-01",
			Type:                 "ssh",
			Host:                 "198.51.100.5",
			Port:                 22,
			AuthMethod:           "password",
			OSType:               "alpine",
			CapabilitiesJSON:     `{"proxmox":false,"docker":false,"agent":false,"systemd":false,"cron":true}`,
			Status:               "ok",
			CredentialsEncrypted: emptySealed,
		},
	}
	for _, n := range nodes {
		if err := store.CreateNode(ctx, n); err != nil {
			return fmt.Errorf("create node %s: %w", n.Name, err)
		}
	}
	fmt.Println("nodes: 3 created")

	// ====================================================================
	// 2. Containers (~8, spread across proxmox + docker nodes)
	// ====================================================================
	fakeHex64 := func(seed string) string {
		h := fmt.Sprintf("%064x", []byte(seed))
		if len(h) > 64 {
			return h[:64]
		}
		return fmt.Sprintf("%-064s", h)
	}
	fakeDigest := func(seed string) string {
		return "sha256:" + fmt.Sprintf("%064x", []byte("digest-"+seed))[:64]
	}

	containers := []appdb.Container{
		{
			ID: jellyfinID, NodeID: proxNodeID,
			DockerID: fakeHex64("jellyfin"), Name: "jellyfin",
			Image: "jellyfin/jellyfin:latest", ImageID: fakeDigest("jellyfin"),
			Status: "running", ComposeProject: "media",
		},
		{
			ID: plexID, NodeID: proxNodeID,
			DockerID: fakeHex64("plex"), Name: "plex",
			Image: "plexinc/pms-docker:latest", ImageID: fakeDigest("plex"),
			Status: "running", ComposeProject: "media",
		},
		{
			ID: postgresID, NodeID: proxNodeID,
			DockerID: fakeHex64("postgres"), Name: "postgres",
			Image: "postgres:16", ImageID: fakeDigest("postgres"),
			Status: "running", ComposeProject: "db",
		},
		{
			ID: mariadbID, NodeID: proxNodeID,
			DockerID: fakeHex64("mariadb"), Name: "mariadb",
			Image: "mariadb:11", ImageID: fakeDigest("mariadb"),
			Status: "exited", ComposeProject: "db",
		},
		{
			ID: prometheusID, NodeID: dockerNodeID,
			DockerID: fakeHex64("prometheus"), Name: "prometheus",
			Image: "prom/prometheus:latest", ImageID: fakeDigest("prometheus"),
			Status: "running", ComposeProject: "monitoring",
		},
		{
			ID: grafanaID, NodeID: dockerNodeID,
			DockerID: fakeHex64("grafana"), Name: "grafana",
			Image: "grafana/grafana:latest", ImageID: fakeDigest("grafana"),
			Status: "running", ComposeProject: "monitoring",
		},
		{
			ID: piholeID, NodeID: dockerNodeID,
			DockerID: fakeHex64("pihole"), Name: "pihole",
			Image: "pihole/pihole:latest", ImageID: fakeDigest("pihole"),
			Status: "running", ComposeProject: "network",
		},
		{
			ID: nginxID, NodeID: dockerNodeID,
			DockerID: fakeHex64("nginx"), Name: "nginx",
			Image: "nginx:1.25", ImageID: fakeDigest("nginx"),
			Status: "running", ComposeProject: "network",
		},
	}
	for _, c := range containers {
		if err := store.UpsertContainer(ctx, c); err != nil {
			return fmt.Errorf("upsert container %s: %w", c.Name, err)
		}
	}
	fmt.Println("containers: 8 created")

	// ====================================================================
	// 3. Remediation proposal (grafana bind-mount permission fix)
	// ====================================================================
	proposal := appdb.RemediationProposal{
		ID:          uuid.NewString(),
		Source:      "diagnostic",
		Title:       "Fix bind-mount permissions on grafana /var/lib/grafana",
		Rationale:   "The grafana container runs as UID 472 (grafana), but /opt/demo/grafana on the host is owned by UID 1000 (ubuntu). The container process falls into the 'other' permission category and cannot write to /var/lib/grafana. Running `chown -R 472:472 /opt/demo/grafana` on the host will align ownership and restore write access.",
		NodeID:      dockerNodeID,
		ContainerID: grafanaID,
		Commands:    []string{"chown -R 472:472 /opt/demo/grafana"},
		RiskLevel:   "medium",
		Status:      "proposed",
		CreatedBy:   adminID,
		CreatedAt:   time.Now(),
	}
	if err := store.CreateProposal(ctx, proposal); err != nil {
		return fmt.Errorf("create proposal: %w", err)
	}
	fmt.Println("remediation proposal: 1 created")

	// ====================================================================
	// 4. Bind mount (grafana — UID/GID mismatch scenario)
	// ====================================================================
	mount := appdb.MountRow{
		ID:               uuid.NewString(),
		NodeID:           dockerNodeID,
		ContainerID:      grafanaID,
		Type:             "bind",
		Source:           "/opt/demo/grafana",
		NormalizedSource: "/opt/demo/grafana",
		Destination:      "/var/lib/grafana",
		RW:               true,
	}
	if err := store.ReplaceContainerMounts(ctx, grafanaID, []appdb.MountRow{mount}); err != nil {
		return fmt.Errorf("replace mounts: %w", err)
	}
	fmt.Println("mounts: 1 bind mount created (grafana)")

	// ====================================================================
	// 5. CVE scan data (nginx:1.25 + grafana)
	// ====================================================================
	nginxDigest := "sha256:nginx121demo0000000000000000000000000000000000000000000000000001"
	grafanaDigest := "sha256:grafanademo0000000000000000000000000000000000000000000000000001"
	prometheusDigest := "sha256:prometheusdemo00000000000000000000000000000000000000000000000001"

	scans := []appdb.ImageScanRow{
		{ImageDigest: nginxDigest, Image: "nginx:1.21", Critical: 2, High: 5, Medium: 8, Low: 3},
		{ImageDigest: grafanaDigest, Image: "grafana/grafana:9.5.2", Critical: 1, High: 3, Medium: 6, Low: 2},
		{ImageDigest: prometheusDigest, Image: "prom/prometheus:2.44.0", Critical: 0, High: 1, Medium: 4, Low: 5},
	}
	for _, scan := range scans {
		if err := store.UpsertImageScan(ctx, scan); err != nil {
			return fmt.Errorf("upsert image scan %s: %w", scan.Image, err)
		}
	}

	nginxCVEs := []appdb.CVEResultRow{
		{CVEID: "CVE-2021-23017", Severity: "critical", Package: "nginx", InstalledVersion: "1.21.0", FixedVersion: "1.21.1", Title: "1-byte memory overwrite in DNS response handling"},
		{CVEID: "CVE-2021-3618", Severity: "critical", Package: "nginx", InstalledVersion: "1.21.0", FixedVersion: "1.21.3", Title: "ALPACA: Cross-protocol attack on TLS sessions"},
		{CVEID: "CVE-2022-41741", Severity: "high", Package: "nginx", InstalledVersion: "1.21.0", FixedVersion: "1.23.2", Title: "Memory corruption in ngx_http_mp4_module"},
		{CVEID: "CVE-2021-3711", Severity: "high", Package: "openssl", InstalledVersion: "1.1.1k", FixedVersion: "1.1.1l", Title: "SM2 decryption buffer overflow"},
		{CVEID: "CVE-2021-3712", Severity: "high", Package: "openssl", InstalledVersion: "1.1.1k", FixedVersion: "1.1.1l", Title: "Read buffer overruns in ASN1_STRING_to_UTF8"},
		{CVEID: "CVE-2022-0778", Severity: "medium", Package: "openssl", InstalledVersion: "1.1.1k", FixedVersion: "1.1.1n", Title: "Infinite loop in BN_mod_sqrt leading to DoS"},
	}
	if err := store.ReplaceCVEResults(ctx, nginxDigest, nginxCVEs); err != nil {
		return fmt.Errorf("replace cve results (nginx): %w", err)
	}

	grafanaCVEs := []appdb.CVEResultRow{
		{CVEID: "CVE-2023-29409", Severity: "critical", Package: "golang.org/x/net", InstalledVersion: "0.8.0", FixedVersion: "0.13.0", Title: "Excessive CPU consumption in certificate chain verification"},
		{CVEID: "CVE-2022-41723", Severity: "high", Package: "golang.org/x/net", InstalledVersion: "0.8.0", FixedVersion: "0.7.0", Title: "HPACK decoder DoS via crafted header block"},
		{CVEID: "CVE-2023-24540", Severity: "high", Package: "html/template", InstalledVersion: "go1.20.1", FixedVersion: "go1.20.4", Title: "Improper handling of JavaScript whitespace"},
		{CVEID: "CVE-2022-27664", Severity: "medium", Package: "golang.org/x/net", InstalledVersion: "0.8.0", FixedVersion: "0.1.1-beta1", Title: "HTTP/2 server DoS via RST_STREAM flood"},
	}
	if err := store.ReplaceCVEResults(ctx, grafanaDigest, grafanaCVEs); err != nil {
		return fmt.Errorf("replace cve results (grafana): %w", err)
	}

	prometheusCVEs := []appdb.CVEResultRow{
		{CVEID: "CVE-2022-41717", Severity: "high", Package: "golang.org/x/net", InstalledVersion: "0.1.0", FixedVersion: "0.4.0", Title: "Memory exhaustion in HTTP/2 server"},
		{CVEID: "CVE-2021-33196", Severity: "medium", Package: "archive/zip", InstalledVersion: "go1.16.0", FixedVersion: "go1.16.5", Title: "Invalid file count in ZIP archive causes panic"},
	}
	if err := store.ReplaceCVEResults(ctx, prometheusDigest, prometheusCVEs); err != nil {
		return fmt.Errorf("replace cve results (prometheus): %w", err)
	}
	fmt.Println("CVE data: 3 image scans + 12 CVE results created")

	// ====================================================================
	// 6. Admin user + TOTP
	// ====================================================================
	hash, err := auth.HashPassword("demo-admin-pw")
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	adminUser := appdb.User{
		ID:           adminID,
		Username:     "demo-admin",
		Email:        "demo@example.com",
		PasswordHash: hash,
		Role:         "admin",
	}
	if err := store.CreateUser(ctx, adminUser); err != nil {
		return fmt.Errorf("create admin user: %w", err)
	}

	sealed, err := cipher.Seal([]byte(demoTOTPSecret))
	if err != nil {
		return fmt.Errorf("seal totp secret: %w", err)
	}
	totpRow := appdb.UserTOTP{
		UserID:          adminID,
		SecretEncrypted: sealed,
		Enabled:         true,
		RecoveryHashes:  []string{},
	}
	if err := store.UpsertUserTOTP(ctx, totpRow); err != nil {
		return fmt.Errorf("upsert totp: %w", err)
	}
	fmt.Println("admin user: created with TOTP enabled")

	// ====================================================================
	// 7. Secret group + one secret
	// ====================================================================
	groupID := uuid.NewString()
	group := appdb.SecretGroup{
		ID:          groupID,
		Name:        "monitoring-stack",
		Description: "Credentials for the Prometheus/Grafana monitoring stack",
	}
	if err := store.CreateSecretGroup(ctx, group); err != nil {
		return fmt.Errorf("create secret group: %w", err)
	}

	grafanaPassEncrypted, err := cipher.Seal([]byte("demo-grafana-admin-password"))
	if err != nil {
		return fmt.Errorf("seal grafana password: %w", err)
	}
	secret := appdb.SecretRow{
		ID:             uuid.NewString(),
		GroupID:        groupID,
		Key:            "GF_SECURITY_ADMIN_PASSWORD",
		ValueEncrypted: grafanaPassEncrypted,
	}
	if err := store.UpsertSecret(ctx, secret); err != nil {
		return fmt.Errorf("upsert secret: %w", err)
	}
	fmt.Println("secrets: 1 group + 1 secret created")

	// ====================================================================
	// Summary
	// ====================================================================
	fmt.Println()
	fmt.Println("======================================================")
	fmt.Println("  STRATUM DEMO SEED COMPLETE")
	fmt.Println("======================================================")
	fmt.Printf("  DEMO ADMIN:  demo-admin / demo-admin-pw\n")
	fmt.Printf("  TOTP secret: %s\n", demoTOTPSecret)
	fmt.Println("  (use any RFC 6238 authenticator app with the TOTP secret above)")
	fmt.Println("======================================================")
	return nil
}
