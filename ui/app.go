package ui

import (
	"database/sql"
	_ "embed"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"syscall"
	"time"
	"unsafe"

	"github.com/focustrack/focustrack/internal/aggregator"
	"github.com/focustrack/focustrack/internal/config"
	"github.com/getlantern/systray"
	webview "github.com/jchv/go-webview2"
	"golang.org/x/sys/windows/registry"
)

const startupRegKey = `Software\Microsoft\Windows\CurrentVersion\Run`
const appName       = "FocusTrack"

func setStartup(enable bool) {
	k, err := registry.OpenKey(
		registry.CURRENT_USER,
		startupRegKey,
		registry.SET_VALUE|registry.QUERY_VALUE,
	)
	if err != nil {
		log.Printf("warn: cannot open startup registry key: %v", err)
		return
	}
	defer k.Close()

	if enable {
		exePath, err := os.Executable()
		if err != nil {
			log.Printf("warn: cannot get exe path: %v", err)
			return
		}
		if err := k.SetStringValue(appName, exePath); err != nil {
			log.Printf("warn: cannot set startup registry value: %v", err)
		}
	} else {
		_ = k.DeleteValue(appName)
	}
}

func SyncStartupState() {
	k, err := registry.OpenKey(
		registry.CURRENT_USER,
		startupRegKey,
		registry.QUERY_VALUE,
	)
	if err != nil {
		return
	}
	defer k.Close()

	_, _, err = k.GetStringValue(appName)
	registryEnabled := err == nil

	cfg, _ := config.Load()
	if cfg == nil {
		cfg = &config.Config{}
	}

	if cfg.StartWithWindows != registryEnabled {
		cfg.StartWithWindows = registryEnabled
		config.Save(cfg)
	}
}

//go:embed FocusTrack.ico
var appIcon []byte

//go:embed icon.png
var trayIcon []byte

func setWindowIcon(hwnd syscall.Handle) {
	tmp, err := os.CreateTemp("", "ft-icon-*.ico")
	if err != nil {
		return
	}
	defer os.Remove(tmp.Name())
	tmp.Write(appIcon)
	tmp.Close()

	iconPath, _ := syscall.UTF16PtrFromString(tmp.Name())
	hIcon, _, _ := procLoadImage.Call(
		0,
		uintptr(unsafe.Pointer(iconPath)),
		1, // IMAGE_ICON
		0, 0,
		0x0010|0x0040, // LR_LOADFROMFILE | LR_DEFAULTSIZE
	)
	if hIcon == 0 {
		return
	}

	procSendMessage.Call(uintptr(hwnd), 0x0080, 0, hIcon) // small icon
	procSendMessage.Call(uintptr(hwnd), 0x0080, 1, hIcon) // big icon (taskbar)
}

var w webview.WebView

// Run starts the UI. Blocks until window is closed.
func Run(db *sql.DB) {
	// Start embedded HTTP server on a random free port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatalf("ui: listen: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	go http.Serve(listener, newRouter(db))

	// Create WebView2 window
	w = webview.NewWithOptions(webview.WebViewOptions{
		Debug:  false,
		Window: nil,
	})
	if w == nil {
		log.Fatal("Failed to load webview.")
	}
	defer w.Destroy()

	w.SetTitle("FocusTrack")
	w.SetSize(960, 640, webview.HintMin)

	// Start systray
	go systray.Run(onTrayReady(db), func() {})

	// Register Go functions callable from JS
	registerBridge(w, db)

	// Load frontend
	w.Navigate(fmt.Sprintf("http://127.0.0.1:%d", port))

	// Give WebView2 a moment to create its window, then set icon
	go func() {
		time.Sleep(500 * time.Millisecond)
		titlePtr, _ := syscall.UTF16PtrFromString("FocusTrack")
		hwnd, _, _ := procFindWindow.Call(0, uintptr(unsafe.Pointer(titlePtr)))
		if hwnd != 0 {
			setWindowIcon(syscall.Handle(hwnd))
		}
	}()

	w.Run() // blocks
}

// onTrayReady handles the system tray
func onTrayReady(db *sql.DB) func() {
	return func() {
		// Just a fallback title since we don't have iconBytes easily available from main.go
		// though we could pass it or read it, for now we will rely on default blank icon.
		systray.SetTitle("FocusTrack")
		systray.SetTooltip("FocusTrack — screen time tracker")
		log.Printf("tray icon bytes (ico): %d", len(appIcon))
		systray.SetIcon(appIcon)

		mOpen := systray.AddMenuItem("Open FocusTrack", "")
		mToday := systray.AddMenuItem("Today: loading...", "")
		mToday.Disable()
		systray.AddSeparator()
		mQuit := systray.AddMenuItem("Quit", "")

		// Refresh tray label every 60 seconds.
		go func() {
			updateTrayLabel(db, mToday)
			ticker := time.NewTicker(60 * time.Second)
			defer ticker.Stop()
			for range ticker.C {
				updateTrayLabel(db, mToday)
			}
		}()

		for {
			select {
			case <-mOpen.ClickedCh:
				if w != nil {
					w.Dispatch(func() {
						hwnd := (syscall.Handle)(w.Window())
						if hwnd != 0 {
							// SW_RESTORE = 9, SW_SHOW = 5
							showWindow(hwnd, 9)
							setWindowPos(hwnd, 0, 0, 0, 0, 0, 1|2|64)
						}
					})
				}
			case <-mQuit.ClickedCh:
				systray.Quit()
				if w != nil {
					w.Dispatch(func() {
						w.Terminate()
					})
				}
				return
			}
		}
	}
}

func updateTrayLabel(db *sql.DB, item *systray.MenuItem) {
	report, err := aggregator.GetDailyReport(db)
	if err != nil {
		return
	}
	item.SetTitle("Today: " + report.FormattedTotal)
}

// Win32 API endpoints for window show/hide
var (
	user32           = syscall.NewLazyDLL("user32.dll")
	procShowWindow   = user32.NewProc("ShowWindow")
	procSetWindowPos = user32.NewProc("SetWindowPos")
	procLoadImage    = user32.NewProc("LoadImageW")
	procSendMessage  = user32.NewProc("SendMessageW")
	procFindWindow   = user32.NewProc("FindWindowW")
)

func showWindow(hwnd syscall.Handle, nCmdShow int) {
	procShowWindow.Call(uintptr(hwnd), uintptr(nCmdShow))
}

func setWindowPos(hwnd syscall.Handle, hWndInsertAfter syscall.Handle, x, y, cx, cy, wFlags uint) {
	procSetWindowPos.Call(uintptr(hwnd), uintptr(hWndInsertAfter), uintptr(x), uintptr(y), uintptr(cx), uintptr(cy), uintptr(wFlags))
}
