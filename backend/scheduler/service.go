package scheduler

import (
	"context"
	"fmt"
	"strings"
)

// ExecFunc runs a command on a node over SSH (matches fs.Service.Exec).
type ExecFunc func(ctx context.Context, nodeID, cmd string, args ...string) (string, error)

// WriteFunc writes a file on a node (matches an adapter over fs.Service.Write).
type WriteFunc func(ctx context.Context, nodeID, path string, content []byte) error

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
	exec  ExecFunc
	write WriteFunc
}

// New wires the SSH exec + file-write adapters.
func New(exec ExecFunc, write WriteFunc) *Service {
	return &Service{exec: exec, write: write}
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

// SetCrontab installs new crontab content for a user by writing it to a temp
// file and running `crontab -u <user> <file>`. The caller must validate user.
func (s *Service) SetCrontab(ctx context.Context, nodeID, user, content string) error {
	if !ValidUser(user) {
		return fmt.Errorf("scheduler: invalid user %q", user)
	}
	tmp := "/tmp/stratum-cron-" + user
	if err := s.write(ctx, nodeID, tmp, []byte(ensureTrailingNewline(content))); err != nil {
		return err
	}
	defer func() { _, _ = s.exec(ctx, nodeID, "rm", "-f", tmp) }()
	if _, err := s.exec(ctx, nodeID, "crontab", "-u", user, tmp); err != nil {
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
