package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"mousemover/internal/config"
	"mousemover/internal/core"
	"mousemover/internal/win"
)

const (
	idActivate = 1001 + iota
	idPause
	idSave
	idHide
	idQuit
	idIdle
	idInterval
	idPixels
	idMode
	idDisplay
	idAutostart
	idAutoActivate
	idMinActivate
	idStartMin
	idNotify
	idStatus
	idIdleStatus
	idNext
	idCycles
	idRuntime
	idModeStatus
	idIdleUnit
)
const trayMessage = win.WM_APP + 1
const timerID = 1

type application struct {
	hwnd, instance, hook, mutex                              uintptr
	iconActive, iconPaused, iconOff                          uintptr
	cfg                                                      config.Config
	path                                                     string
	state                                                    core.State
	controls                                                 map[int]uintptr
	logger                                                   *log.Logger
	logFile                                                  *os.File
	powerOn                                                  bool
	powerFlags                                               uintptr
	quitting                                                 bool
	trayReady                                                bool
	fontBody, fontCaption, fontValue, fontHeading, fontTitle uintptr
	background                                               uintptr
	textColors                                               map[uintptr]uintptr
}

var app *application

func main() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	minimized := flag.Bool("minimized", false, "Start im Tray")
	autostart := flag.Bool("autostart", false, "Autostart")
	flag.Parse()
	a := &application{controls: make(map[int]uintptr)}
	app = a
	a.initLog()
	defer a.closeLog()
	h, exists, err := win.SingleInstance()
	if err != nil {
		win.ErrorBox(0, err.Error())
		return
	}
	a.mutex = h
	defer win.Kernel("CloseHandle").Call(h)
	if exists {
		win.MessageBox(0, "MouseMover läuft bereits. Öffnen Sie das vorhandene Fenster über das Tray-Symbol.", "MouseMover")
		return
	}
	path, err := config.Path()
	if err != nil {
		win.ErrorBox(0, err.Error())
		return
	}
	a.path = path
	a.cfg, err = config.Load(path)
	if err != nil {
		a.logf("Konfigurationsfehler: %v", err)
		win.ErrorBox(0, "Konfiguration ungültig. Standardwerte werden verwendet. Bitte Einstellungen prüfen und speichern.\n"+err.Error())
	}
	a.state.LastPhysical = time.Now()
	if err = a.createWindow(); err != nil {
		a.logf("Startfehler: %v", err)
		win.ErrorBox(0, err.Error())
		return
	}
	defer a.cleanup()
	if a.cfg.AutoActivate {
		a.activate()
	}
	if !a.trayReady || !(*minimized || (*autostart && a.cfg.StartMinimized) || a.cfg.StartMinimized) {
		win.Show(a.hwnd, win.SW_SHOW)
	}
	a.refresh()
	var msg win.Msg
	for {
		r, _, _ := win.User("GetMessageW").Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if int32(r) <= 0 {
			break
		}
		win.User("TranslateMessage").Call(uintptr(unsafe.Pointer(&msg)))
		win.User("DispatchMessageW").Call(uintptr(unsafe.Pointer(&msg)))
	}
}

func (a *application) initLog() {
	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		return
	}
	p := filepath.Join(base, "Mousemover", "logs")
	if os.MkdirAll(p, 0700) != nil {
		return
	}
	path := filepath.Join(p, "mm.log")
	if st, e := os.Stat(path); e == nil && st.Size() > 1024*1024 {
		os.Rename(path, path+".old")
	}
	f, e := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if e == nil {
		a.logFile = f
		a.logger = log.New(f, "", log.LstdFlags)
		a.logf("Programmstart")
	}
}
func (a *application) closeLog() {
	a.logf("Reguläres Beenden")
	if a.logFile != nil {
		a.logFile.Close()
	}
}
func (a *application) logf(f string, args ...any) {
	if a.logger != nil {
		a.logger.Printf(f, args...)
	}
}

