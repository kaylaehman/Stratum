package api

import "testing"

func TestSafeDeployDir(t *testing.T) {
	// Empty dir derives a sanitized default under /opt.
	if got, ok := safeDeployDir("", "my stack!"); !ok || got != "/opt/stratum-stacks/my-stack-" {
		t.Errorf("default dir = %q ok=%v", got, ok)
	}

	allowed := []string{"/opt/stacks/x", "/srv/app", "/home/kayla/stacks", "/var/lib/x", "/mnt/data/x", "/opt"}
	for _, d := range allowed {
		if _, ok := safeDeployDir(d, "n"); !ok {
			t.Errorf("dir %q should be allowed", d)
		}
	}

	rejected := []string{
		"/etc/cron.d",       // host config root not allowlisted
		"/root/x",           // not allowlisted
		"relative/path",     // not absolute
		"/opt/../etc",       // traversal
		"/optfoo/x",         // prefix lookalike, not under /opt/
		"/usr/local/x",      // not allowlisted
		"",                  // handled above, but empty name+dir still ok — skip
	}
	for _, d := range rejected {
		if d == "" {
			continue
		}
		if _, ok := safeDeployDir(d, "n"); ok {
			t.Errorf("dir %q should be rejected", d)
		}
	}
}
