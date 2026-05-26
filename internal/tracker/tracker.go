package tracker

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Event represents a snapshot of the active window at a point in time.
type Event struct {
	AppName     string
	ProcessName string
	WindowTitle string
	Timestamp   time.Time
}

// Tracker polls the active window every second and emits events.
// It does NOT write to the DB directly — it sends events via a channel.
type Tracker struct {
	events chan Event
}

// blocklist contains system processes that should be ignored.
var blocklist = map[string]bool{
	"explorer":                true,
	"SearchHost":              true,
	"ShellExperienceHost":     true,
	"StartMenuExperienceHost": true,
	"TextInputHost":           true,
	"LockApp":                 true,
}

// New creates a new Tracker with an internal event channel.
func New() *Tracker {
	return &Tracker{
		events: make(chan Event, 64),
	}
}

// Start begins polling the active window every second.
// Returns a read-only channel that emits window focus events.
// Stops when the context is cancelled.
func (t *Tracker) Start(ctx context.Context) <-chan Event {
	go func() {
		defer close(t.events)
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				ev, err := t.activeWindow()
				if err != nil {
					// Log but don't crash — transient failures are normal
					continue
				}
				// Skip empty events (desktop focused) and blocklisted processes
				if ev.ProcessName == "" || blocklist[ev.AppName] {
					continue
				}
				select {
				case t.events <- ev:
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	return t.events
}

var (
	modUser32                    = windows.NewLazySystemDLL("user32.dll")
	procGetForegroundWindow      = modUser32.NewProc("GetForegroundWindow")
	procGetWindowTextW           = modUser32.NewProc("GetWindowTextW")
	procGetWindowTextLengthW     = modUser32.NewProc("GetWindowTextLengthW")
	procGetWindowThreadProcessId = modUser32.NewProc("GetWindowThreadProcessId")
)

// activeWindow returns the currently focused window's app name, process, and title.
func (t *Tracker) activeWindow() (Event, error) {
	// Step 1: Get foreground window handle
	hwnd, _, _ := procGetForegroundWindow.Call()
	if hwnd == 0 {
		// Desktop has focus or no window — return empty event, not an error
		return Event{Timestamp: time.Now()}, nil
	}

	// Step 2: Get window title
	title := getWindowText(hwnd)

	// Step 3: Get process ID from window handle
	var pid uint32
	procGetWindowThreadProcessId.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
	if pid == 0 {
		return Event{Timestamp: time.Now()}, nil
	}

	// Step 4: Open the process to query its image name
	handle, err := windows.OpenProcess(
		windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid,
	)
	if err != nil {
		return Event{}, fmt.Errorf("opening process %d: %w", pid, err)
	}
	defer windows.CloseHandle(handle)

	// Step 5: Get the full path to the process executable
	var buf [windows.MAX_PATH]uint16
	size := uint32(len(buf))
	err = windows.QueryFullProcessImageName(handle, 0, &buf[0], &size)
	if err != nil {
		return Event{}, fmt.Errorf("querying process image name: %w", err)
	}

	exePath := windows.UTF16ToString(buf[:size])
	processName := filepath.Base(exePath)
	appName := strings.TrimSuffix(processName, ".exe")

	return Event{
		AppName:     appName,
		ProcessName: processName,
		WindowTitle: title,
		Timestamp:   time.Now(),
	}, nil
}

// getWindowText retrieves the title bar text.
func getWindowText(hwnd uintptr) string {
	length, _, _ := procGetWindowTextLengthW.Call(hwnd)
	if length == 0 {
		return ""
	}

	buf := make([]uint16, length+1)
	procGetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), uintptr(length+1))
	return windows.UTF16ToString(buf)
}
