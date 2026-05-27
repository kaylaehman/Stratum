package scheduler

import "testing"

func TestParseCrontab(t *testing.T) {
	in := `# m h dom mon dow command
SHELL=/bin/bash
0 3 * * * /usr/bin/backup.sh
*/15 * * * * curl -s http://localhost/ping
@daily /opt/cleanup.sh
@reboot /opt/start.sh
not enough fields
`
	jobs := ParseCrontab(in, "root")
	if len(jobs) != 4 {
		t.Fatalf("parsed %d jobs, want 4: %+v", len(jobs), jobs)
	}
	if jobs[0].Schedule != "0 3 * * *" || jobs[0].Command != "/usr/bin/backup.sh" || jobs[0].Human != "Every day at 03:00" {
		t.Errorf("job0 = %+v", jobs[0])
	}
	if jobs[2].Schedule != "@daily" || jobs[2].Human != "Every day at 00:00" {
		t.Errorf("@daily job = %+v", jobs[2])
	}
	if jobs[0].User != "root" {
		t.Errorf("user = %q", jobs[0].User)
	}
}

func TestDescribeCron(t *testing.T) {
	cases := map[string]string{
		"0 3 * * *":     "Every day at 03:00",
		"30 9 * * *":    "Every day at 09:30",
		"5 * * * *":     "Every hour at :05",
		"*/10 * * * *":  "Every 10 minutes",
		"0 9 * * 1":     "Every Monday at 09:00",
		"0 0 1 1 *":     "0 0 1 1 *", // complex -> raw fallback
		"bad expr here": "bad expr here",
	}
	for expr, want := range cases {
		if got := DescribeCron(expr); got != want {
			t.Errorf("DescribeCron(%q) = %q, want %q", expr, got, want)
		}
	}
}

func TestParseTimers(t *testing.T) {
	// Representative `systemctl list-timers --all --no-pager` output.
	out := `NEXT                        LEFT     LAST                        PASSED   UNIT                         ACTIVATES
Mon 2026-06-01 00:00:00 UTC 5h left  Sun 2026-05-31 00:00:00 UTC 19h ago  logrotate.timer              logrotate.service
n/a                         n/a      n/a                         n/a      backup.timer                 backup.service

2 timers listed.`
	timers := ParseTimers(out)
	if len(timers) != 2 {
		t.Fatalf("parsed %d timers, want 2: %+v", len(timers), timers)
	}
	if timers[0].Unit != "logrotate.timer" || timers[0].Activates != "logrotate.service" {
		t.Errorf("timer0 = %+v", timers[0])
	}
	if timers[1].Unit != "backup.timer" {
		t.Errorf("timer1 = %+v", timers[1])
	}
}

func TestValidUser(t *testing.T) {
	for _, u := range []string{"root", "kayla", "www-data", "user_1"} {
		if !ValidUser(u) {
			t.Errorf("%q should be valid", u)
		}
	}
	for _, u := range []string{"", "a b", "root; rm -rf /", "$(whoami)", "../etc"} {
		if ValidUser(u) {
			t.Errorf("%q should be rejected", u)
		}
	}
}
