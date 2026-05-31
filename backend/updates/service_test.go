package updates

import (
	"errors"
	"testing"
)

// errFake is a sentinel error used in table tests to simulate fetch failures.
var errFake = errors.New("test error")

func TestClassify(t *testing.T) {
	cases := []struct {
		name          string
		local, remote string
		localErr      error
		remoteErr     error
		wantStatus    string
		wantReason    string // non-empty substring that must appear in unknownReason
	}{
		{
			name:       "matching digests → up_to_date",
			local:      "sha256:aaa", remote: "sha256:aaa",
			wantStatus: StatusUpToDate,
		},
		{
			name:       "differing digests → update_available",
			local:      "sha256:aaa", remote: "sha256:bbb",
			wantStatus: StatusUpdateAvailable,
		},
		{
			name:       "empty local (locally-built) → unknown with reason",
			local:      "", remote: "sha256:bbb",
			wantStatus: StatusUnknown,
			wantReason: "no repo digest",
		},
		{
			name:       "empty remote → unknown with reason",
			local:      "sha256:aaa", remote: "",
			wantStatus: StatusUnknown,
			wantReason: "empty digest",
		},
		{
			name:       "remote error → unknown with reason",
			local:      "sha256:aaa", remote: "sha256:bbb", remoteErr: errFake,
			wantStatus: StatusUnknown,
			wantReason: "registry lookup failed",
		},
		{
			name:       "local error → unknown with reason",
			localErr:   errFake,
			wantStatus: StatusUnknown,
			wantReason: "local digest unavailable",
		},
		{
			name:       "both errors → unknown, local error takes precedence",
			localErr:   errFake, remoteErr: errFake,
			wantStatus: StatusUnknown,
			wantReason: "local digest unavailable",
		},
		{
			name:       "both empty, no errors → unknown (no repo digest)",
			local:      "", remote: "",
			wantStatus: StatusUnknown,
			wantReason: "no repo digest",
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			gotStatus, gotReason := Classify(tc.local, tc.remote, tc.localErr, tc.remoteErr)
			if gotStatus != tc.wantStatus {
				t.Errorf("status = %q, want %q", gotStatus, tc.wantStatus)
			}
			if tc.wantReason != "" && !containsSubstr(gotReason, tc.wantReason) {
				t.Errorf("unknownReason = %q, want it to contain %q", gotReason, tc.wantReason)
			}
			if tc.wantStatus != StatusUnknown && gotReason != "" {
				t.Errorf("non-unknown status should have empty reason, got %q", gotReason)
			}
		})
	}
}

func containsSubstr(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && func() bool {
		for i := 0; i <= len(s)-len(sub); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	}())
}
