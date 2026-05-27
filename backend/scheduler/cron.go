// Package scheduler reads (and edits) scheduled tasks on a node over SSH
// (Feature 10) — cron jobs and systemd timers — no agent required. The parsing
// and human-readable schedule description are pure and unit-tested; the SSH
// orchestration lives in the service.
package scheduler

import (
	"strconv"
	"strings"
)

// CronJob is one parsed crontab entry.
type CronJob struct {
	User     string `json:"user"`
	Schedule string `json:"schedule"` // raw cron expression (or @shortcut)
	Command  string `json:"command"`
	Human    string `json:"human"` // best-effort human-readable schedule
	Raw      string `json:"raw"`   // the original line
}

// nicknameDesc maps cron @nicknames to descriptions.
var nicknameDesc = map[string]string{
	"@reboot":   "At startup",
	"@yearly":   "Once a year (Jan 1, 00:00)",
	"@annually": "Once a year (Jan 1, 00:00)",
	"@monthly":  "Once a month (1st, 00:00)",
	"@weekly":   "Once a week (Sun, 00:00)",
	"@daily":    "Every day at 00:00",
	"@midnight": "Every day at 00:00",
	"@hourly":   "Every hour (:00)",
}

// ParseCrontab parses `crontab -l` output for a user into jobs. Comment and
// blank lines and environment assignments (FOO=bar) are skipped.
func ParseCrontab(output, user string) []CronJob {
	jobs := []CronJob{}
	for _, line := range strings.Split(output, "\n") {
		raw := line
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "@") {
			fields := strings.Fields(line)
			if len(fields) < 2 {
				continue
			}
			sched := fields[0]
			jobs = append(jobs, CronJob{
				User: user, Schedule: sched, Command: strings.Join(fields[1:], " "),
				Human: nicknameDesc[sched], Raw: raw,
			})
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 6 {
			// Not a 5-field schedule + command; could be an env assignment — skip.
			continue
		}
		// An env assignment like FOO=bar has '=' in the first field and no spaces.
		if strings.Contains(fields[0], "=") {
			continue
		}
		sched := strings.Join(fields[0:5], " ")
		jobs = append(jobs, CronJob{
			User: user, Schedule: sched, Command: strings.Join(fields[5:], " "),
			Human: DescribeCron(sched), Raw: raw,
		})
	}
	return jobs
}

// DescribeCron renders a best-effort human-readable description of a 5-field
// cron expression. Falls back to the raw expression for anything non-trivial.
func DescribeCron(expr string) string {
	f := strings.Fields(expr)
	if len(f) != 5 {
		return expr
	}
	min, hour, dom, mon, dow := f[0], f[1], f[2], f[3], f[4]

	allStar := dom == "*" && mon == "*" && dow == "*"
	// Every day at HH:MM.
	if allStar && isNum(min) && isNum(hour) {
		return "Every day at " + clock(hour, min)
	}
	// Hourly at minute M.
	if allStar && isNum(min) && hour == "*" {
		return "Every hour at :" + pad(min)
	}
	// Every N minutes.
	if allStar && hour == "*" && strings.HasPrefix(min, "*/") {
		return "Every " + strings.TrimPrefix(min, "*/") + " minutes"
	}
	// Weekly on DOW at HH:MM.
	if dom == "*" && mon == "*" && isNum(dow) && isNum(min) && isNum(hour) {
		return "Every " + weekday(dow) + " at " + clock(hour, min)
	}
	return expr
}

func isNum(s string) bool { _, err := strconv.Atoi(s); return err == nil }

func pad(s string) string {
	if len(s) == 1 {
		return "0" + s
	}
	return s
}

func clock(hour, min string) string { return pad(hour) + ":" + pad(min) }

func weekday(dow string) string {
	names := []string{"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"}
	n, err := strconv.Atoi(dow)
	if err != nil || n < 0 || n > 7 {
		return "day " + dow
	}
	return names[n%7]
}
