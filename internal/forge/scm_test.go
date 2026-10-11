package forge

import "testing"

func TestIssueOf(t *testing.T) {
	for _, c := range [][3]string{{"PIN-12 fix parser", "x", "PIN-12"}, {"fix", "feature/pin-7-parser", "PIN-7"}, {"fix", "main", ""}} {
		if got := IssueOf(c[0], c[1]); got != c[2] {
			t.Errorf("%q %q: %q", c[0], c[1], got)
		}
	}
}
