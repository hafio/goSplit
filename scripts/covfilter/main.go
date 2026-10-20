// Command covfilter drops the code marked "cov:ignore" from a Go coverage
// profile, so the cov task's total measures only code a test can check. The
// cov task in scripts/dev.sh and scripts/dev.ps1 runs it between go test and
// go tool cover; docs/development.md (Testing) says what may be marked.
//
// A marker is a line comment with a reason:
//
//	if err != nil { // cov:ignore store-fault: needs a broken database
//
// At the end of a line that opens a block -- an if or else body, a case, a
// loop, a closure, a function -- it excludes that block with everything nested
// in it (the rightmost one when several open there). On a line that opens no
// block it excludes the coverage blocks that start on that line. A marker
// alone on its line applies to the next line, so the last line of a doc
// comment covers the whole function.
//
// A marker without a reason, or one that excludes nothing, fails the run; one
// whose code the tests already cover is reported, so it can be dropped.
package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// main reports run's error and exits non-zero.
// cov:ignore glue: a wrapper around run, which the tests call directly
func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "covfilter:", err)
		os.Exit(1)
	}
}

const usage = "usage: covfilter -in <profile> -out <filtered profile> [-root <module root>]"

// run filters the profile named by -in into -out, then prints the total
// before exclusions, what was excluded, and every marker the tests have made
// unnecessary.
func run(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("covfilter", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	in := fs.String("in", "", "profile written by go test -coverprofile")
	out := fs.String("out", "", "where to write the filtered profile")
	root := fs.String("root", ".", "module root, where go.mod is")
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("%w -- %s", err, usage)
	}
	if *in == "" || *out == "" || fs.NArg() > 0 {
		return errors.New(usage)
	}

	module, err := modulePath(filepath.Join(*root, "go.mod"))
	if err != nil {
		return err
	}
	data, err := os.ReadFile(*in)
	if err != nil {
		return fmt.Errorf("read the profile: %w -- run go test -coverprofile first", err)
	}
	mode, blocks, err := parseProfile(string(data))
	if err != nil {
		return fmt.Errorf("%s: %w", *in, err)
	}
	files, order := units(blocks)
	excluded, stale, err := applyMarkers(*root, module, files, order)
	if err != nil {
		return err
	}
	if err := writeProfile(*out, mode, blocks, excluded); err != nil {
		return err
	}
	report(stdout, files, excluded, stale)
	return nil
}

