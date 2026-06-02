package automation

import "testing"

// TestCatalogHandlerParity is the single-source guard for the automations count:
// every catalog entry must have exactly one handler and vice-versa. This keeps
// the catalog (the one source of truth the /automations page derives from) in
// lock-step with the executable handlers, so the displayed count can never
// silently drift from what actually runs. Passing nil deps is safe — handlers
// are constructed (closures) but never invoked here.
func TestCatalogHandlerParity(t *testing.T) {
	handlers := BuildHandlers(nil, Deps{})
	cat := Catalog()

	if len(handlers) != len(cat) {
		t.Errorf("handler count %d != catalog count %d", len(handlers), len(cat))
	}
	for _, e := range cat {
		if _, ok := handlers[e.Key]; !ok {
			t.Errorf("catalog entry %q has no handler", e.Key)
		}
	}
	for key := range handlers {
		if _, ok := CatalogEntry(key); !ok {
			t.Errorf("handler %q has no catalog entry", key)
		}
	}
}