func wndproc(hwnd, msg, w, l uintptr) uintptr {
	if app == nil {
		return def(hwnd, msg, w, l)
	}
	a := app
	switch msg {
	case win.WM_CLOSE:
		if !a.quitting && a.trayReady {
			win.Show(hwnd, win.SW_HIDE)
			return 0
		}
	case win.WM_DESTROY:
		win.User("PostQuitMessage").Call(0)
		return 0
	case win.WM_QUERYENDSESSION:
		return 1
	case win.WM_ENDSESSION:
		if w != 0 {
			a.stopPower()
		}
		return 0
	case win.WM_WTSSESSION_CHANGE:
		if w == win.WTS_SESSION_LOCK {
			a.state.Locked = true
			a.stopPower()
		} else if w == win.WTS_SESSION_UNLOCK {
			a.state.Locked = false
			a.state.LastPhysical = time.Now()
		}
		a.refresh()
		return 0
	case win.WM_TIMER:
		if w == timerID {
			a.tick()
			return 0
		}
	case trayMessage:
		if l == win.WM_LBUTTONDBLCLK {
			a.show()
			return 0
		}
		if l == win.WM_RBUTTONUP {
			a.trayMenu()
			return 0
		}
	case win.WM_COMMAND:
		a.command(int(w & 0xffff))
		return 0
	case win.WM_CTLCOLORSTATIC, win.WM_CTLCOLORBTN:
		if a.background != 0 {
			return a.controlColor(w, l)
		}
	}
	return def(hwnd, msg, w, l)
}
func def(hwnd, msg, w, l uintptr) uintptr {
	r, _, _ := win.User("DefWindowProcW").Call(hwnd, msg, w, l)
	return r
}
func hookproc(code, w, l uintptr) uintptr {
	if app != nil && int32(code) == win.HC_ACTION && w == win.WM_MOUSEMOVE {
		m := win.CopyMouseHook(l)
		if m.Flags&win.LLMHF_INJECTED == 0 {
			app.state.LastPhysical = time.Now()
			app.state.LastMove = time.Time{}
		}
	}
	r, _, _ := win.User("CallNextHookEx").Call(0, code, w, l)
	return r
}

