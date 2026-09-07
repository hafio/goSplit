package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/hafio/gosplit/internal/backup"
	"github.com/hafio/gosplit/internal/config"
	"github.com/hafio/gosplit/internal/store"
)

// The CLI subcommands run as their own process against a database a live
// server may still be serving, so they deliberately do NOT go through
// store.Open: that migrates unconditionally, and a bare `gosplit backup` --
// typically run right before a version upgrade -- would then silently apply
// pending migrations as a side effect of taking a backup. They use
// store.OpenNoMigrate instead, and `restore --force` says plainly when the
// schema does not match.

// backupCommands are the subcommand names dispatch recognizes.
var backupCommands = map[string]bool{"backup": true, "restore": true, "inspect": true, "paths": true}

// wantsBackup reports which backup subcommand args names, if any. It is
// separate from wantsVersion so that function and its tests stay untouched.
func wantsBackup(args []string) (string, bool) {
	if len(args) < 2 {
		return "", false
	}
	cmd := args[1]
	if backupCommands[cmd] {
		return cmd, true
	}
	return "", false
}

// runBackupCommand executes a backup subcommand. cmd is one of the names
// wantsBackup recognizes; rest is everything after it.
func runBackupCommand(cmd string, rest []string) error {
	switch cmd {
	case "backup":
		return runBackupCLI(rest)
	case "restore":
		return runRestoreCLI(rest)
	case "inspect":
		return runInspectCLI(rest)
	case "paths":
		return runPathsCLI(rest)
	default:
		return fmt.Errorf("unknown subcommand %q", cmd)
	}
}

// cliContext gives a subcommand a cancellable context so Ctrl-C during a long
// restore rolls the transaction back rather than leaving it hanging.
func cliContext() (context.Context, func()) {
	return signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
}

// loadCLIConfig loads configuration and stamps the build version, exactly as
// the server does, so an archive records the tag it was written by.
func loadCLIConfig() (*config.Config, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	cfg.AppVersion = version
	return cfg, nil
}

func runBackupCLI(args []string) error {
	fs := flag.NewFlagSet("backup", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	out := fs.String("o", "", "directory to write the archive into (defaults to BACKUP_DIR)")
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: gosplit backup [-o DIR]\n\nWrites a sealed archive of every table plus the upload tree.\nThe archive is encrypted with a key derived from SESSION_SECRET.\n\n")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg, err := loadCLIConfig()
	if err != nil {
		return err
	}
	dir := *out
	if dir == "" {
		dir = cfg.BackupDir
	}

	ctx, stop := cliContext()
	defer stop()

	st, err := store.OpenNoMigrate(ctx, cfg)
	if err != nil {
		return err
	}
	defer func() { _ = st.Close() }()

	if err := warnPendingMigrations(ctx, st); err != nil {
		return err
	}

	r := backup.New(st, cfg)
	path, man, err := r.DumpToDir(ctx, dir, backup.ArchivePrefix)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "wrote %s\n", path)
	printManifest(man)
	return nil
}

func runRestoreCLI(args []string) error {
	fs := flag.NewFlagSet("restore", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	file := fs.String("file", "", "archive to restore (required)")
	force := fs.Bool("force", false, "actually perform the restore; without it, the archive is only validated")
	noPreDump := fs.Bool("no-pre-dump", false, "skip the pre-restore safety dump of current data (not recommended)")
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: gosplit restore --file ARCHIVE [--force] [--no-pre-dump]\n\nWithout --force the archive is validated and summarized, and nothing is changed.\nWith --force EVERY TABLE IS EMPTIED and replaced by the archive's contents, and\nthe upload directory is replaced. Rows keep their original ids.\n\n")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *file == "" {
		fs.Usage()
		return errors.New("restore: --file is required")
	}

	cfg, err := loadCLIConfig()
	if err != nil {
		return err
	}

	// An upload swap interrupted by a crash leaves a recognizable state on
	// disk. Repair it here as well as on server boot: a CLI restore killed
	// mid-swap would otherwise stay broken until someone started the server.
	if err := backup.RecoverIncompleteSwap(cfg); err != nil {
		return err
	}

	ctx, stop := cliContext()
	defer stop()

	// Validation only reads, so it uses the non-migrating handle. A confirmed
	// restore needs the live schema to match the archive anyway, and Validate
	// is what enforces that -- so there is still no reason to migrate here.
	st, err := store.OpenNoMigrate(ctx, cfg)
	if err != nil {
		return err
	}
	defer func() { _ = st.Close() }()

	r := backup.New(st, cfg)
	v, err := r.Validate(ctx, *file)
	if err != nil {
		return err
	}
	defer v.Discard()

	fmt.Fprintf(os.Stderr, "archive: %s\n", *file)
	printManifest(v.Manifest)

	if !*force {
		fmt.Fprintf(os.Stderr, "\nThis archive is valid and matches this database's schema.\nNothing has been changed. Re-run with --force to replace ALL current data.\n")
		return nil
	}

	if cfg.Engine != config.EnginePostgres {
		fmt.Fprintf(os.Stderr, "\nnote: on SQLite the restore holds the database's only connection for its\nduration, so a running server will stall until it finishes. Treat this as a\nmaintenance window.\n")
	}
	fmt.Fprintf(os.Stderr, "\nrestoring...\n")

	rep, err := r.Apply(ctx, v, backup.ApplyOptions{Confirmed: true, SkipSafetyDump: *noPreDump})
	if err != nil {
		return err
	}
	if rep.SafetyDumpPath != "" {
		fmt.Fprintf(os.Stderr, "pre-restore safety dump: %s\n", rep.SafetyDumpPath)
	}
	fmt.Fprintf(os.Stderr, "restored %d tables and %d upload files\n", len(rep.RowsRestored), rep.UploadsRestored)
	return nil
}

