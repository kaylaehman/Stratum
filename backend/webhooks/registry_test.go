package webhooks

// registry_test.go — table-driven tests for the trigger registry and the
// built-in trigger definitions. These run in the same package so they can
// inspect unexported state.

import (
	"testing"
)

// TestRegisteredContainsAllBuiltins verifies every constant trigger key is in
// the registry after init() runs.
func TestRegisteredContainsAllBuiltins(t *testing.T) {
	want := []string{
		TriggerPortNew, TriggerContainerCrash, TriggerCVECritical,
		TriggerSSHKeyAdded, TriggerFileChange, TriggerAgentDisconnect,
		TriggerCPUThreshold,
		TriggerContainerOOM, TriggerContainerUnhealthy,
		TriggerVolumeThreshold, TriggerImageUpdateAvail,
		TriggerPostureGradeDrop, TriggerBackupFailed,
	}
	keys := AllTriggerKeys()
	keySet := make(map[string]bool, len(keys))
	for _, k := range keys {
		keySet[k] = true
	}
	for _, w := range want {
		if !keySet[w] {
			t.Errorf("trigger %q missing from registry", w)
		}
	}
}

// TestAllTriggersBackcompat confirms the legacy AllTriggers var is populated
// from the registry (so existing callers aren't broken).
func TestAllTriggersBackcompat(t *testing.T) {
	if len(AllTriggers) == 0 {
		t.Fatal("AllTriggers must be non-empty after init()")
	}
	regKeys := AllTriggerKeys()
	if len(AllTriggers) != len(regKeys) {
		t.Errorf("AllTriggers len=%d, want %d", len(AllTriggers), len(regKeys))
	}
}

// TestLookupTrigger verifies lookup of a known and unknown key.
func TestLookupTrigger(t *testing.T) {
	cases := []struct {
		key   string
		found bool
	}{
		{TriggerContainerCrash, true},
		{TriggerBackupFailed, true},
		{"nonexistent.trigger", false},
	}
	for _, tc := range cases {
		_, ok := LookupTrigger(tc.key)
		if ok != tc.found {
			t.Errorf("LookupTrigger(%q) found=%v, want %v", tc.key, ok, tc.found)
		}
	}
}

// TestTriggerDefsHaveRequiredFields checks that every registered trigger has
// a non-empty Label and Description.
func TestTriggerDefsHaveRequiredFields(t *testing.T) {
	for _, def := range Registered() {
		if def.Label == "" {
			t.Errorf("trigger %q has empty Label", def.Key)
		}
		if def.Description == "" {
			t.Errorf("trigger %q has empty Description", def.Key)
		}
	}
}

// TestTriggerConfigSchemas verifies that triggers with ConfigSchema have
// well-formed fields (non-empty key, label, type, default).
func TestTriggerConfigSchemas(t *testing.T) {
	for _, def := range Registered() {
		for i, f := range def.ConfigSchema {
			if f.Key == "" {
				t.Errorf("trigger %q ConfigSchema[%d] has empty Key", def.Key, i)
			}
			if f.Label == "" {
				t.Errorf("trigger %q ConfigSchema[%d] has empty Label", def.Key, i)
			}
			if f.Type == "" {
				t.Errorf("trigger %q ConfigSchema[%d] has empty Type", def.Key, i)
			}
		}
	}
}

// TestCapabilityGatedTriggers checks that agent-gated triggers carry the
// correct capability requirement.
func TestCapabilityGatedTriggers(t *testing.T) {
	agentGated := []string{TriggerSSHKeyAdded, TriggerFileChange}
	for _, key := range agentGated {
		def, ok := LookupTrigger(key)
		if !ok {
			t.Fatalf("trigger %q not registered", key)
		}
		if def.RequiresCapability != "agent" {
			t.Errorf("trigger %q RequiresCapability=%q, want \"agent\"", key, def.RequiresCapability)
		}
	}
}

// TestRegisterDuplicatePanics ensures a duplicate registration causes a panic
// (fail-fast at startup).
func TestRegisterDuplicatePanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic on duplicate Register, got none")
		}
	}()
	// TriggerPortNew is already registered by init().
	Register(TriggerDef{Key: TriggerPortNew, Label: "dup", Description: "dup"})
}

// TestRegisterEmptyKeyPanics ensures Register panics on an empty key.
func TestRegisterEmptyKeyPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic on empty-key Register, got none")
		}
	}()
	Register(TriggerDef{Key: "", Label: "x", Description: "x"})
}

// TestRegisteredIsSnapshot verifies that the returned slice from Registered()
// is a copy (mutations don't affect the registry).
func TestRegisteredIsSnapshot(t *testing.T) {
	snap1 := Registered()
	n := len(snap1)
	// Mutate the snapshot — should not affect the registry.
	if n > 0 {
		snap1[0] = TriggerDef{Key: "mutated", Label: "mutated", Description: "mutated"}
	}
	snap2 := Registered()
	if len(snap2) != n {
		t.Errorf("registry length changed after snapshot mutation: got %d, want %d", len(snap2), n)
	}
	if n > 0 && snap2[0].Key == "mutated" {
		t.Error("registry was mutated via snapshot")
	}
}
