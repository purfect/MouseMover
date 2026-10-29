package main

import (
	"mousemover/internal/win"
)

const (
	colorBackground = 0x00fcf9f8 // RGB(248, 249, 252)
	colorText       = 0x0032241c // RGB(28, 36, 50)
	colorMuted      = 0x007e6c61 // RGB(97, 108, 126)
	colorAccent     = 0x00bb6023 // RGB(35, 96, 187)
	colorActive     = 0x00548214 // RGB(20, 130, 84)
	colorPaused     = 0x00146eb2 // RGB(178, 110, 20)
)

func makeFont(height, weight int32) uintptr {
	r, _, _ := win.GDI("CreateFontW").Call(uintptr(height), 0, 0, 0, uintptr(weight), 0, 0, 0, 1, 0, 0, 5, 0, win.U("Segoe UI"))
	return r
}

func (a *application) initStyle() {
	a.fontBody = makeFont(-16, 400)
	a.fontCaption = makeFont(-14, 400)
	a.fontValue = makeFont(-19, 600)
	a.fontHeading = makeFont(-18, 600)
	a.fontTitle = makeFont(-30, 700)
	a.background, _, _ = win.GDI("CreateSolidBrush").Call(colorBackground)
	a.textColors = make(map[uintptr]uintptr)
}

func (a *application) setFont(hwnd, font uintptr) {
	if hwnd != 0 && font != 0 {
		win.Send(hwnd, win.WM_SETFONT, font, 1)
	}
}

func (a *application) staticText(text string, x, y, width, height int32, font, color uintptr) uintptr {
	h := win.CreateWindow("STATIC", text, win.WS_CHILD|win.WS_VISIBLE, x, y, width, height, a.hwnd, 0, a.instance)
	a.setFont(h, font)
	a.textColors[h] = color
	return h
}

func (a *application) controlColor(hdc, child uintptr) uintptr {
	color, ok := a.textColors[child]
	if !ok {
		color = colorText
	}
	win.GDI("SetTextColor").Call(hdc, color)
	win.GDI("SetBkMode").Call(hdc, 1)
	return a.background
}

func (a *application) releaseStyle() {
	for _, f := range []uintptr{a.fontBody, a.fontCaption, a.fontValue, a.fontHeading, a.fontTitle} {
		if f != 0 {
			win.GDI("DeleteObject").Call(f)
		}
	}
	if a.background != 0 {
		win.GDI("DeleteObject").Call(a.background)
	}
}
