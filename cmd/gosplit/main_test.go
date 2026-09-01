package main

import "testing"

func TestWantsVersion(t *testing.T) {
	cases := []struct {
		args []string
		want bool
	}{
		{[]string{"gosplit", "--version"}, true},
		{[]string{"gosplit", "-version"}, true},
		{[]string{"gosplit", "version"}, true},
		{[]string{"gosplit"}, false},
		{[]string{}, false},
		{[]string{"gosplit", "serve"}, false},
		{[]string{"gosplit", "--help"}, false},
		{[]string{"gosplit", "serve", "--version"}, false},
	}
	for _, c := range cases {
		if got := wantsVersion(c.args); got != c.want {
			t.Errorf("wantsVersion(%q)=%v want %v", c.args, got, c.want)
		}
	}
}

// The default must stay non-empty: an unstamped binary has to report something
// obviously wrong rather than an empty line the release check would misread.
func TestVersionDefault(t *testing.T) {
	if version == "" {
		t.Fatal("version is empty")
	}
}
