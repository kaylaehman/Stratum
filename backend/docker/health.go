package docker

import (
	"context"
	"time"
)

// HealthLogEntry is one healthcheck probe result.
type HealthLogEntry struct {
	Start    time.Time `json:"start"`
	End      time.Time `json:"end"`
	ExitCode int       `json:"exit_code"`
	Output   string    `json:"output"`
}

// HealthReport is a container's healthcheck configuration + recent results.
type HealthReport struct {
	Configured     bool             `json:"configured"`
	Test           []string         `json:"test"`
	IntervalSec    float64          `json:"interval_sec"`
	TimeoutSec     float64          `json:"timeout_sec"`
	StartPeriodSec float64          `json:"start_period_sec"`
	Retries        int              `json:"retries"`
	Status         string           `json:"status"` // healthy | unhealthy | starting | none
	FailingStreak  int              `json:"failing_streak"`
	Log            []HealthLogEntry `json:"log"`
}

// ContainerHealth returns a container's healthcheck config + recent probe
// results from a single inspect.
func (c *Client) ContainerHealth(ctx context.Context, id string) (HealthReport, error) {
	insp, err := c.cli.ContainerInspect(ctx, id)
	if err != nil {
		return HealthReport{}, err
	}
	r := HealthReport{Status: "none"}

	if hc := insp.Config.Healthcheck; hc != nil && len(hc.Test) > 0 && !isDisabledHealthcheck(hc.Test) {
		r.Configured = true
		r.Test = hc.Test
		r.IntervalSec = hc.Interval.Seconds()
		r.TimeoutSec = hc.Timeout.Seconds()
		r.StartPeriodSec = hc.StartPeriod.Seconds()
		r.Retries = hc.Retries
	}

	if insp.State != nil && insp.State.Health != nil {
		h := insp.State.Health
		r.Status = string(h.Status)
		r.FailingStreak = h.FailingStreak
		for _, l := range h.Log {
			if l == nil {
				continue
			}
			r.Log = append(r.Log, HealthLogEntry{Start: l.Start, End: l.End, ExitCode: l.ExitCode, Output: l.Output})
		}
	}
	if r.Log == nil {
		r.Log = []HealthLogEntry{}
	}
	return r, nil
}

// isDisabledHealthcheck reports whether the test explicitly disables the check
// ({"NONE"}).
func isDisabledHealthcheck(test []string) bool {
	return len(test) == 1 && test[0] == "NONE"
}
