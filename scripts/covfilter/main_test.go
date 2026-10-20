package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeModule lays out module example.com/m under a temp root: a go.mod plus
// files, which may replace the go.mod.
func writeModule(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	all := map[string]string{"go.mod": "module example.com/m\n\ngo 1.27\n"}
	for name, body := range files {
		all[name] = body
	}
	for name, body := range all {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// covRun runs covfilter over profile in a module made of files, and returns
// what it printed and the filtered profile ("" when none was written).
func covRun(t *testing.T, files map[string]string, profile string) (stdout, filtered string, err error) {
	t.Helper()
	root := writeModule(t, files)
	in, out := filepath.Join(root, "coverage.raw.out"), filepath.Join(root, "coverage.out")
	if werr := os.WriteFile(in, []byte(profile), 0o644); werr != nil {
		t.Fatal(werr)
	}
	var buf strings.Builder
	err = run([]string{"-in", in, "-out", out, "-root", root}, &buf)
	if data, rerr := os.ReadFile(out); rerr == nil {
		filtered = string(data)
	}
	return buf.String(), filtered, err
}

// marked has a marker in each placement. The profile in TestRunDropsMarkedCode
// points at its lines, so keep the two in step.
const marked = `package p

import "errors"

func f(x int) (int, error) {
	if x < 0 { // cov:ignore impossible: callers pass lengths
		return 0, errors.New("negative")
	}
	switch x {
	case 1: // cov:ignore log-only: nothing to assert
		x = 2
	default:
		x++
	}
	if x > 5 { x = 5 } else { x = 4 } // cov:ignore fixture: the rightmost block wins
	return x, nil
}

// g is wiring.
// cov:ignore glue: proven end to end
func g(c chan int) int {
	select {
	case v := <-c:
		return v
	default:
		return 0
	}
}

func h() int {
	if g(nil) > 0 {
		return 1
	}
	return 2 // cov:ignore fixture: no block opens on this line
}
`

// TestRunDropsMarkedCode: each placement excludes exactly its code -- an if
// body, a case, the rightmost of two blocks on one line, a whole function from
// its doc comment, and the block starting on a line that opens none -- while
// unmarked code stays, covered or not.
func TestRunDropsMarkedCode(t *testing.T) {
	entries := []struct {
		pos  string
		keep bool
	}{
		{"6.2,6.11 1 1", true},
		{"7.3,7.35 1 0", false}, // the if body marked on line 6
		{"9.2,9.11 1 1", true},
		{"11.3,11.8 1 0", false}, // case 1, marked on its own line
		{"13.3,13.6 1 1", true},
		{"15.2,15.11 1 1", true},
		{"15.13,15.18 1 1", true},
		{"15.28,15.33 1 0", false}, // the else body, rightmost on line 15
		{"16.2,16.15 1 1", true},
		{"22.2,22.9 1 0", false}, // g, marked from its doc comment
		{"24.3,24.11 1 0", false},
		{"26.3,26.11 1 0", false},
		{"31.2,31.16 1 1", true},
		{"32.3,32.11 1 0", true}, // uncovered but unmarked: it still counts
		{"34.2,34.10 1 0", false},
	}
	profile, kept := "mode: atomic\n", "mode: atomic\n"
	for _, e := range entries {
		line := "example.com/m/p/p.go:" + e.pos + "\n"
		profile += line
		if e.keep {
			kept += line
		}
	}

	stdout, filtered, err := covRun(t, map[string]string{"p/p.go": marked}, profile)
	if err != nil {
		t.Fatal(err)
	}
	if filtered != kept {
		t.Errorf("filtered profile:\n%s\nwant:\n%s", filtered, kept)
	}
	want := "before cov:ignore exclusions: 46.7% of 15 statements\nexcluded by cov:ignore: 7 statements in 7 blocks\n"
	if stdout != want {
		t.Errorf("stdout = %q, want %q", stdout, want)
	}
}

// TestRunReportsStaleMarker: a marker on code the tests now reach still
// excludes it, and is reported so it can be dropped.
func TestRunReportsStaleMarker(t *testing.T) {
	src := "package q\n\nfunc k(x int) int {\n\tif x > 0 { // cov:ignore fixture: the tests reach this\n\t\treturn 1\n\t}\n\treturn 0\n}\n"
	profile := "mode: set\nexample.com/m/q/q.go:4.2,4.11 1 1\nexample.com/m/q/q.go:5.3,5.11 1 1\nexample.com/m/q/q.go:7.2,7.10 1 1\n"
	stdout, filtered, err := covRun(t, map[string]string{"q/q.go": src}, profile)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(filtered, ":5.3,") {
		t.Error("the marked block was kept")
	}
	if !strings.Contains(stdout, "warn: q/q.go:4: the tests cover everything this marker excludes; drop it") {
		t.Errorf("no stale-marker warning in:\n%s", stdout)
	}
}

// TestRunCountsARepeatedBlockOnce: a block listed by two test binaries is one
// block, covered if either ran it, and both lines stay for go tool cover to
// merge.
func TestRunCountsARepeatedBlockOnce(t *testing.T) {
	src := "package q\n\nfunc k() int {\n\treturn 1\n}\n"
	profile := "mode: atomic\nexample.com/m/q/q.go:4.2,4.10 1 0\nexample.com/m/q/q.go:4.2,4.10 1 3\n"
	stdout, filtered, err := covRun(t, map[string]string{"q/q.go": src}, profile)
	if err != nil {
		t.Fatal(err)
	}
	if filtered != profile {
		t.Errorf("filtered profile = %q, want the input unchanged", filtered)
	}
	if !strings.HasPrefix(stdout, "before cov:ignore exclusions: 100.0% of 1 statements\n") {
		t.Errorf("stdout = %q", stdout)
	}
}

// TestRunAcceptsAnEmptyProfile: a profile without blocks reports zero instead
// of dividing by it.
func TestRunAcceptsAnEmptyProfile(t *testing.T) {
	stdout, filtered, err := covRun(t, nil, "mode: set\n")
	if err != nil {
		t.Fatal(err)
	}
	if filtered != "mode: set\n" || !strings.Contains(stdout, "0.0% of 0 statements") {
		t.Errorf("filtered = %q, stdout = %q", filtered, stdout)
	}
}

// TestRunRejectsBadMarkers: a marker without a reason and one that excludes
// nothing are reported together, and no filtered profile is written.
func TestRunRejectsBadMarkers(t *testing.T) {
	src := "package q\n\nimport \"fmt\" // cov:ignore fixture: nothing opens or starts here\n\nfunc k() {\n\tif true { // cov:ignore\n\t\tfmt.Println()\n\t}\n}\n"
	profile := "mode: set\nexample.com/m/q/q.go:6.2,6.10 1 1\nexample.com/m/q/q.go:7.3,7.16 1 1\n"
	_, filtered, err := covRun(t, map[string]string{"q/q.go": src}, profile)
	if err == nil {
		t.Fatal("bad markers were accepted")
	}
	for _, want := range []string{"2 cov:ignore marker(s) to fix", "q/q.go:3: excludes nothing", "q/q.go:6: add a reason"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err, want)
		}
	}
	if filtered != "" {
		t.Errorf("a failed run wrote a filtered profile:\n%s", filtered)
	}
}