func runInspectCLI(args []string) error {
	fs := flag.NewFlagSet("inspect", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: gosplit inspect ARCHIVE\n\nPrints an archive's plaintext header. Needs no database and no matching\nSESSION_SECRET, so it works on any archive file.\n\n")
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		fs.Usage()
		return errors.New("inspect: exactly one archive path is required")
	}

	// Deliberately no config.Load: inspect must work without SESSION_SECRET,
	// and without a database.
	hdr, err := backup.InspectFile(fs.Arg(0))
	if err != nil {
		return err
	}
	fmt.Printf("format version:   %d\n", hdr.FormatVersion)
	fmt.Printf("created at:       %s\n", hdr.CreatedAt)
	fmt.Printf("source engine:    %s\n", hdr.SourceEngine)
	fmt.Printf("gosplit version:  %s\n", orDash(hdr.GosplitVersion))
	fmt.Printf("encryption:       %s (key from SESSION_SECRET via %s)\n", hdr.AEADAlgo, hdr.KDFAlgo)
	fmt.Printf("key fingerprint:  %s\n", hdr.KeyFingerprintB64)
	fmt.Printf("migrations:       %s\n", orDash(strings.Join(hdr.SchemaMigrationsHint, ", ")))
	return nil
}

// warnPendingMigrations refuses to back up a database the running binary would
// migrate. Backing it up as-is would produce an archive whose recorded
// migration set no longer matches the schema the app expects, and migrating it
// silently would mutate live data as a side effect of a read-only-sounding
// command. Neither is acceptable, so the operator is told to start the server
// once first.
func warnPendingMigrations(ctx context.Context, st *store.Store) error {
	pending, err := st.PendingMigrations(ctx)
	if err != nil {
		return err
	}
	if len(pending) == 0 {
		return nil
	}
	return fmt.Errorf("this database is missing %d migration(s) this build applies (%s). Start the server once so they are applied, then take the backup -- `gosplit backup` deliberately does not migrate, because that would change live data as a side effect of a backup", len(pending), strings.Join(pending, ", "))
}

func printManifest(m backup.Manifest) {
	fmt.Fprintf(os.Stderr, "created at:      %s\n", m.CreatedAt)
	fmt.Fprintf(os.Stderr, "source engine:   %s\n", m.SourceEngine)
	fmt.Fprintf(os.Stderr, "gosplit version: %s\n", orDash(m.GosplitVersion))
	fmt.Fprintf(os.Stderr, "uploads:         %d files, %s\n", m.UploadFileCount, humanBytes(m.UploadTotalBytes))
	fmt.Fprintf(os.Stderr, "tables:\n")
	for _, t := range m.Tables {
		fmt.Fprintf(os.Stderr, "  %-32s %d\n", t.Name, t.RowCount)
	}
}

func orDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "-"
	}
	return s
}

func humanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	v := float64(n)
	for _, suffix := range []string{"KiB", "MiB", "GiB", "TiB"} {
		v /= unit
		if v < unit {
			return fmt.Sprintf("%.1f %s", v, suffix)
		}
	}
	return fmt.Sprintf("%.1f PiB", v)
}
