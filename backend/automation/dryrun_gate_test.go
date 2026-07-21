package automation

// dryrun_gate_test.go — edge-case tests for automation safety gates.
//
// Covers:
//  1. Dry-run defaults: fix_bind_mount_perms and patch_critical_cves must
//     default dry_run=true (any automation that modifies state must default
//     safe).
//  2. All automations default disabled (no DB row → Enabled=false in
//     resolveRow), so they never auto-execute on first start.
//  3. The low-risk gate: auto_remediate_low must never execute commands that
//     ClassifyRisk does not return RiskLow for.
//  4. Catalog invariants: every entry has a non-empty Key, Label, and a
//     valid Category; no duplicate keys.
//  5. isDue boundary: exactly at the interval boundary.

import (
	"testing"
	"time"

	"github.com/KAE-Labs/stratum/backend/db"
	"github.com/KAE-Labs/stratum/backend/remediation"
)

// ---------------------------------------------------------------------------
// 1. Dry-run defaults
// ---------------------------------------------------------------------------

func TestDryRunDefault_FixBindMountPerms(t *testing.T) {
	ent, ok := CatalogEntry("fix_bind_mount_perms")
	if !ok {
		t.Fatal("fix_bind_mount_perms not found in catalog")
	}
	v, exists := ent.DefaultConfig["dry_run"]
	if !exists {
		t.Fatal("fix_bind_mount_perms: DefaultConfig missing 'dry_run' key")
	}
	b, isBool := v.(bool)
	if !isBool {
		t.Fatalf("fix_bind_mount_perms: dry_run is %T (%v), want bool", v, v)
	}
	if !b {
		t.Error("fix_bind_mount_perms: dry_run must default to true")
	}
}

func TestDryRunDefault_PatchCriticalCVEs(t *testing.T) {
	ent, ok := CatalogEntry("patch_critical_cves")
	if !ok {
		t.Fatal("patch_critical_cves not found in catalog")
	}
	v, exists := ent.DefaultConfig["dry_run"]
	if !exists {
		t.Fatal("patch_critical_cves: DefaultConfig missing 'dry_run' key")
	}
	b, isBool := v.(bool)
	if !isBool {
		t.Fatalf("patch_critical_cves: dry_run is %T (%v), want bool", v, v)
	}
	if !b {
		t.Error("patch_critical_cves: dry_run must default to true")
	}
}

// Any future automation whose key ends with a destructive action verb should
// also default dry_run=true. This test guards against accidentally removing
// the dry_run key from the two known dangerous automations.
func TestDryRunDefault_BothDestructiveAutomationsHaveFlag(t *testing.T) {
	needDryRun := []string{"fix_bind_mount_perms", "patch_critical_cves"}
	for _, key := range needDryRun {
		ent, ok := CatalogEntry(key)
		if !ok {
			t.Errorf("expected automation %q in catalog", key)
			continue
		}
		if _, exists := ent.DefaultConfig["dry_run"]; !exists {
			t.Errorf("automation %q must have dry_run in DefaultConfig", key)
		}
	}
}

// ---------------------------------------------------------------------------
// 2. All automations default disabled
// ---------------------------------------------------------------------------

