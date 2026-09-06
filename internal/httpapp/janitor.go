package httpapp

import (
	"context"
	"log/slog"
	"time"

	"github.com/hafio/gosplit/internal/backup"
)

// janitorInterval is how often expired background jobs and abandoned restore
// uploads are swept. Well below the job TTL, so nothing lingers much past it.
const janitorInterval = 5 * time.Minute

// StartJanitor sweeps this process's background-job state until ctx is done.
//
// It exists because an abandoned restore upload holds real resources: the
// uploaded archive under BackupDir plus its extracted staging directory beside
// UploadDir. An admin who uploads an archive, sees the preview, and then walks
// away would otherwise leave both on the data volume permanently.
//
// Deliberately NOT run from the scheduler, and deliberately not behind the
// leader lock. The job tracker is per-process memory, so every instance has to
// sweep its own -- a leader-gated sweep would leave every replica but one
// leaking. It is also independent of BACKUP_CRON, since uploads happen through
// the admin panel whether or not scheduled backups are configured.
func (s *Server) StartJanitor(ctx context.Context) {
	go func() {
		t := time.NewTicker(janitorInterval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case now := <-t.C:
				s.Jobs.Prune(now)
				// Stale .partial files are a dump killed mid-write. Swept here
				// rather than in the scheduler so it happens even with
				// scheduled backups switched off.
				if n := backup.SweepStalePartials(s.Cfg.BackupDir, now); n > 0 {
					slog.Info("janitor: swept stale partial archives", "removed", n)
				}
			}
		}
	}()
}
