package agent

import (
	"testing"

	"github.com/kaylaehman/stratum/backend/db"
	stratumv1 "github.com/kaylaehman/stratum/proto/gen/stratum/v1"
)

// dbNode is a test helper that builds a minimal db.Node from a host string.
func dbNode(host string) db.Node {
	return db.Node{Host: host}
}

func TestProtoEventTypeToDB(t *testing.T) {
	cases := []struct {
		in   stratumv1.FileEventType
		want string
	}{
		{stratumv1.FileEventType_FILE_EVENT_TYPE_CREATE, "create"},
		{stratumv1.FileEventType_FILE_EVENT_TYPE_MODIFY, "modified"},
		{stratumv1.FileEventType_FILE_EVENT_TYPE_DELETE, "delete"},
		{stratumv1.FileEventType_FILE_EVENT_TYPE_RENAME, "rename"},
		{stratumv1.FileEventType_FILE_EVENT_TYPE_ATTRIB, "attrib"},
		{stratumv1.FileEventType_FILE_EVENT_TYPE_UNSPECIFIED, "modified"},
	}

	for _, tc := range cases {
		got := protoEventTypeToDB(tc.in)
		if got != tc.want {
			t.Errorf("protoEventTypeToDB(%v) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestAgentAddr(t *testing.T) {
	cases := []struct {
		host string
		want string
	}{
		{"192.168.1.10", "192.168.1.10:7750"},
		{"agent.local", "agent.local:7750"},
		{"", ""},
	}

	for _, tc := range cases {
		got := agentAddr(dbNode(tc.host))
		if got != tc.want {
			t.Errorf("agentAddr(%q) = %q, want %q", tc.host, got, tc.want)
		}
	}
}