// TestRunRejectsBadInput: a profile or source tree covfilter cannot trust
// fails with the reason, and nothing is written.
func TestRunRejectsBadInput(t *testing.T) {
	pkg := map[string]string{"p/p.go": "package p\n\nfunc k() int {\n\treturn 1\n}\n"}
	block := "example.com/m/p/p.go:4.2,4.10 1 1\n"
	for _, tc := range []struct {
		name    string
		files   map[string]string
		profile string
		want    string
	}{
		{"no mode line", pkg, block, `want "mode:`},
		{"malformed block", pkg, "mode: set\nexample.com/m/p/p.go:4.2 1\n", "line 2:"},
		{"block without a file", pkg, "mode: set\n4.2,4.10 1 1\n", "is not a coverage block"},
		{"file outside the module", pkg, "mode: set\nother.org/x/p.go:4.2,4.10 1 1\n", "outside module example.com/m"},
		{"file missing from the tree", pkg, "mode: set\nexample.com/m/p/gone.go:4.2,4.10 1 1\n", "does not match the source tree"},
		{"source that does not parse", map[string]string{"p/p.go": "package p // cov:ignore x\n\nfunc {\n"}, "mode: set\n" + block, "parse "},
		{"go.mod without a module line", map[string]string{"go.mod": "go 1.27\n"}, "mode: set\n", "has no module line"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, filtered, err := covRun(t, tc.files, tc.profile)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want it to mention %q", err, tc.want)
			}
			if filtered != "" {
				t.Errorf("a failed run wrote a filtered profile:\n%s", filtered)
			}
		})
	}
}

// TestRunRejectsBadArguments: wrong flags, a missing go.mod or profile, and an
// output path that cannot be written each fail with what to do instead.
func TestRunRejectsBadArguments(t *testing.T) {
	root := writeModule(t, nil)
	in := filepath.Join(root, "coverage.raw.out")
	if err := os.WriteFile(in, []byte("mode: set\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(root, "coverage.out")
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{"unknown flag", []string{"-bogus"}, "flag provided but not defined"},
		{"no output", []string{"-in", in}, "usage: covfilter"},
		{"no go.mod", []string{"-in", in, "-out", out, "-root", t.TempDir()}, "pass -root"},
		{"no profile", []string{"-in", filepath.Join(root, "missing.out"), "-out", out, "-root", root}, "run go test -coverprofile first"},
		{"unwritable output", []string{"-in", in, "-out", filepath.Join(root, "no", "such", "dir", "coverage.out"), "-root", root}, "write the filtered profile"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := run(tc.args, io.Discard)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want it to mention %q", err, tc.want)
			}
		})
	}
}

// TestDirective: only a line comment that starts with the exact word is a
// marker, so prose that mentions it excludes nothing.
func TestDirective(t *testing.T) {
	for _, tc := range []struct {
		text, reason string
		ok           bool
	}{
		{"// cov:ignore store-fault: needs a broken database", "store-fault: needs a broken database", true},
		{"//cov:ignore\tglue", "glue", true},
		{"// cov:ignore", "", true},
		{"// cov:ignored x", "", false},
		{"// cov:ignore-func x", "", false},
		{"// see cov:ignore x", "", false},
		{"/* cov:ignore x */", "", false},
	} {
		reason, ok := directive(tc.text)
		if reason != tc.reason || ok != tc.ok {
			t.Errorf("directive(%q) = %q, %v; want %q, %v", tc.text, reason, ok, tc.reason, tc.ok)
		}
	}
}
