package tracker

import (
	"context"
	"database/sql"
	"log"
	"strings"
	"time"

	"github.com/focustrack/focustrack/internal/config"
	"github.com/focustrack/focustrack/internal/notify"
	"github.com/focustrack/focustrack/internal/storage"
)

// session tracks the currently active app session for DB persistence.
type session struct {
	id      int64
	appName string
}

// Manage listens for tracker events and persists sessions to the database.
// It batches writes by only flushing every 5 ticks (5 seconds) to avoid
// hammering disk. Runs until ctx is cancelled.
func Manage(ctx context.Context, db *sql.DB, events <-chan Event) {
	var current *session
	tickCount := 0

	for {
		select {
		case <-ctx.Done():
			// Close the current session on shutdown
			if current != nil {
				closeSession(db, current)
			}
			return

		case ev, ok := <-events:
			if !ok {
				// Channel closed — tracker stopped
				if current != nil {
					closeSession(db, current)
				}
				return
			}

			tickCount++

			// App changed — close old session, open new one
			if current == nil || current.appName != ev.AppName {
				if current != nil {
					closeSession(db, current)
					go CheckAndNotifyLimits(db)
				}

				// Only open a new session if the event has a valid app
				if ev.AppName != "" {
					id, err := storage.InsertSession(
						db, ev.AppName, ev.ProcessName,
						ev.WindowTitle, ev.Timestamp.Unix(),
					)
					if err != nil {
						log.Printf("warn: failed to insert session: %v", err)
						current = nil
						continue
					}
					current = &session{id: id, appName: ev.AppName}
				} else {
					current = nil
				}
			}

			// Every 5 ticks, update the current session's end time
			// so duration stays fresh even if the app doesn't change
			if tickCount >= 5 && current != nil {
				tickCount = 0
				if err := storage.CloseSession(db, current.id, time.Now().Unix()); err != nil {
					log.Printf("warn: failed to update session: %v", err)
				}
				// Note: We used to reopen a new session here, but now we just update
				// the existing one to avoid row explosion. storage.CloseSession
				// correctly updates ended_at and duration_sec for the row.
			}

		}
	}
}

// closeSession marks the current session as ended in the database.
func closeSession(db *sql.DB, s *session) {
	if err := storage.CloseSession(db, s.id, time.Now().Unix()); err != nil {
		log.Printf("warn: failed to close session %d: %v", s.id, err)
	}
}

// notifiedToday prevents duplicate limit notifications per app per day.
// Resets automatically on process restart (which happens at most once per day).
var notifiedToday = make(map[string]bool)

// CheckAndNotifyLimits compares today's usage against configured limits and
// fires a toast notification the first time an app crosses its daily limit.
func CheckAndNotifyLimits(db *sql.DB) {
	cfg, err := config.Load()
	if err != nil || cfg == nil || !cfg.LimitAlerts {
		return
	}
	if len(cfg.AppLimits) == 0 {
		return
	}

	stats, err := storage.GetDailyStats(db, time.Now())
	if err != nil {
		return
	}

	for _, stat := range stats {
		limitMin, ok := cfg.AppLimits[strings.ToLower(stat.AppName)]
		if !ok {
			continue
		}
		usedMin := int(stat.TotalSeconds / 60)
		key := stat.AppName + ":" + time.Now().Format("2006-01-02")
		if usedMin >= limitMin && !notifiedToday[key] {
			notifiedToday[key] = true
			go notify.SendLimitAlert(stat.AppName, limitMin)
		}
	}
}
