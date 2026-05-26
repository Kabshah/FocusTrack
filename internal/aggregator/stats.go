package aggregator

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/focustrack/focustrack/internal/storage"
)

// DailyReport holds formatted stats for a single day.
type DailyReport struct {
	Date           time.Time
	TotalSeconds   int64
	Apps           []storage.AppStat
	FormattedTotal string
}

// WeeklyReport holds formatted stats for the last 7 days.
type WeeklyReport struct {
	Days           []storage.DayStat
	WeekTotal      int64
	FormattedTotal string
}

// GetDailyReport computes today's usage stats from the database.
func GetDailyReport(db *sql.DB) (DailyReport, error) {
	now := time.Now()
	apps, err := storage.GetDailyStats(db, now)
	if err != nil {
		return DailyReport{}, fmt.Errorf("getting daily report: %w", err)
	}

	var total int64
	for _, a := range apps {
		total += a.TotalSeconds
	}

	return DailyReport{
		Date:           now,
		TotalSeconds:   total,
		Apps:           apps,
		FormattedTotal: FormatDuration(total),
	}, nil
}

// GetWeeklyReport computes the last 7 days of usage stats from the database.
func GetWeeklyReport(db *sql.DB) (WeeklyReport, error) {
	days, err := storage.GetWeeklyStats(db)
	if err != nil {
		return WeeklyReport{}, fmt.Errorf("getting weekly report: %w", err)
	}

	var total int64
	for _, d := range days {
		total += d.TotalSeconds
	}

	return WeeklyReport{
		Days:           days,
		WeekTotal:      total,
		FormattedTotal: FormatDuration(total),
	}, nil
}

// FormatDuration converts seconds into a human-readable "Xh Ym" string.
func FormatDuration(seconds int64) string {
	if seconds <= 0 {
		return "0m"
	}

	hours := seconds / 3600
	mins := (seconds % 3600) / 60

	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, mins)
	}
	return fmt.Sprintf("%dm", mins)
}
