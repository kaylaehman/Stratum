package remediation

import (
	"testing"
)

func TestClassifyRisk(t *testing.T) {
	cases := []struct {
		name     string
		commands []string
		want     string
	}{
		{
			name:     "comment only",
			commands: []string{"# nothing to do"},
			want:     RiskLow,
		},
		{
			name:     "empty command",
			commands: []string{""},
			want:     RiskLow,
		},
		{
			name:     "setfacl surgical",
			commands: []string{"setfacl -m u:1000:r /var/data/file.txt"},
			want:     RiskHigh,
		},
		{
			name:     "chmod non-recursive",
			commands: []string{"chmod o+r /srv/media/config.yml"},
			want:     RiskHigh,
		},
		{
			name:     "chown non-recursive",
			commands: []string{"chown 999 /data/config"},
			want:     RiskHigh,
		},
		{
			name:     "chmod -R recursive",
			commands: []string{"chmod -R 755 /var/www"},
			want:     RiskDestructive,
		},
		{
			name:     "chown -R recursive",
			commands: []string{"chown -R 1000:1000 /config"},
			want:     RiskDestructive,
		},
		{
			name:     "rm -rf",
			commands: []string{"rm -rf /tmp/broken"},
			want:     RiskDestructive,
		},
		{
			name:     "rm -r (no f)",
			commands: []string{"rm -r /var/junk"},
			want:     RiskDestructive,
		},
		{
			name:     "dd",
			commands: []string{"dd if=/dev/zero of=/dev/sda"},
			want:     RiskDestructive,
		},
		{
			name:     "mkfs",
			commands: []string{"mkfs.ext4 /dev/sdb1"},
			want:     RiskDestructive,
		},
		{
			name:     "shutdown",
			commands: []string{"shutdown -h now"},
			want:     RiskDestructive,
		},
		{
			name:     "reboot",
			commands: []string{"reboot"},
			want:     RiskDestructive,
		},
		{
			name:     "poweroff",
			commands: []string{"poweroff"},
			want:     RiskDestructive,
		},
		{
			name:     "highest wins across multiple commands",
			commands: []string{"chmod o+r /etc/config", "chown -R 0:0 /etc"},
			want:     RiskDestructive,
		},
		{
			name:     "generic shell command",
			commands: []string{"systemctl restart nginx"},
			want:     RiskMedium,
		},
		{
			name:     "systemctl stop is high",
			commands: []string{"systemctl stop myservice"},
			want:     RiskHigh,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := ClassifyRisk(tc.commands)
			if got != tc.want {
				t.Errorf("ClassifyRisk(%v) = %q; want %q", tc.commands, got, tc.want)
			}
		})
	}
}

func TestRequiresStepUp(t *testing.T) {
	cases := []struct {
		level string
		want  bool
	}{
		{RiskLow, false},
		{RiskMedium, false},
		{RiskHigh, false},
		{RiskDestructive, true},
	}
	for _, tc := range cases {
		if got := RequiresStepUp(tc.level); got != tc.want {
			t.Errorf("RequiresStepUp(%q) = %v; want %v", tc.level, got, tc.want)
		}
	}
}