func (a *application) createWindow() error {
	a.initStyle()
	if a.background == 0 {
		return fmt.Errorf("Fensterhintergrund konnte nicht erstellt werden")
	}
	instance, _, _ := win.Kernel("GetModuleHandleW").Call(0)
	a.instance = instance
	class := win.Ptr("MouseMoverWindow")
	wc := win.WndClass{Size: uint32(unsafe.Sizeof(win.WndClass{})), WndProc: syscall.NewCallback(wndproc), Instance: instance, ClassName: uintptr(unsafe.Pointer(class)), Background: a.background}
	a.iconActive = win.StatusIcon(34, 166, 72)
	a.iconPaused = win.StatusIcon(230, 172, 25)
	a.iconOff = win.StatusIcon(128, 128, 128)
	if a.iconActive == 0 || a.iconPaused == 0 || a.iconOff == 0 {
		return fmt.Errorf("Status-Symbole konnten nicht erstellt werden")
	}
	wc.Icon = a.iconActive
	wc.SmallIcon = wc.Icon
	if _, err := win.Call(win.User("RegisterClassExW"), uintptr(unsafe.Pointer(&wc))); err != nil {
		return fmt.Errorf("Fensterklasse: %w", err)
	}
	a.hwnd = win.CreateWindow("MouseMoverWindow", "MouseMover", win.WS_OVERLAPPEDWINDOW&^0x00040000&^0x00010000, 100, 100, 690, 750, 0, 0, instance)
	if a.hwnd == 0 {
		return fmt.Errorf("Hauptfenster konnte nicht erstellt werden")
	}
	a.buildControls()
	a.updateFields()
	if err := a.updateTray(true); err != nil {
		a.logf("Tray noch nicht verfügbar: %v", err)
	}
	h, _, e := win.User("SetWindowsHookExW").Call(win.WH_MOUSE_LL, syscall.NewCallback(hookproc), instance, 0)
	if h == 0 {
		return fmt.Errorf("Maus-Hook: %v", e)
	}
	a.hook = h
	win.WTS("WTSRegisterSessionNotification").Call(a.hwnd, win.NOTIFY_FOR_THIS_SESSION)
	win.User("SetTimer").Call(a.hwnd, timerID, 1000, 0)
	return nil
}
func (a *application) control(id int, class, title string, style uint32, x, y, w, h int32) uintptr {
	p := win.CreateWindow(class, title, win.WS_CHILD|win.WS_VISIBLE|style, x, y, w, h, a.hwnd, uintptr(id), a.instance)
	a.controls[id] = p
	font := a.fontBody
	if id >= idStatus && id <= idModeStatus {
		font = a.fontValue
	}
	if id == idActivate {
		font = a.fontHeading
	}
	a.setFont(p, font)
	a.textColors[p] = colorText
	return p
}
func (a *application) label(title string, x, y, w int32) {
	a.staticText(title, x, y, w, 20, a.fontCaption, colorMuted)
}
func (a *application) buildControls() {
	a.staticText("MouseMover", 24, 18, 440, 40, a.fontTitle, colorText)
	a.staticText("Mausaktivität und Ruhezustand bewusst steuern", 24, 60, 620, 24, a.fontBody, colorMuted)
	a.staticText("LIVE-STATUS", 24, 101, 620, 24, a.fontHeading, colorAccent)
	a.label("STATUS", 24, 137, 290)
	a.control(idStatus, "STATIC", "Deaktiviert", 0, 24, 157, 302, 30)
	a.label("INAKTIVITÄT", 350, 137, 290)
	a.control(idIdleStatus, "STATIC", "0 s", 0, 350, 157, 290, 30)
	a.label("NÄCHSTE AKTIVITÄT", 24, 191, 290)
	a.control(idNext, "STATIC", "–", 0, 24, 211, 290, 30)
	a.label("BEWEGUNGSZYKLEN", 350, 191, 290)
	a.control(idCycles, "STATIC", "0", 0, 350, 211, 290, 30)
	a.label("LAUFZEIT", 24, 245, 290)
	a.control(idRuntime, "STATIC", "–", 0, 24, 265, 290, 30)
	a.label("BETRIEBSMODUS", 350, 245, 290)
	a.control(idModeStatus, "STATIC", "MouseMover", 0, 350, 265, 290, 30)
	a.staticText("EINSTELLUNGEN", 24, 312, 620, 24, a.fontHeading, colorAccent)
	a.staticText("Aktivieren nach", 24, 350, 245, 25, a.fontBody, colorText)
	a.control(idIdle, "EDIT", "", win.WS_BORDER|win.ES_NUMBER|win.WS_TABSTOP, 310, 345, 105, 28)
	unit := a.control(idIdleUnit, "COMBOBOX", "", win.CBS_DROPDOWNLIST|win.WS_TABSTOP, 427, 345, 205, 160)
	for _, s := range []string{"Sekunden", "Minuten", "Stunden"} {
		win.Send(unit, win.CB_ADDSTRING, 0, win.U(s))
	}
	a.staticText("Bewegungsintervall (Sekunden)", 24, 387, 265, 25, a.fontBody, colorText)
	a.control(idInterval, "EDIT", "", win.WS_BORDER|win.ES_NUMBER|win.WS_TABSTOP, 310, 382, 105, 28)
	a.staticText("Bewegungsdistanz (Pixel)", 24, 424, 265, 25, a.fontBody, colorText)
	a.control(idPixels, "EDIT", "", win.WS_BORDER|win.ES_NUMBER|win.WS_TABSTOP, 310, 419, 105, 28)
	a.staticText("Betriebsmodus", 24, 461, 265, 25, a.fontBody, colorText)
	mode := a.control(idMode, "COMBOBOX", "", win.CBS_DROPDOWNLIST|win.WS_TABSTOP, 310, 456, 322, 190)
	for _, s := range []string{"MouseMover", "Keep Awake", "Kombiniert"} {
		win.Send(mode, win.CB_ADDSTRING, 0, win.U(s))
	}
	a.control(idDisplay, "BUTTON", "Bildschirm aktiv halten", win.BS_AUTOCHECKBOX|win.WS_TABSTOP, 24, 497, 290, 28)
	a.control(idAutostart, "BUTTON", "Mit Windows starten", win.BS_AUTOCHECKBOX|win.WS_TABSTOP, 350, 497, 290, 28)
	a.control(idAutoActivate, "BUTTON", "Beim Start automatisch aktivieren", win.BS_AUTOCHECKBOX|win.WS_TABSTOP, 24, 526, 290, 28)
	a.control(idMinActivate, "BUTTON", "Beim Aktivieren in Tray minimieren", win.BS_AUTOCHECKBOX|win.WS_TABSTOP, 350, 526, 290, 28)
	a.control(idStartMin, "BUTTON", "Beim Start im Tray öffnen", win.BS_AUTOCHECKBOX|win.WS_TABSTOP, 24, 555, 290, 28)
	a.control(idNotify, "BUTTON", "Benachrichtigungen", win.BS_AUTOCHECKBOX|win.WS_TABSTOP, 350, 555, 290, 28)
	a.control(idActivate, "BUTTON", "Aktivieren", win.WS_TABSTOP, 24, 596, 198, 36)
	a.control(idPause, "BUTTON", "5 Min. pausieren", win.WS_TABSTOP, 234, 596, 198, 36)
	a.control(idSave, "BUTTON", "Speichern", win.WS_TABSTOP, 444, 596, 198, 36)
	a.control(idHide, "BUTTON", "In Tray minimieren", win.WS_TABSTOP, 24, 638, 198, 34)
	a.control(idQuit, "BUTTON", "Beenden", win.WS_TABSTOP, 234, 638, 198, 34)
	a.staticText("Keine Garantie für den Anwesenheitsstatus in Kommunikationsprogrammen.", 24, 685, 620, 24, a.fontCaption, colorMuted)
}
func (a *application) updateFields() {
	c := a.cfg
	win.SetText(a.controls[idIdle], strconv.Itoa(c.IdleTimeoutSeconds))
	win.Send(a.controls[idIdleUnit], win.CB_SETCURSEL, 0, 0)
	win.SetText(a.controls[idInterval], strconv.Itoa(c.MovementIntervalSeconds))
	win.SetText(a.controls[idPixels], strconv.Itoa(c.MovementPixels))
	i := uintptr(0)
	if c.Mode == "awake" {
		i = 1
	} else if c.Mode == "combined" {
		i = 2
	}
	win.Send(a.controls[idMode], win.CB_SETCURSEL, i, 0)
	for _, v := range []struct {
		id int
		on bool
	}{{idDisplay, c.KeepDisplayAwake}, {idAutostart, c.StartWithWindows}, {idAutoActivate, c.AutoActivate}, {idMinActivate, c.MinimizeOnActivate}, {idStartMin, c.StartMinimized}, {idNotify, c.NotificationsEnabled}} {
		win.SetCheck(a.controls[v.id], v.on)
	}
}
func (a *application) readFields() (config.Config, error) {
	c := a.cfg
	var err error
	c.IdleTimeoutSeconds, err = strconv.Atoi(strings.TrimSpace(win.Text(a.controls[idIdle])))
	if err != nil {
		return c, fmt.Errorf("Aktivierungszeit: %w", err)
	}
	factor := 1
	switch win.Send(a.controls[idIdleUnit], win.CB_GETCURSEL, 0, 0) {
	case 1:
		factor = 60
	case 2:
		factor = 3600
	}
	if c.IdleTimeoutSeconds < 1 || c.IdleTimeoutSeconds > 86400/factor {
		return c, fmt.Errorf("Aktivierungszeit ist außerhalb des gültigen Bereichs")
	}
	c.IdleTimeoutSeconds *= factor
	c.MovementIntervalSeconds, err = strconv.Atoi(strings.TrimSpace(win.Text(a.controls[idInterval])))
	if err != nil {
		return c, fmt.Errorf("Intervall: %w", err)
	}
	c.MovementPixels, err = strconv.Atoi(strings.TrimSpace(win.Text(a.controls[idPixels])))
	if err != nil {
		return c, fmt.Errorf("Bewegungsdistanz: %w", err)
	}
	switch win.Send(a.controls[idMode], win.CB_GETCURSEL, 0, 0) {
	case 0:
		c.Mode = "mouse"
	case 1:
		c.Mode = "awake"
	case 2:
		c.Mode = "combined"
	default:
		return c, fmt.Errorf("Bitte Betriebsmodus wählen")
	}
	c.KeepDisplayAwake = win.Check(a.controls[idDisplay])
	c.StartWithWindows = win.Check(a.controls[idAutostart])
	c.AutoActivate = win.Check(a.controls[idAutoActivate])
	c.MinimizeOnActivate = win.Check(a.controls[idMinActivate])
	c.StartMinimized = win.Check(a.controls[idStartMin])
	c.NotificationsEnabled = win.Check(a.controls[idNotify])
	return c, c.Validate()
}
func (a *application) save() {
	c, err := a.readFields()
	if err != nil {
		win.ErrorBox(a.hwnd, err.Error())
		return
	}
	exe, err := os.Executable()
	if err == nil && c.StartWithWindows != a.cfg.StartWithWindows {
		err = win.SetAutostart(exe, c.StartWithWindows)
	}
	if err != nil {
		win.ErrorBox(a.hwnd, "Autostart: "+err.Error())
		return
	}
	if err = config.Save(a.path, c); err != nil {
		win.ErrorBox(a.hwnd, "Speichern: "+err.Error())
		return
	}
	if c.Mode != a.cfg.Mode {
		a.logf("Betriebsmodus: %s", c.Mode)
	}
	a.cfg = c
	a.syncPower()
	a.refresh()
	if c.NotificationsEnabled {
		win.MessageBox(a.hwnd, "Einstellungen gespeichert.", "MouseMover")
	}
}
func (a *application) command(id int) {
	switch id {
	case idActivate:
		if a.state.Enabled {
			a.deactivate()
		} else {
			a.activate()
			if a.cfg.MinimizeOnActivate && a.trayReady {
				win.Show(a.hwnd, win.SW_HIDE)
			}
		}
	case idPause:
		a.pause(5 * time.Minute)
	case idSave:
		a.save()
	case idHide:
		if a.trayReady {
			win.Show(a.hwnd, win.SW_HIDE)
		}
	case idQuit:
		a.quit()
	}
}
func (a *application) activate() {
	a.state.Enabled = true
	a.state.Activated = time.Now()
	a.state.LastPhysical = time.Now()
	a.state.LastMove = time.Time{}
	a.state.PausedUntil = time.Time{}
	a.syncPower()
	a.logf("Aktiviert")
	a.refresh()
}
func (a *application) deactivate() {
	a.state.Enabled = false
	a.state.PausedUntil = time.Time{}
	a.stopPower()
	a.logf("Deaktiviert")
	a.refresh()
}
func (a *application) pause(d time.Duration) {
	if !a.state.Enabled {
		a.activate()
	}
	a.state.PausedUntil = time.Now().Add(d)
	a.stopPower()
	a.logf("Pausiert für %s", d)
	a.refresh()
}
func (a *application) tick() {
	now := time.Now()
	if !a.state.PausedUntil.IsZero() && !now.Before(a.state.PausedUntil) {
		a.state.PausedUntil = time.Time{}
		a.state.LastPhysical = now
		a.state.LastMove = time.Time{}
	}
	a.syncPower()
	if (a.cfg.Mode == "mouse" || a.cfg.Mode == "combined") && a.state.ShouldMove(now, time.Duration(a.cfg.IdleTimeoutSeconds)*time.Second, time.Duration(a.cfg.MovementIntervalSeconds)*time.Second) {
		if a.move() {
			a.state.Cycles++
		}
		a.state.LastMove = now
	}
	a.refresh()
}
func (a *application) move() bool {
	before, err := win.Cursor()
	if err != nil {
		a.logf("GetCursorPos: %v", err)
		return false
	}
	physicalBefore := a.state.LastPhysical
	distance := a.cfg.MovementPixels
	// A move towards the screen interior avoids clipping at virtual desktop edges.
	x := int32(0)
	left, _, _ := win.User("GetSystemMetrics").Call(76)
	width, _, _ := win.User("GetSystemMetrics").Call(78)
	if before.X < int32(left)+int32(width)/2 {
		x = int32(distance)
	} else {
		x = -int32(distance)
	}
	if err = win.MovePair(int(x)); err != nil {
		a.logf("SendInput: %v", err)
		return false
	}
	after, err := win.Cursor()
	if err == nil && after != before && a.state.LastPhysical.Equal(physicalBefore) {
		if e := win.SetCursor(before); e != nil {
			a.logf("SetCursorPos: %v", e)
		}
	}
	return true
}
func (a *application) syncPower() {
	want := a.state.Enabled && !a.state.Paused(time.Now()) && (a.cfg.Mode == "awake" || a.cfg.Mode == "combined")
	flags := uintptr(win.ES_CONTINUOUS | win.ES_SYSTEM_REQUIRED)
	if a.cfg.KeepDisplayAwake {
		flags |= win.ES_DISPLAY_REQUIRED
	}
	if want && (!a.powerOn || a.powerFlags != flags) {
		if err := win.ExecutionState(flags); err != nil {
			a.logf("SetThreadExecutionState: %v", err)
		} else {
			a.powerOn = true
			a.powerFlags = flags
		}
	} else if !want && a.powerOn {
		a.stopPower()
	}
}
func (a *application) stopPower() {
	if !a.powerOn {
		return
	}
	if err := win.ExecutionState(win.ES_CONTINUOUS); err != nil {
		a.logf("Ruhezustandsschutz beenden: %v", err)
	}
	a.powerOn = false
}
func (a *application) refresh() {
	if a.hwnd == 0 {
		return
	}
	now := time.Now()
	status := "Deaktiviert"
	if a.state.Enabled {
		status = "Wartet auf Inaktivität"
		if a.state.Paused(now) {
			status = "Pausiert"
		} else if a.cfg.Mode == "awake" {
			status = "Aktiv (Keep Awake)"
		} else if a.state.Idle(now) >= time.Duration(a.cfg.IdleTimeoutSeconds)*time.Second {
			status = "Automatische Aktivität läuft"
		}
	}
	statusColor := uintptr(colorMuted)
	if a.state.Enabled {
		statusColor = colorActive
	}
	if a.state.Paused(now) {
		statusColor = colorPaused
	}
	a.textColors[a.controls[idStatus]] = statusColor
	win.SetText(a.controls[idStatus], status)
	win.SetText(a.controls[idIdleStatus], fmt.Sprintf("%d s", int(a.state.Idle(now).Seconds())))
	next := "–"
	if a.state.Enabled && !a.state.Paused(now) && a.cfg.Mode != "awake" {
		wait := time.Duration(a.cfg.IdleTimeoutSeconds)*time.Second - a.state.Idle(now)
		if wait <= 0 {
			wait = time.Duration(a.cfg.MovementIntervalSeconds)*time.Second - now.Sub(a.state.LastMove)
		}
		if wait < 0 {
			wait = 0
		}
		next = fmt.Sprintf("%d s", int(wait.Seconds()))
	}
	win.SetText(a.controls[idNext], next)
	win.SetText(a.controls[idCycles], strconv.FormatUint(a.state.Cycles, 10))
	uptime := "–"
	if a.state.Enabled {
		uptime = fmt.Sprintf("%d min %d s", int(now.Sub(a.state.Activated).Minutes()), int(now.Sub(a.state.Activated).Seconds())%60)
	}
	win.SetText(a.controls[idRuntime], uptime)
	m := map[string]string{"mouse": "MouseMover", "awake": "Keep Awake", "combined": "Kombiniert"}
	win.SetText(a.controls[idModeStatus], m[a.cfg.Mode])
	if a.state.Enabled {
		win.SetText(a.controls[idActivate], "Deaktivieren")
	} else {
		win.SetText(a.controls[idActivate], "Aktivieren")
	}
	a.updateTray(!a.trayReady)
}
func (a *application) show() {
	win.Show(a.hwnd, win.SW_RESTORE)
	win.User("SetForegroundWindow").Call(a.hwnd)
}
func (a *application) quit() {
	a.quitting = true
	a.stopPower()
	win.User("DestroyWindow").Call(a.hwnd)
}
func (a *application) cleanup() {
	win.User("KillTimer").Call(a.hwnd, timerID)
	if a.hook != 0 {
		win.User("UnhookWindowsHookEx").Call(a.hook)
	}
	win.WTS("WTSUnRegisterSessionNotification").Call(a.hwnd)
	a.stopPower()
	a.removeTray()
	for _, icon := range []uintptr{a.iconActive, a.iconPaused, a.iconOff} {
		if icon != 0 {
			win.User("DestroyIcon").Call(icon)
		}
	}
	a.releaseStyle()
}
