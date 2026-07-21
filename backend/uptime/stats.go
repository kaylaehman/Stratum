package uptime

import (
	"time"

	"github.com/KAE-Labs/stratum/backend/db"
)

// UptimePct computes the percentage of up/degraded results within a window.
// A degraded result counts as up for uptime purposes.
// Returns 100.0 if there are no results in the window.
func UptimePct(results []db.UptimeResult, from time.Time) float64 {
	var total, up int
	for _, r := range results {
		if r.CheckedAt.Before(from) {
			continue
		}
		total++
		if r.Status == string(StatusUp) || r.Status == string(StatusDegraded) {
			up++
		}
	}
	if total == 0 {
		return 100.0
	}
	return float64(up) / float64(total) * 100.0
}

// AvgResponseMs computes the average response time across all results.
// Returns 0 if there are no results.
func AvgResponseMs(results []db.UptimeResult) float64 {
	if len(results) == 0 {
		return 0
	}
	var sum int64
	for _, r := range results {
		sum += int64(r.ResponseTimeMs)
	}
	return float64(sum) / float64(len(results))
}

// ComputeStats builds Stats from a pre-fetched result slice.
func ComputeStats(monitorID string, results []db.UptimeResult) Stats {
	now := time.Now()
	current := StatusDown
	if len(results) > 0 {
		newest := results[0]
		for _, r := range results[1:] {
			if r.CheckedAt.After(newest.CheckedAt) {
				newest = r
			}
		}
		current = CheckStatus(newest.Status)
	}
	return Stats{
		MonitorID:  monitorID,
		Current:    current,
		Uptime24h:  UptimePct(results, now.Add(-24*time.Hour)),
		Uptime7d:   UptimePct(results, now.Add(-7*24*time.Hour)),
		Uptime30d:  UptimePct(results, now.Add(-30*24*time.Hour)),
		AvgRespMs:  AvgResponseMs(results),
	}
}