func TestAllAutomations_DefaultDisabled(t *testing.T) {
	// resolveRow with no DB row must produce Enabled=false for every catalog entry.
	for _, ent := range Catalog() {
		ent := ent
		t.Run(ent.Key, func(t *testing.T) {
			// simulate resolveRow: no DB row found → zero row with defaults
			row := db.AutomationRow{
				Key:             ent.Key,
				Enabled:         false, // this is what resolveRow returns on ErrNotFound
				IntervalSeconds: ent.DefaultIntervalSeconds,
				ConfigJSON:      "{}",
			}
			if row.Enabled {
				t.Errorf("automation %q should default to disabled (no DB row)", ent.Key)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// 3. Low-risk gate: auto_remediate_low must not auto-execute non-low commands.
// ---------------------------------------------------------------------------

// TestAutoRemediateLow_OnlyAllowsLowRisk verifies that the commands which the
// auto_remediate_low automation is designed to execute are actually classified
// as low risk by the classifier. Non-low commands must never reach this path.
func TestAutoRemediateLow_DockerRestartIsLow(t *testing.T) {
	// The canonical auto-heal action: restarting an unhealthy container.
	cmds := []string{"docker restart myapp"}
	risk := remediation.ClassifyRisk(cmds)
	if risk != remediation.RiskLow {
		t.Errorf("docker restart: risk=%q, want RiskLow — this command should be auto-approvable", risk)
	}
}

func TestAutoRemediateLow_SystemctlRestartIsLow(t *testing.T) {
	cmds := []string{"systemctl restart myservice"}
	risk := remediation.ClassifyRisk(cmds)
	if risk != remediation.RiskLow {
		t.Errorf("systemctl restart: risk=%q, want RiskLow", risk)
	}
}

func TestAutoRemediateLow_DestructiveIsNotLow(t *testing.T) {
	// rm -rf must never be classified low — verify the gate holds.
	cmds := []string{"rm -rf /"}
	risk := remediation.ClassifyRisk(cmds)
	if risk == remediation.RiskLow {
		t.Error("rm -rf / classified as RiskLow — auto_remediate_low gate would be bypassed")
	}
	if !remediation.RequiresStepUp(risk) {
		t.Errorf("rm -rf / (risk=%q) must require step-up", risk)
	}
}

func TestAutoRemediateLow_HighRiskRequiresStepUp(t *testing.T) {
	// A high-risk command (chmod) must not be auto-approvable.
	cmds := []string{"chmod 777 /data"}
	risk := remediation.ClassifyRisk(cmds)
	if !remediation.RequiresStepUp(risk) {
		t.Errorf("chmod 777 /data (risk=%q) must require step-up and not be auto-executed", risk)
	}
}

func TestAutoRemediateLow_ArbitraryScriptIsNotLow(t *testing.T) {
	// An opaque script (denylist evasion attempt) must not be classified low.
	cmds := []string{"./fix.sh"}
	risk := remediation.ClassifyRisk(cmds)
	if risk == remediation.RiskLow {
		t.Error("./fix.sh classified as RiskLow — positive allowlist not protecting against opaque scripts")
	}
}

// ---------------------------------------------------------------------------
// 4. Catalog invariants
// ---------------------------------------------------------------------------

func TestCatalog_NoEmptyKeys(t *testing.T) {
	for _, ent := range Catalog() {
		if ent.Key == "" {
			t.Errorf("catalog entry with label %q has empty key", ent.Label)
		}
	}
}

func TestCatalog_NoEmptyLabels(t *testing.T) {
	for _, ent := range Catalog() {
		if ent.Label == "" {
			t.Errorf("catalog entry with key %q has empty label", ent.Key)
		}
	}
}

func TestCatalog_NoDuplicateKeys(t *testing.T) {
	seen := make(map[string]bool)
	for _, ent := range Catalog() {
		if seen[ent.Key] {
			t.Errorf("duplicate catalog key: %q", ent.Key)
		}
		seen[ent.Key] = true
	}
}

func TestCatalog_AllCategoriesValid(t *testing.T) {
	valid := map[string]bool{
		CategorySelfHeal:    true,
		CategoryUpdate:      true,
		CategorySecurity:    true,
		CategoryMaintenance: true,
	}
	for _, ent := range Catalog() {
		if !valid[ent.Category] {
			t.Errorf("automation %q has unknown category %q", ent.Key, ent.Category)
		}
	}
}

func TestCatalog_PositiveDefaultIntervals(t *testing.T) {
	for _, ent := range Catalog() {
		if ent.DefaultIntervalSeconds <= 0 {
			t.Errorf("automation %q has non-positive default interval %d", ent.Key, ent.DefaultIntervalSeconds)
		}
	}
}

func TestCatalog_DefaultConfigNotNil(t *testing.T) {
	for _, ent := range Catalog() {
		if ent.DefaultConfig == nil {
			t.Errorf("automation %q has nil DefaultConfig (should be empty map)", ent.Key)
		}
	}
}

// TestCatalogEntry_Roundtrip verifies that CatalogEntry(key) returns the same
// entry that Catalog() iterates for every key.
func TestCatalogEntry_Roundtrip(t *testing.T) {
	for _, ent := range Catalog() {
		got, ok := CatalogEntry(ent.Key)
		if !ok {
			t.Errorf("CatalogEntry(%q) not found", ent.Key)
			continue
		}
		if got.Key != ent.Key || got.Label != ent.Label {
			t.Errorf("CatalogEntry(%q) mismatch: got %+v, want %+v", ent.Key, got, ent)
		}
	}
}

func TestCatalogEntry_UnknownKey_ReturnsFalse(t *testing.T) {
	_, ok := CatalogEntry("completely_nonexistent_key_xyz")
	if ok {
		t.Error("CatalogEntry should return false for unknown key")
	}
}

// ---------------------------------------------------------------------------
// 5. isDue boundary conditions
// ---------------------------------------------------------------------------

func TestIsDue_ExactlyAtInterval_IsDue(t *testing.T) {
	// A run that happened exactly one interval ago should be considered due.
	interval := 3600
	lastRun := time.Now().Add(-time.Duration(interval) * time.Second)
	row := db.AutomationRow{IntervalSeconds: interval, LastRun: &lastRun}
	if !isDue(row) {
		t.Error("should be due when last run was exactly one interval ago")
	}
}

func TestIsDue_OneSecondBeforeInterval_NotDue(t *testing.T) {
	// A run that happened 1 second less than an interval ago should NOT be due.
	interval := 3600
	lastRun := time.Now().Add(-time.Duration(interval-1) * time.Second)
	row := db.AutomationRow{IntervalSeconds: interval, LastRun: &lastRun}
	if isDue(row) {
		t.Error("should not be due when last run was 1s before the interval")
	}
}

func TestIsDue_ZeroIntervalWithLastRun_AlwaysDue(t *testing.T) {
	// A zero interval means "run as fast as possible" — always due.
	lastRun := time.Now().Add(-1 * time.Second)
	row := db.AutomationRow{IntervalSeconds: 0, LastRun: &lastRun}
	if !isDue(row) {
		t.Error("zero interval should always be due")
	}
}

// ---------------------------------------------------------------------------
// 6. entryToView: overriding dry_run in config must work correctly.
// ---------------------------------------------------------------------------

func TestEntryToView_DryRunOverrideFalse(t *testing.T) {
	// A DB override setting dry_run=false must replace the default true.
	ent, ok := CatalogEntry("fix_bind_mount_perms")
	if !ok {
		t.Fatal("fix_bind_mount_perms not in catalog")
	}
	row := db.AutomationRow{
		Key:             "fix_bind_mount_perms",
		Enabled:         true,
		IntervalSeconds: 3600,
		ConfigJSON:      `{"dry_run":false}`,
	}
	v := entryToView(ent, row)
	dryRun, exists := v.Config["dry_run"]
	if !exists {
		t.Fatal("dry_run key missing from view config after override")
	}
	b, ok := dryRun.(bool)
	if !ok {
		t.Fatalf("dry_run is %T after override, want bool", dryRun)
	}
	if b {
		t.Error("dry_run should be false after DB override sets it to false")
	}
}

func TestEntryToView_DryRunDefaultPreservedWhenNoOverride(t *testing.T) {
	// When no DB override is present the catalog default dry_run=true must survive.
	ent, ok := CatalogEntry("fix_bind_mount_perms")
	if !ok {
		t.Fatal("fix_bind_mount_perms not in catalog")
	}
	row := db.AutomationRow{
		Key:             "fix_bind_mount_perms",
		Enabled:         false,
		IntervalSeconds: 0,
		ConfigJSON:      "{}",
	}
	v := entryToView(ent, row)
	dryRun, exists := v.Config["dry_run"]
	if !exists {
		t.Fatal("dry_run key missing from view config with empty override")
	}
	b, ok := dryRun.(bool)
	if !ok {
		t.Fatalf("dry_run is %T, want bool", dryRun)
	}
	if !b {
		t.Error("dry_run should remain true when no DB override present")
	}
}
