package storage

import (
	"database/sql"
	"fmt"
	"time"
)

// AppStat holds aggregated usage for a single application.
type AppStat struct {
	AppName      string
	ProcessName  string
	TotalSeconds int64
	Sessions     int
}

// DayStat holds aggregated usage for a single day.
type DayStat struct {
	Date         time.Time
	TotalSeconds int64
	TopApps      []AppStat
}

// InsertSession opens a new tracking session and returns its row ID.
func InsertSession(db *sql.DB, appName, processName, windowTitle string, startedAt int64) (int64, error) {
	result, err := db.Exec(
		`INSERT INTO sessions (app_name, process_name, window_title, started_at) VALUES (?, ?, ?, ?)`,
		appName, processName, windowTitle, startedAt,
	)
	if err != nil {
		return 0, fmt.Errorf("inserting session: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("getting session id: %w", err)
	}

	return id, nil
}

// CloseSession marks a session as ended and computes its duration.
func CloseSession(db *sql.DB, id int64, endedAt int64) error {
	_, err := db.Exec(
		`UPDATE sessions SET ended_at = ?, duration_sec = ? - started_at WHERE id = ?`,
		endedAt, endedAt, id,
	)
	if err != nil {
		return fmt.Errorf("closing session %d: %w", id, err)
	}
	return nil
}

// GetDailyStats returns the top apps for the given day, sorted by total time descending.
func GetDailyStats(db *sql.DB, date time.Time) ([]AppStat, error) {
	year, month, day := date.Date()
	loc := date.Location()
	dayStart := time.Date(year, month, day, 0, 0, 0, 0, loc).Unix()
	dayEnd := time.Date(year, month, day, 23, 59, 59, 0, loc).Unix()

	rows, err := db.Query(`
		SELECT app_name, process_name, 
		       COALESCE(SUM(duration_sec), 0) AS total_sec,
		       COUNT(*) AS session_count
		FROM sessions
		WHERE started_at BETWEEN ? AND ?
		  AND duration_sec IS NOT NULL
		GROUP BY app_name, process_name
		ORDER BY total_sec DESC
	`, dayStart, dayEnd)
	if err != nil {
		return nil, fmt.Errorf("querying daily stats: %w", err)
	}
	defer rows.Close()

	var stats []AppStat
	for rows.Next() {
		var s AppStat
		if err := rows.Scan(&s.AppName, &s.ProcessName, &s.TotalSeconds, &s.Sessions); err != nil {
			return nil, fmt.Errorf("scanning daily stat row: %w", err)
		}
		stats = append(stats, s)
	}

	return stats, nil
}

// GetWeeklyStats returns per-day totals for the last 7 days, each with its top apps.
func GetWeeklyStats(db *sql.DB) ([]DayStat, error) {
	now := time.Now()
	loc := now.Location()
	var days []DayStat

	for i := 6; i >= 0; i-- {
		d := now.AddDate(0, 0, -i)
		year, month, day := d.Date()
		dayStart := time.Date(year, month, day, 0, 0, 0, 0, loc).Unix()
		dayEnd := time.Date(year, month, day, 23, 59, 59, 0, loc).Unix()

		rows, err := db.Query(`
			SELECT app_name, process_name,
			       COALESCE(SUM(duration_sec), 0) AS total_sec,
			       COUNT(*) AS session_count
			FROM sessions
			WHERE started_at BETWEEN ? AND ?
			  AND duration_sec IS NOT NULL
			GROUP BY app_name, process_name
			ORDER BY total_sec DESC
		`, dayStart, dayEnd)
		if err != nil {
			return nil, fmt.Errorf("querying weekly stats for day %d: %w", i, err)
		}

		var apps []AppStat
		var dayTotal int64
		for rows.Next() {
			var s AppStat
			if err := rows.Scan(&s.AppName, &s.ProcessName, &s.TotalSeconds, &s.Sessions); err != nil {
				rows.Close()
				return nil, fmt.Errorf("scanning weekly stat row: %w", err)
			}
			dayTotal += s.TotalSeconds
			apps = append(apps, s)
		}
		rows.Close()

		days = append(days, DayStat{
			Date:         time.Date(year, month, day, 0, 0, 0, 0, loc),
			TotalSeconds: dayTotal,
			TopApps:      apps,
		})
	}

	return days, nil
}

// DeleteOldSessions removes all sessions older than 6 months.
// Called once on app startup, then every 24 hours via a ticker in main.go.
func DeleteOldSessions(db *sql.DB) error {
	cutoff := time.Now().AddDate(0, -6, 0).Unix()
	_, err := db.Exec(`DELETE FROM sessions WHERE started_at < ?`, cutoff)
	if err != nil {
		return fmt.Errorf("deleting old sessions: %w", err)
	}
	// VACUUM reclaims freed disk space — mandatory after bulk deletes in SQLite
	_, err = db.Exec(`VACUUM`)
	return err
}
