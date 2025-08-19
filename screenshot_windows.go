//go:build windows

// Файл: screenshot_windows.go
package main

import (
	"fmt"
	"image"
	"syscall"
	"unsafe"
)

// --- Загрузка функций Windows API ---
var (
	user32 = syscall.NewLazyDLL("user32.dll")
	gdi32  = syscall.NewLazyDLL("gdi32.dll")

	procGetSystemMetrics       = user32.NewProc("GetSystemMetrics")
	procGetDC                  = user32.NewProc("GetDC")
	procReleaseDC              = user32.NewProc("ReleaseDC")
	procCreateCompatibleDC     = gdi32.NewProc("CreateCompatibleDC")
	procCreateCompatibleBitmap = gdi32.NewProc("CreateCompatibleBitmap")
	procSelectObject           = gdi32.NewProc("SelectObject")
	procBitBlt                 = gdi32.NewProc("BitBlt")
	procDeleteDC               = gdi32.NewProc("DeleteDC")
	procDeleteObject           = gdi32.NewProc("DeleteObject")
	procGetDIBits              = gdi32.NewProc("GetDIBits")
)

// Константы для Windows API
const (
	SM_XVIRTUALSCREEN  = 76
	SM_YVIRTUALSCREEN  = 77
	SM_CXVIRTUALSCREEN = 78
	SM_CYVIRTUALSCREEN = 79
	SRCCOPY            = 0x00CC0020
	BI_RGB             = 0
	DIB_RGB_COLORS     = 0
)

// Структура BITMAPINFOHEADER
type bitmapInfoHeader struct {
	Size          uint32
	Width         int32
	Height        int32
	Planes        uint16
	BitCount      uint16
	Compression   uint32
	SizeImage     uint32
	XPelsPerMeter int32
	YPelsPerMeter int32
	ClrUsed       uint32
	ClrImportant  uint32
}

// captureScreen делает скриншот всего экрана и возвращает его как объект image.Image
func captureScreen() (*image.RGBA, error) {
	x, _, _ := procGetSystemMetrics.Call(SM_XVIRTUALSCREEN)
	y, _, _ := procGetSystemMetrics.Call(SM_YVIRTUALSCREEN)
	width, _, _ := procGetSystemMetrics.Call(SM_CXVIRTUALSCREEN)
	height, _, _ := procGetSystemMetrics.Call(SM_CYVIRTUALSCREEN)

	screenDC, _, _ := procGetDC.Call(0)
	if screenDC == 0 {
		return nil, fmt.Errorf("GetDC(0) failed")
	}
	defer procReleaseDC.Call(0, screenDC)

	memDC, _, _ := procCreateCompatibleDC.Call(screenDC)
	if memDC == 0 {
		return nil, fmt.Errorf("CreateCompatibleDC failed")
	}
	defer procDeleteDC.Call(memDC)

	memBitmap, _, _ := procCreateCompatibleBitmap.Call(screenDC, width, height)
	if memBitmap == 0 {
		return nil, fmt.Errorf("CreateCompatibleBitmap failed")
	}
	defer procDeleteObject.Call(memBitmap)

	procSelectObject.Call(memDC, memBitmap)
	procBitBlt.Call(memDC, 0, 0, width, height, screenDC, x, y, SRCCOPY)

	var bi bitmapInfoHeader
	bi.Size = uint32(unsafe.Sizeof(bi))
	bi.Width = int32(width)
	bi.Height = -int32(height) // Отрицательная высота для top-down DIB
	bi.Planes = 1
	bi.BitCount = 32
	bi.Compression = BI_RGB

	bitmapData := make([]byte, int(width*height*4))
	procGetDIBits.Call(memDC, memBitmap, 0, uintptr(height), uintptr(unsafe.Pointer(&bitmapData[0])), uintptr(unsafe.Pointer(&bi)), DIB_RGB_COLORS)

	img := image.NewRGBA(image.Rect(0, 0, int(width), int(height)))
	for i := 0; i < int(width*height); i++ {
		img.Pix[i*4+0] = bitmapData[i*4+2] // B
		img.Pix[i*4+1] = bitmapData[i*4+1] // G
		img.Pix[i*4+2] = bitmapData[i*4+0] // R
		img.Pix[i*4+3] = 255               // A
	}
	return img, nil
}
