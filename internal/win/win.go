package win

import (
	"errors"
	"fmt"
	"syscall"
	"unsafe"
)

var user = syscall.NewLazyDLL("user32.dll")
var kernel = syscall.NewLazyDLL("kernel32.dll")
var shell = syscall.NewLazyDLL("shell32.dll")
var advapi = syscall.NewLazyDLL("advapi32.dll")
var wts = syscall.NewLazyDLL("wtsapi32.dll")
var ntdll = syscall.NewLazyDLL("ntdll.dll")
var gdi = syscall.NewLazyDLL("gdi32.dll")

func Proc(d *syscall.LazyDLL, n string) *syscall.LazyProc { return d.NewProc(n) }
func Call(p *syscall.LazyProc, a ...uintptr) (uintptr, error) {
	r, _, e := p.Call(a...)
	if r == 0 {
		if e != syscall.Errno(0) {
			return r, e
		}
		return r, errors.New("Windows-API-Aufruf fehlgeschlagen")
	}
	return r, nil
}
func Ptr(s string) *uint16              { return syscall.StringToUTF16Ptr(s) }
func Str(p *uint16) uintptr             { return uintptr(unsafe.Pointer(p)) }
func U(s string) uintptr                { return Str(Ptr(s)) }
func User(n string) *syscall.LazyProc   { return Proc(user, n) }
func Kernel(n string) *syscall.LazyProc { return Proc(kernel, n) }
func Shell(n string) *syscall.LazyProc  { return Proc(shell, n) }
func WTS(n string) *syscall.LazyProc    { return Proc(wts, n) }
func GDI(n string) *syscall.LazyProc    { return Proc(gdi, n) }

const (
	WS_OVERLAPPEDWINDOW     = 0x00CF0000
	WS_VISIBLE              = 0x10000000
	WS_CHILD                = 0x40000000
	WS_TABSTOP              = 0x00010000
	WS_BORDER               = 0x00800000
	ES_NUMBER               = 0x2000
	CBS_DROPDOWNLIST        = 3
	BS_AUTOCHECKBOX         = 3
	SW_HIDE                 = 0
	SW_SHOW                 = 5
	SW_RESTORE              = 9
	WM_CREATE               = 1
	WM_DESTROY              = 2
	WM_CLOSE                = 16
	WM_COMMAND              = 0x111
	WM_TIMER                = 0x113
	WM_LBUTTONDBLCLK        = 0x203
	WM_RBUTTONUP            = 0x205
	WM_MOUSEMOVE            = 0x200
	WM_QUERYENDSESSION      = 0x11
	WM_ENDSESSION           = 0x16
	WM_WTSSESSION_CHANGE    = 0x2B1
	WM_CTLCOLORSTATIC       = 0x138
	WM_CTLCOLORBTN          = 0x135
	WM_SETFONT              = 0x30
	WM_APP                  = 0x8000
	WM_USER                 = 0x400
	BM_GETCHECK             = 0xF0
	BM_SETCHECK             = 0xF1
	CB_ADDSTRING            = 0x143
	CB_GETCURSEL            = 0x147
	CB_SETCURSEL            = 0x14E
	MF_STRING               = 0
	MF_SEPARATOR            = 0x800
	MF_GRAYED               = 1
	TPM_RIGHTBUTTON         = 2
	TPM_RETURNCMD           = 0x100
	NIM_ADD                 = 0
	NIM_MODIFY              = 1
	NIM_DELETE              = 2
	NIF_MESSAGE             = 1
	NIF_ICON                = 2
	NIF_TIP                 = 4
	WH_MOUSE_LL             = 14
	HC_ACTION               = 0
	LLMHF_INJECTED          = 1
	INPUT_MOUSE             = 0
	MOUSEEVENTF_MOVE        = 1
	ES_SYSTEM_REQUIRED      = 1
	ES_DISPLAY_REQUIRED     = 2
	ES_CONTINUOUS           = 0x80000000
	WTS_SESSION_LOCK        = 7
	WTS_SESSION_UNLOCK      = 8
	NOTIFY_FOR_THIS_SESSION = 0
	KEY_SET_VALUE           = 2
	REG_SZ                  = 1
	HKEY_CURRENT_USER       = 0x80000001
)

type Point struct{ X, Y int32 }
type Msg struct {
	Hwnd           uintptr
	Message        uint32
	Padding        uint32
	WParam, LParam uintptr
	Time           uint32
	Pt             Point
	LPrivate       uint32
}
type WndClass struct {
	Size, Style                                             uint32
	WndProc                                                 uintptr
	ClsExtra, WndExtra                                      int32
	Instance, Icon, Cursor, Background, MenuName, ClassName uintptr
	SmallIcon                                               uintptr
}
type MouseHook struct {
	Pt                     Point
	MouseData, Flags, Time uint32
	ExtraInfo              uintptr
}

func CopyMouseHook(src uintptr) MouseHook {
	var value MouseHook
	ntdll.NewProc("RtlMoveMemory").Call(uintptr(unsafe.Pointer(&value)), src, unsafe.Sizeof(value))
	return value
}

type MouseInput struct {
	Dx, Dy                      int32
	MouseData, Flags, Time, Pad uint32
	ExtraInfo                   uintptr
}
type Input struct {
	Type, Pad uint32
	Mouse     MouseInput
}
type NotifyIcon struct {
	Size                       uint32
	Hwnd                       uintptr
	ID, Flags, CallbackMessage uint32
	Icon                       uintptr
	Tip                        [128]uint16
	State, StateMask           uint32
	Info                       [256]uint16
	Timeout                    uint32
	InfoTitle                  [64]uint16
	InfoFlags                  uint32
	Guid                       [16]byte
	BalloonIcon                uintptr
}

