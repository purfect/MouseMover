package main

import (
	"fmt"
	"syscall"
	"time"
	"unsafe"

	"mousemover/internal/config"
	"mousemover/internal/win"
)

const (
	menuToggle = 2001 + iota
	menuPause5
	menuPause15
	menuPause30
	menuMouse
	menuAwake
	menuCombined
	menuSettings
	menuQuit
)

func (a *application) trayIcon() uintptr {
	if !a.state.Enabled {
		return a.iconOff
	}
	if a.state.Paused(now()) {
		return a.iconPaused
	}
	return a.iconActive
}
func now() time.Time { return time.Now() }

func (a *application) updateTray(add bool) error {
	if a.hwnd == 0 {
		return nil
	}
	status := "Deaktiviert"
	if a.state.Enabled {
		status = "Aktiv"
		if a.state.Paused(now()) {
			status = "Pausiert"
		}
	}
	n := win.NotifyIcon{Size: uint32(unsafe.Sizeof(win.NotifyIcon{})), Hwnd: a.hwnd, ID: 1, Flags: win.NIF_MESSAGE | win.NIF_ICON | win.NIF_TIP, CallbackMessage: trayMessage, Icon: a.trayIcon()}
	copy(n.Tip[:], syscall.StringToUTF16("MouseMover – "+status))
	op := uintptr(win.NIM_MODIFY)
	if add {
		op = win.NIM_ADD
	}
	r, _, e := win.Shell("Shell_NotifyIconW").Call(op, uintptr(unsafe.Pointer(&n)))
	if r == 0 {
		a.trayReady = false
		return fmt.Errorf("Tray-Symbol: %v", e)
	}
	a.trayReady = true
	return nil
}
func (a *application) removeTray() {
	n := win.NotifyIcon{Size: uint32(unsafe.Sizeof(win.NotifyIcon{})), Hwnd: a.hwnd, ID: 1}
	win.Shell("Shell_NotifyIconW").Call(win.NIM_DELETE, uintptr(unsafe.Pointer(&n)))
}
func appendMenu(menu uintptr, flags uintptr, id int, title string) {
	win.User("AppendMenuW").Call(menu, flags, uintptr(id), win.U(title))
}
func (a *application) trayMenu() {
	menu, _, _ := win.User("CreatePopupMenu").Call()
	if menu == 0 {
		return
	}
	defer win.User("DestroyMenu").Call(menu)
	status := "Deaktiviert"
	if a.state.Enabled {
		status = "Aktiv"
		if a.state.Paused(now()) {
			status = "Pausiert"
		}
	}
	appendMenu(menu, win.MF_STRING|win.MF_GRAYED, 0, "Status: "+status)
	toggle := "Aktivieren"
	if a.state.Enabled {
		toggle = "Deaktivieren"
	}
	appendMenu(menu, win.MF_STRING, menuToggle, toggle)
	appendMenu(menu, win.MF_SEPARATOR, 0, "")
	appendMenu(menu, win.MF_STRING, menuPause5, "5 Minuten pausieren")
	appendMenu(menu, win.MF_STRING, menuPause15, "15 Minuten pausieren")
	appendMenu(menu, win.MF_STRING, menuPause30, "30 Minuten pausieren")
	appendMenu(menu, win.MF_SEPARATOR, 0, "")
	for _, item := range []struct {
		id          int
		mode, title string
	}{{menuMouse, "mouse", "Modus: MouseMover"}, {menuAwake, "awake", "Modus: Keep Awake"}, {menuCombined, "combined", "Modus: Kombiniert"}} {
		title := item.title
		if a.cfg.Mode == item.mode {
			title = "✓ " + title
		}
		appendMenu(menu, win.MF_STRING, item.id, title)
	}
	appendMenu(menu, win.MF_SEPARATOR, 0, "")
	appendMenu(menu, win.MF_STRING, menuSettings, "Einstellungen öffnen")
	appendMenu(menu, win.MF_STRING, menuQuit, "Beenden")
	var p win.Point
	win.User("GetCursorPos").Call(uintptr(unsafe.Pointer(&p)))
	win.User("SetForegroundWindow").Call(a.hwnd)
	choice, _, _ := win.User("TrackPopupMenu").Call(menu, win.TPM_RIGHTBUTTON|win.TPM_RETURNCMD, uintptr(p.X), uintptr(p.Y), 0, a.hwnd, 0)
	switch choice {
	case menuToggle:
		if a.state.Enabled {
			a.deactivate()
		} else {
			a.activate()
		}
	case menuPause5:
		a.pause(5 * time.Minute)
	case menuPause15:
		a.pause(15 * time.Minute)
	case menuPause30:
		a.pause(30 * time.Minute)
	case menuMouse:
		a.changeMode("mouse")
	case menuAwake:
		a.changeMode("awake")
	case menuCombined:
		a.changeMode("combined")
	case menuSettings:
		a.show()
	case menuQuit:
		a.quit()
	}
}
func (a *application) changeMode(mode string) {
	previous := a.cfg.Mode
	a.cfg.Mode = mode
	if err := config.Save(a.path, a.cfg); err != nil {
		a.cfg.Mode = previous
		win.ErrorBox(a.hwnd, err.Error())
		return
	}
	a.logf("Betriebsmodus: %s", mode)
	a.updateFields()
	a.syncPower()
	a.refresh()
}
