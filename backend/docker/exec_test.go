package docker

import "testing"

func TestParseContainerStat(t *testing.T) {
	raw := "directory|4096|755|0|0|root|root|1700000000|config\n" +
		"regular file|1234|640|1000|1000|node|node|1700000100|app.conf\n" +
		"symbolic link|11|777|0|0|root|root|1700000200|link\n" +
		"\n" + // blank line skipped
		"garbage line without delimiters\n" + // skipped
		"regular file|5|644|0|0|root|root|1700000300|weird|name\n" // name contains "|"

	entries := parseContainerStat(raw)
	if len(entries) != 4 {
		t.Fatalf("got %d entries, want 4: %+v", len(entries), entries)
	}

	if entries[0].Name != "config" || entries[0].Type != "directory" || entries[0].PermOctal != "755" {
		t.Errorf("dir entry wrong: %+v", entries[0])
	}
	if entries[1].Size != 1234 || entries[1].UID != 1000 || entries[1].Owner != "node" || entries[1].ModUnix != 1700000100 {
		t.Errorf("file entry wrong: %+v", entries[1])
	}
	if entries[2].Type != "symbolic link" {
		t.Errorf("symlink entry wrong: %+v", entries[2])
	}
	// A name containing "|" must be kept whole (SplitN with 9 fields).
	if entries[3].Name != "weird|name" {
		t.Errorf("name with pipe = %q, want weird|name", entries[3].Name)
	}
}

func TestParseContainerStat_Empty(t *testing.T) {
	if got := parseContainerStat(""); len(got) != 0 {
		t.Errorf("empty input = %+v, want none", got)
	}
}
