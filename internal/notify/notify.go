package notify

import (
	"fmt"
	"log"

	"github.com/focustrack/focustrack/internal/aggregator"
	"github.com/go-toast/toast"
)

const appID = "FocusTrack"

// SendLimitAlert notifies the user that they've exceeded their daily limit for an app.
func SendLimitAlert(appName string, limitMinutes int) error {
	n := toast.Notification{
		AppID:   appID,
		Title:   "⏰ Time Limit Reached",
		Message: fmt.Sprintf("You've used %s for %d minutes today.", appName, limitMinutes),
	}
	if err := n.Push(); err != nil {
		log.Printf("warn: toast notification failed: %v", err)
		return err
	}
	return nil
}

// SendDailySummary notifies the user with a summary of today's screen time.
func SendDailySummary(totalSeconds int64, topApp string) error {
	formatted := aggregator.FormatDuration(totalSeconds)
	n := toast.Notification{
		AppID:   appID,
		Title:   "📊 Daily Screen Time",
		Message: fmt.Sprintf("Total: %s — Most used: %s", formatted, topApp),
	}
	if err := n.Push(); err != nil {
		log.Printf("warn: toast notification failed: %v", err)
		return err
	}
	return nil
}
