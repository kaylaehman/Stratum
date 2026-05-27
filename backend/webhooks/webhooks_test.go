package webhooks

import "testing"

func TestPayloadFor(t *testing.T) {
	slack, err := payloadFor("slack", Message{Title: "T", Text: "body"})
	if err != nil {
		t.Fatal(err)
	}
	if got := string(slack); got != `{"text":"*T*\nbody"}` {
		t.Errorf("slack payload = %s", got)
	}
	discord, err := payloadFor("discord", Message{Title: "T", Text: "body"})
	if err != nil {
		t.Fatal(err)
	}
	if got := string(discord); got != `{"content":"**T**\nbody"}` {
		t.Errorf("discord payload = %s", got)
	}
	// Unknown provider falls back to slack format.
	other, _ := payloadFor("teams", Message{Text: "x"})
	if got := string(other); got != `{"text":"x"}` {
		t.Errorf("fallback payload = %s", got)
	}
}

func TestRateLimit(t *testing.T) {
	d := New(nil)
	if !d.allow("wh1", "port.new") {
		t.Fatal("first send should be allowed")
	}
	if d.allow("wh1", "port.new") {
		t.Error("second send within window should be blocked")
	}
	// Different trigger / webhook is independent.
	if !d.allow("wh1", "container.crash") {
		t.Error("different trigger should be allowed")
	}
	if !d.allow("wh2", "port.new") {
		t.Error("different webhook should be allowed")
	}
}

func TestValidProvider(t *testing.T) {
	if !ValidProvider("slack") || !ValidProvider("discord") {
		t.Error("slack/discord should be valid")
	}
	if ValidProvider("teams") || ValidProvider("") {
		t.Error("teams/empty should be invalid")
	}
}