func CreateWindow(class, title string, style uint32, x, y, w, h int32, parent, menu, instance uintptr) uintptr {
	r, _, _ := User("CreateWindowExW").Call(0, U(class), U(title), uintptr(style), uintptr(x), uintptr(y), uintptr(w), uintptr(h), parent, menu, instance, 0)
	return r
}
func Send(hwnd uintptr, msg uint32, w, l uintptr) uintptr {
	r, _, _ := User("SendMessageW").Call(hwnd, uintptr(msg), w, l)
	return r
}
func SetText(hwnd uintptr, s string) { User("SetWindowTextW").Call(hwnd, U(s)) }
func Text(hwnd uintptr) string {
	var b [256]uint16
	User("GetWindowTextW").Call(hwnd, uintptr(unsafe.Pointer(&b[0])), 256)
	return syscall.UTF16ToString(b[:])
}
func Check(hwnd uintptr) bool { return Send(hwnd, BM_GETCHECK, 0, 0) == 1 }
func SetCheck(hwnd uintptr, v bool) {
	n := uintptr(0)
	if v {
		n = 1
	}
	Send(hwnd, BM_SETCHECK, n, 0)
}
func Show(hwnd uintptr, how int) { User("ShowWindow").Call(hwnd, uintptr(how)) }
func MessageBox(hwnd uintptr, text, title string) {
	User("MessageBoxW").Call(hwnd, U(text), U(title), 0x40)
}
func ErrorBox(hwnd uintptr, text string) {
	User("MessageBoxW").Call(hwnd, U(text), U("MouseMover"), 0x10)
}

func Cursor() (Point, error) {
	var p Point
	_, err := Call(User("GetCursorPos"), uintptr(unsafe.Pointer(&p)))
	return p, err
}
func MovePair(px int) error {
	a := [2]Input{{Type: INPUT_MOUSE, Mouse: MouseInput{Dx: int32(px), Flags: MOUSEEVENTF_MOVE}}, {Type: INPUT_MOUSE, Mouse: MouseInput{Dx: int32(-px), Flags: MOUSEEVENTF_MOVE}}}
	r, _, e := User("SendInput").Call(2, uintptr(unsafe.Pointer(&a[0])), unsafe.Sizeof(a[0]))
	if r != 2 {
		return fmt.Errorf("SendInput: %v", e)
	}
	return nil
}
func SetCursor(p Point) error {
	_, err := Call(User("SetCursorPos"), uintptr(p.X), uintptr(p.Y))
	return err
}
func ExecutionState(flags uintptr) error {
	_, err := Call(Kernel("SetThreadExecutionState"), flags)
	return err
}

// StatusIcon builds a 16-pixel icon from color data held in the executable.
func StatusIcon(red, green, blue byte) uintptr {
	const size = 16
	var mask [size * size / 8]byte
	var pixels [size * size * 4]byte
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			dx, dy := x-7, y-7
			i := (y*size + x) * 4
			if dx*dx+dy*dy > 42 {
				mask[y*2+x/8] |= 1 << uint(7-x%8)
				continue
			}
			pixels[i], pixels[i+1], pixels[i+2], pixels[i+3] = blue, green, red, 255
		}
	}
	r, _, _ := User("CreateIcon").Call(0, size, size, 1, 32, uintptr(unsafe.Pointer(&mask[0])), uintptr(unsafe.Pointer(&pixels[0])))
	return r
}

func SetAutostart(exe string, on bool) error {
	path := Ptr(`Software\Microsoft\Windows\CurrentVersion\Run`)
	name := Ptr("MouseMover")
	if !on {
		var key uintptr
		r, _, _ := advapi.NewProc("RegOpenKeyExW").Call(HKEY_CURRENT_USER, Str(path), 0, KEY_SET_VALUE, uintptr(unsafe.Pointer(&key)))
		if r != 0 {
			return nil
		}
		defer advapi.NewProc("RegCloseKey").Call(key)
		r, _, _ = advapi.NewProc("RegDeleteValueW").Call(key, Str(name))
		if r != 0 && r != 2 {
			return syscall.Errno(r)
		}
		return nil
	}
	var key uintptr
	r, _, _ := advapi.NewProc("RegCreateKeyExW").Call(HKEY_CURRENT_USER, Str(path), 0, 0, 0, KEY_SET_VALUE, 0, uintptr(unsafe.Pointer(&key)), 0)
	if r != 0 {
		return syscall.Errno(r)
	}
	defer advapi.NewProc("RegCloseKey").Call(key)
	val := syscall.StringToUTF16(`"` + exe + `" --autostart --minimized`)
	r, _, _ = advapi.NewProc("RegSetValueExW").Call(key, Str(name), 0, REG_SZ, uintptr(unsafe.Pointer(&val[0])), uintptr(len(val)*2))
	if r != 0 {
		return syscall.Errno(r)
	}
	return nil
}

func SingleInstance() (uintptr, bool, error) {
	name := Ptr("Local\\MouseMover_mm_single_instance")
	r, _, e := Kernel("CreateMutexW").Call(0, 0, Str(name))
	if r == 0 {
		return 0, false, e
	}
	return r, e == syscall.Errno(183), nil
}
