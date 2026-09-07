package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/hafio/gosplit/internal/backup"
	"github.com/hafio/gosplit/internal/config"
	"github.com/hafio/gosplit/internal/store"
)

// `gosplit paths` exists because the runtime image is distroless: no shell, no
// ls, no mkdir. Without it, answering "where is my backup, and why can't the
// app write there" needs a throwaway container with a shell mounted against
// the same volume. This reports the same facts from inside the binary.

// dirStatus is one configured directory and what is actually true about it.
type dirStatus struct {
	Setting  string
	Path     string
	Exists   bool
	Writable bool
	Owner    string // "uid:gid" of the directory, where the OS reports it
	Problem  string
}

func runPathsCLI(args []string) error {
	fs := flag.NewFlagSet("paths", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: gosplit paths\n\nReports every directory this instance writes to, whether it exists and is\nwritable, and the archives found in each. Reads nothing from the database.\n\n")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("paths takes no arguments, got %q", fs.Arg(0))
	}

	cfg, err := loadCLIConfig()
	if err != nil {
		return err
	}

	fmt.Printf("gosplit %s\n", version)
	fmt.Printf("running as uid %d gid %d\n", os.Getuid(), os.Getgid())
	fmt.Printf("engine       %s\n", cfg.Engine)
	fmt.Printf("database     %s\n", cfg.DatabaseURL)
	fmt.Println()

	for _, d := range collectDirStatuses(cfg) {
		printDirStatus(d)
	}

	fmt.Println("A directory must be writable by this uid. On a bind mount the host")
	fmt.Println("directory keeps its own ownership, so fix it on the host with:")
	fmt.Printf("  chown -R %d:%d <host path>\n", os.Getuid(), os.Getgid())
	return nil
}

// collectDirStatuses inspects every directory the app writes to. Nothing is
// created here -- `paths` is a diagnostic and must not change what it reports.
func collectDirStatuses(cfg *config.Config) []dirStatus {
	var out []dirStatus

	if dir := store.SQLiteDataDir(cfg); dir != "" {
		out = append(out, inspectDir("DATABASE_URL", dir))
	}
	out = append(out, inspectDir("UPLOAD_DIR", cfg.UploadDir))
	out = append(out, inspectDir("BACKUP_DIR", cfg.BackupDir))
	if cfg.AutoRestoreDir != "" {
		out = append(out, inspectDir("AUTO_RESTORE_DIR", cfg.AutoRestoreDir))
	} else {
		out = append(out, dirStatus{Setting: "AUTO_RESTORE_DIR", Path: "(not set)"})
	}
	return out
}

// inspectDir reports on one directory without modifying it.
func inspectDir(setting, path string) dirStatus {
	d := dirStatus{Setting: setting, Path: path}
	if path == "" {
		d.Path = "(not set)"
		return d
	}

	info, err := os.Stat(path)
	switch {
	case os.IsNotExist(err):
		d.Problem = "does not exist yet"
		// The parent is what has to allow creating it, so that is what a
		// permission report should be about.
		if perr := probeDir(filepath.Dir(path)); perr != nil {
			d.Problem = fmt.Sprintf("does not exist, and its parent %s is not writable: %v", filepath.Dir(path), perr)
		}
		return d
	case err != nil:
		d.Problem = err.Error()
		return d
	case !info.IsDir():
		d.Problem = "exists but is not a directory"
		return d
	}

	d.Exists = true
	d.Owner = ownerOf(info)
	if perr := probeDir(path); perr != nil {
		d.Problem = perr.Error()
		return d
	}
	d.Writable = true
	return d
}

// probeDir reports whether a new file can be created here, by trying. Mode bits
// alone would not catch a read-only mount or a full filesystem.
func probeDir(path string) error {
	f, err := os.CreateTemp(path, ".gosplit-probe-*")
	if err != nil {
		return err
	}
	name := f.Name()
	_ = f.Close()
	return os.Remove(name)
}

// printDirStatus writes one directory's line, then whatever it holds.
func printDirStatus(d dirStatus) {
	state := "MISSING"
	switch {
	case d.Path == "(not set)":
		state = "unset"
	case d.Exists && d.Writable:
		state = "ok"
	case d.Exists:
		state = "NOT WRITABLE"
	}

	fmt.Printf("%-17s %s\n", d.Setting, d.Path)
	fmt.Printf("%-17s   %s", "", state)
	if d.Owner != "" {
		fmt.Printf("   owner %s", d.Owner)
	}
	fmt.Println()
	if d.Problem != "" {
		fmt.Printf("%-17s   %s\n", "", d.Problem)
	}
	if d.Exists {
		printDirContents(d.Path)
	}
	fmt.Println()
}

// printDirContents lists the archives and the restore marker, which is what an
// operator is actually looking for -- not a full directory listing.
func printDirContents(path string) {
	entries, err := os.ReadDir(path)
	if err != nil {
		fmt.Printf("%-17s   cannot list: %v\n", "", err)
		return
	}

	var lines []string
	marker := false
	others := 0
	for _, e := range entries {
		name := e.Name()
		if name == backup.MarkerName {
			marker = true
			continue
		}
		if e.IsDir() || !strings.HasSuffix(name, backup.ArchiveExt) {
			others++
			continue
		}
		info, err := e.Info()
		if err != nil {
			lines = append(lines, fmt.Sprintf("%s  (cannot stat)", name))
			continue
		}
		lines = append(lines, fmt.Sprintf("%s  %s  %s",
			name, humanBytes(info.Size()), info.ModTime().UTC().Format("2006-01-02 15:04")))
	}
	sort.Strings(lines)

	for _, l := range lines {
		fmt.Printf("%-17s   %s\n", "", l)
	}
	if marker {
		fmt.Printf("%-17s   %s present -- a startup restore has already run here\n", "", backup.MarkerName)
	}
	if len(lines) == 0 && !marker {
		if others > 0 {
			fmt.Printf("%-17s   no archives (%d other entries)\n", "", others)
		} else {
			fmt.Printf("%-17s   empty\n", "")
		}
	}
}
