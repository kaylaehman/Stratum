package logtail

import (
	"strings"
	"testing"
)

// collectLines is a helper that collects emitted LogLines via a lineWriter,
// feeding input as byte slices, and returns the collected slice.
func collectLines(t *testing.T, inputs [][]byte) []LogLine {
	t.Helper()
	var got []LogLine
	w := newLineWriter("ctr", "stdout", func(ll LogLine) {
		got = append(got, ll)
	})
	for _, chunk := range inputs {
		n, err := w.Write(chunk)
		if err != nil {
			t.Fatalf("Write error: %v", err)
		}
		if n != len(chunk) {
			t.Fatalf("Write returned %d, want %d", n, len(chunk))
		}
	}
	w.flush()
	return got
}

// TestLineWriter_SimpleLines feeds two complete newline-terminated lines in one
// Write and expects exactly two LogLines emitted.
func TestLineWriter_SimpleLines(t *testing.T) {
	input := []byte("line one\nline two\n")
	got := collectLines(t, [][]byte{input})

	if len(got) != 2 {
		t.Fatalf("got %d lines, want 2", len(got))
	}
	if got[0].Text != "line one" {
		t.Errorf("got[0].Text = %q", got[0].Text)
	}
	if got[1].Text != "line two" {
		t.Errorf("got[1].Text = %q", got[1].Text)
	}
}

// TestLineWriter_SplitAcrossWrites feeds a single line split across two Writes.
func TestLineWriter_SplitAcrossWrites(t *testing.T) {
	got := collectLines(t, [][]byte{
		[]byte("hel"),
		[]byte("lo\n"),
	})

	if len(got) != 1 {
		t.Fatalf("got %d lines, want 1", len(got))
	}
	if got[0].Text != "hello" {
		t.Errorf("got[0].Text = %q", got[0].Text)
	}
}

// TestLineWriter_PartialLineFlush feeds a line without a trailing newline;
// flush should emit it.
func TestLineWriter_PartialLineFlush(t *testing.T) {
	got := collectLines(t, [][]byte{[]byte("no newline here")})

	if len(got) != 1 {
		t.Fatalf("got %d lines, want 1", len(got))
	}
	if got[0].Text != "no newline here" {
		t.Errorf("got[0].Text = %q", got[0].Text)
	}
}

// TestLineWriter_OversizeLine_NoNewline feeds a line that exceeds MaxLineBytes
// without ever encountering a newline. The writer must emit a truncated LogLine
// and must NOT buffer more than MaxLineBytes bytes at any point.
func TestLineWriter_OversizeLine_NoNewline(t *testing.T) {
	var emitted []LogLine
	w := newLineWriter("ctr", "stdout", func(ll LogLine) {
		emitted = append(emitted, ll)
	})

	big := strings.Repeat("A", MaxLineBytes+500)
	n, err := w.Write([]byte(big))
	if err != nil {
		t.Fatalf("Write error: %v", err)
	}
	if n != len(big) {
		t.Fatalf("Write returned %d, want %d", n, len(big))
	}
	w.flush()

	if len(emitted) == 0 {
		t.Fatal("expected at least one LogLine emitted")
	}
	first := emitted[0]
	if !first.Truncated {
		t.Error("expected Truncated=true for oversize line without newline")
	}
	if len(first.Text) > MaxLineBytes {
		t.Errorf("Text length %d exceeds MaxLineBytes", len(first.Text))
	}
}

// TestLineWriter_OversizeLine_ThenNormal feeds an oversize line followed by a
// normal line separated by '\n'. The oversize portion must be emitted as
// truncated; the continuation must be discarded (overflow skip); the next line
// must be emitted clean.
func TestLineWriter_OversizeLine_ThenNormal(t *testing.T) {
	var emitted []LogLine
	w := newLineWriter("ctr", "stdout", func(ll LogLine) {
		emitted = append(emitted, ll)
	})

	// Oversize part — fills the buffer and triggers truncation.
	big := strings.Repeat("B", MaxLineBytes+1)
	// Write with a newline at the end to terminate the big line, then a clean line.
	payload := big + "\nnormal line\n"
	_, _ = w.Write([]byte(payload))
	w.flush()

	// Expect exactly 2 lines: truncated + normal.
	if len(emitted) != 2 {
		t.Fatalf("got %d lines, want 2: %v", len(emitted), emitted)
	}
	if !emitted[0].Truncated {
		t.Error("first line should be truncated")
	}
	if emitted[1].Text != "normal line" {
		t.Errorf("second line Text = %q, want %q", emitted[1].Text, "normal line")
	}
	if emitted[1].Truncated {
		t.Error("second line should NOT be truncated")
	}
}

// TestLineWriter_MultipleChunksPartialLines exercises the common real-world
// scenario where log bytes arrive in small arbitrary chunks.
func TestLineWriter_MultipleChunksPartialLines(t *testing.T) {
	chunks := [][]byte{
		[]byte("fir"),
		[]byte("st\nsec"),
		[]byte("ond\n"),
		[]byte("third"),
	}
	got := collectLines(t, chunks)

	if len(got) != 3 {
		t.Fatalf("got %d lines, want 3", len(got))
	}
	want := []string{"first", "second", "third"}
	for i, w := range want {
		if got[i].Text != w {
			t.Errorf("got[%d].Text = %q, want %q", i, got[i].Text, w)
		}
	}
}

// TestLineWriter_EmptyLines handles consecutive newlines (empty lines).
func TestLineWriter_EmptyLines(t *testing.T) {
	got := collectLines(t, [][]byte{[]byte("a\n\nb\n")})

	if len(got) != 3 {
		t.Fatalf("got %d lines, want 3 (a, empty, b)", len(got))
	}
	if got[0].Text != "a" {
		t.Errorf("got[0].Text = %q", got[0].Text)
	}
	if got[1].Text != "" {
		t.Errorf("got[1].Text = %q, want empty", got[1].Text)
	}
	if got[2].Text != "b" {
		t.Errorf("got[2].Text = %q", got[2].Text)
	}
}
