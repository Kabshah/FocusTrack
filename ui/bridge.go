package ui

import (
	"database/sql"
	"encoding/json"
	"syscall"

	"github.com/focustrack/focustrack/internal/aggregator"
	webview "github.com/jchv/go-webview2"
)

func registerBridge(w webview.WebView, db *sql.DB) {
	// JS: const data = await window.getDailyStats()
	w.Bind("getDailyStats", func() string {
		report, err := aggregator.GetDailyReport(db)
		if err != nil {
			return `{"error":"` + err.Error() + `"}`
		}
		b, _ := json.Marshal(report)
		return string(b)
	})

	// JS: const data = await window.getWeeklyStats()
	w.Bind("getWeeklyStats", func() string {
		report, err := aggregator.GetWeeklyReport(db)
		if err != nil {
			return `{"error":"` + err.Error() + `"}`
		}
		b, _ := json.Marshal(report)
		return string(b)
	})

	// JS: window.hideWindow()
	w.Bind("hideWindow", func() {
		hwnd := (syscall.Handle)(w.Window())
		if hwnd != 0 {
			showWindow(hwnd, 0) // SW_HIDE = 0
		}
	})
}
