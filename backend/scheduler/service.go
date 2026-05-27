package scheduler

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
)

// ExecFunc runs a command on a node over SSH (matches fs.Service.Exec).
type ExecFunc func(ctx context.Context, nodeID, cmd string, args ...string) (string, error)

// Schedule is a node's scheduled tasks.
type Schedule struct {
	Cron   []CronJob      `json:"cron"`
	Timers []SystemdTimer `json:"timers"`
}

const userMarker = "@@USER@@"

// cronScript dumps each root/home user's crontab, prefixed with a user marker.
const cronScript = `for d in /root /home/*; do
  u=$(basename "$d")
  c=$(crontab -l -u "$u" 2>/dev/null) || continue
  [ -z "$c" ] && continue
  echo "` + userMarker + ` $u"
  echo "$c"
done`

// Service reads and edits a node's scheduled tasks over SSH.
type Service struct {
	exec ExecFunc
}

// New wires the SSH exec adapter.
func New(exec ExecFunc) *Service {
	return &Service{exec: exec}
}

// Read returns the node's cron jobs (across users) and systemd timers.
func (s *Service) Read(ctx context.Context, nodeID string) (Schedule, error) {
	cronOut, err := s.exec(ctx, nodeID, "sh", "-c", cronScript)
	if err != nil {
		return Schedule{}, err
	}
	sched := Schedule{Cron: parseCronBlocks(cronOut), Timers: []SystemdTimer{}}
	// systemd may be absent; best-effort.
	if timersOut, err := s.exec(ctx, nodeID, "sh", "-c", "systemctl list-timers --all --no-pager 2>/dev/null || true"); err == nil {
		sched.Timers = ParseTimers(timersOut)
	}
	return sched, nil
}

// parseCronBlocks splits marker-delimited per-user crontab dumps into jobs.
func parseCronBlocks(out string) []CronJob {
	jobs := []CronJob{}
	var user string
	var buf []string
	flush := func() {
		if user != "" && len(buf) > 0 {
			jobs = append(jobs, ParseCrontab(strings.Join(buf, "\n"), user)...)
		}
		buf = nil
	}
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, userMarker+" ") {
			flush()
			user = strings.TrimSpace(strings.TrimPrefix(line, userMarker+" "))
			continue
		}
		buf = append(buf, line)
	}
	flush()
	return jobs
}

// SetCrontab installs new crontab content for a user. The content is
// base64-encoded and piped to `crontab -u <user> -` over stdin in a single
// command — no temp file is ever created, eliminating the /tmp symlink/TOCTOU
// race. The base64 alphabet and the charset-validated user are both safe inside
// single quotes, so there is no shell-injection surface.
func (s *Service) SetCrontab(ctx context.Context, nodeID, user, content string) error {
	if !ValidUser(user) {
		return fmt.Errorf("scheduler: invalid user %q", user)
	}
	b64 := base64.StdEncoding.EncodeToString([]byte(ensureTrailingNewline(content)))
	script := "printf '%s' '" + b64 + "' | base64 -d | crontab -u '" + user + "' -"
	if _, err := s.exec(ctx, nodeID, "sh", "-c", script); err != nil {
		return err
	}
	return nil
}

// ValidUser allows a conservative username charset.
func ValidUser(u string) bool {
	if u == "" || len(u) > 32 {
		return false
	}
	for _, r := range u {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' || r == '-') {
			return false
		}
	}
	return true
}

func ensureTrailingNewline(s string) string {
	if s == "" || strings.HasSuffix(s, "\n") {
		return s
	}
	return s + "\n"
}