// modulePath returns the module path from go.mod; the profile names every
// file by it.
func modulePath(gomod string) (string, error) {
	data, err := os.ReadFile(gomod)
	if err != nil {
		return "", fmt.Errorf("read %s: %w -- run covfilter from the repository root, or pass -root", gomod, err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		if f := strings.Fields(line); len(f) >= 2 && f[0] == "module" {
			return strings.Trim(f[1], `"`), nil
		}
	}
	return "", fmt.Errorf("%s has no module line", gomod)
}

// pos is a 1-based line and byte column, counted the same way by the profile
// and by go/token.
type pos struct{ line, col int }

func (p pos) before(q pos) bool {
	return p.line < q.line || p.line == q.line && p.col < q.col
}

// span is where a block sits. The same span is listed once per test binary
// that runs it.
type span struct {
	file       string // module path + "/" + slash-separated file path
	start, end pos
}

// block is one line of a coverage profile.
type block struct {
	span
	stmts int
	count int64
	text  string // the line as read, written back unchanged when kept
}

// parseProfile reads the "mode:" line and the blocks after it, each
// "file:line.col,line.col statements count".
func parseProfile(data string) (string, []block, error) {
	lines := strings.Split(strings.ReplaceAll(data, "\r\n", "\n"), "\n")
	mode, ok := strings.CutPrefix(lines[0], "mode: ")
	if !ok {
		return "", nil, fmt.Errorf("line 1 is %q, want \"mode: <set|count|atomic>\"", lines[0])
	}
	var blocks []block
	for i, line := range lines[1:] {
		if line == "" {
			continue
		}
		b, err := parseBlock(line)
		if err != nil {
			return "", nil, fmt.Errorf("line %d: %w", i+2, err)
		}
		blocks = append(blocks, b)
	}
	return mode, blocks, nil
}

func parseBlock(line string) (block, error) {
	b := block{text: line}
	colon := strings.LastIndex(line, ":")
	if colon < 1 {
		return b, fmt.Errorf("%q is not a coverage block", line)
	}
	b.file = line[:colon]
	if _, err := fmt.Sscanf(line[colon+1:], "%d.%d,%d.%d %d %d",
		&b.start.line, &b.start.col, &b.end.line, &b.end.col, &b.stmts, &b.count); err != nil {
		return b, fmt.Errorf("%q is not a coverage block: %w", line, err)
	}
	return b, nil
}

// unit is one distinct span with its statements, covered when any test
// binary ran it.
type unit struct {
	span
	stmts   int
	covered bool
}

// units merges repeated spans and groups them by file, in profile order.
func units(blocks []block) (map[string][]*unit, []string) {
	byFile := map[string][]*unit{}
	seen := map[span]*unit{}
	var order []string
	for _, b := range blocks {
		if u, ok := seen[b.span]; ok {
			u.covered = u.covered || b.count > 0
			continue
		}
		u := &unit{span: b.span, stmts: b.stmts, covered: b.count > 0}
		seen[b.span] = u
		if byFile[b.file] == nil {
			order = append(order, b.file)
		}
		byFile[b.file] = append(byFile[b.file], u)
	}
	return byFile, order
}

// marker is one cov:ignore comment, resolved to what it covers.
type marker struct {
	line   int // of the comment, for messages
	reason string
	target int  // the line it applies to
	opens  bool // a block opens on target: [from, to] is that block
	from   pos
	to     pos
}

// covers reports whether u is part of what m excludes.
func (m marker) covers(u *unit) bool {
	if m.opens {
		return !u.start.before(m.from) && !m.to.before(u.start)
	}
	return u.start.line == m.target
}

// applyMarkers resolves the markers in every profiled file and returns the
// spans they exclude, plus the markers whose code is fully covered. A marker
// without a reason or that excludes nothing is an error; they are listed
// together, so one run reports every fix.
func applyMarkers(root, module string, files map[string][]*unit, order []string) (map[span]bool, []string, error) {
	excluded := map[span]bool{}
	var problems, stale []string
	for _, file := range order {
		rel, ok := strings.CutPrefix(file, module+"/")
		if !ok {
			return nil, nil, fmt.Errorf("the profile names %s, which is outside module %s -- pass the module root as -root", file, module)
		}
		marks, err := markers(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			return nil, nil, err
		}
		for _, m := range marks {
			where := fmt.Sprintf("%s:%d", rel, m.line)
			if m.reason == "" {
				problems = append(problems, where+": add a reason, e.g. // cov:ignore store-fault: needs a broken database")
				continue
			}
			hit, covered := 0, 0
			for _, u := range files[file] {
				if m.covers(u) {
					excluded[u.span] = true
					hit++
					if u.covered {
						covered++
					}
				}
			}
			switch {
			case hit == 0:
				problems = append(problems, where+": excludes nothing -- put it at the end of the line that opens the block (if, else, case or func) or alone on the line above it")
			case covered == hit:
				stale = append(stale, where+": the tests cover everything this marker excludes; drop it")
			}
		}
	}
	if len(problems) > 0 {
		return nil, nil, fmt.Errorf("%d cov:ignore marker(s) to fix:\n  %s", len(problems), strings.Join(problems, "\n  "))
	}
	return excluded, stale, nil
}

// opener is a block that opens on a line: a body from its braces, or a case
// clause from its colon to its last statement.
type opener struct{ from, to pos }

// markers parses one source file and resolves its cov:ignore comments. A file
// that never mentions the word is not parsed.
func markers(path string) ([]marker, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w -- the profile does not match the source tree; re-run go test -coverprofile", path, err)
	}
	if !bytes.Contains(src, []byte("cov:ignore")) {
		return nil, nil
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, src, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}

	opens := map[int]opener{}
	ast.Inspect(f, func(n ast.Node) bool {
		var from, to token.Pos
		switch n := n.(type) {
		case *ast.BlockStmt:
			from, to = n.Lbrace, n.Rbrace
		case *ast.CaseClause:
			from, to = n.Colon, n.End()
		case *ast.CommClause:
			from, to = n.Colon, n.End()
		default:
			return true
		}
		a, b := fset.Position(from), fset.Position(to)
		if o, ok := opens[a.Line]; !ok || a.Column > o.from.col {
			opens[a.Line] = opener{pos{a.Line, a.Column}, pos{b.Line, b.Column}}
		}
		return true
	})

	lines := bytes.Split(src, []byte("\n"))
	var marks []marker
	for _, g := range f.Comments {
		for _, c := range g.List {
			reason, ok := directive(c.Text)
			if !ok {
				continue
			}
			p := fset.Position(c.Slash)
			m := marker{line: p.Line, reason: reason, target: p.Line}
			if len(bytes.TrimSpace(lines[p.Line-1][:p.Column-1])) == 0 {
				m.target++ // alone on its line: it applies to the next one
			}
			if o, ok := opens[m.target]; ok {
				m.opens, m.from, m.to = true, o.from, o.to
			}
			marks = append(marks, m)
		}
	}
	return marks, nil
}

// directive returns the reason of a "// cov:ignore <reason>" comment, and
// false for any other comment, /* */ ones included.
func directive(text string) (string, bool) {
	body, ok := strings.CutPrefix(text, "//")
	if !ok {
		return "", false
	}
	rest, ok := strings.CutPrefix(strings.TrimSpace(body), "cov:ignore")
	if !ok || rest != "" && rest[0] != ' ' && rest[0] != '\t' {
		return "", false
	}
	return strings.TrimSpace(rest), true
}

// writeProfile writes the profile back without the excluded blocks.
func writeProfile(path, mode string, blocks []block, excluded map[span]bool) error {
	var b strings.Builder
	b.WriteString("mode: " + mode + "\n")
	for _, bl := range blocks {
		if !excluded[bl.span] {
			b.WriteString(bl.text + "\n")
		}
	}
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		return fmt.Errorf("write the filtered profile: %w", err)
	}
	return nil
}

// report prints the total before exclusions and what the markers removed.
// The cov task prints the total after them, which is the floor.
func report(w io.Writer, files map[string][]*unit, excluded map[span]bool, stale []string) {
	var total, covered, outStmts, outBlocks int
	for _, us := range files {
		for _, u := range us {
			total += u.stmts
			if u.covered {
				covered += u.stmts
			}
			if excluded[u.span] {
				outStmts += u.stmts
				outBlocks++
			}
		}
	}
	pct := 0.0
	if total > 0 {
		pct = 100 * float64(covered) / float64(total)
	}
	fmt.Fprintf(w, "before cov:ignore exclusions: %.1f%% of %d statements\n", pct, total)
	fmt.Fprintf(w, "excluded by cov:ignore: %d statements in %d blocks\n", outStmts, outBlocks)
	for _, s := range stale {
		fmt.Fprintln(w, "warn:", s)
	}
}
