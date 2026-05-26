//go:generate rsrc -manifest ../../assets/FocusTrack.manifest -ico ../../assets/FocusTrack.ico -o FocusTrack.syso

package main

import (
	"context"
	"log"
	"time"

	"github.com/focustrack/focustrack/internal/storage"
	"github.com/focustrack/focustrack/internal/tracker"
	"github.com/focustrack/focustrack/ui"
)

func main() {
	// 1. Open and migrate database.
	db, err := storage.Open(storage.DefaultPath())
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()

	if err := storage.Migrate(db); err != nil {
		log.Fatalf("migrate db: %v", err)
	}

	// 2. Run initial cleanup, then schedule every 24h.
	if err := storage.DeleteOldSessions(db); err != nil {
		log.Printf("warn: initial cleanup: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		t := time.NewTicker(24 * time.Hour)
		defer t.Stop()
		for {
			select {
			case <-t.C:
				if err := storage.DeleteOldSessions(db); err != nil {
					log.Printf("warn: scheduled cleanup: %v", err)
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	// 3. Start tracker and session manager.
	tr := tracker.New()
	events := tr.Start(ctx)
	go tracker.Manage(ctx, db, events)

	// 4. Start UI — blocks until app exits.
	ui.SyncStartupState()
	ui.Run(db)
}
