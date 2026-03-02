//go:build windows

package tray

import (
	"bytes"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"log/slog"
	"runtime"
	"sync"
	"syscall"
	"unsafe"

	"github.com/chrixbedardcad/GhostType/internal/version"
)

// Win32 constants.
const (
	wmDestroy  = 0x0002
	wmCommand  = 0x0111
	wmUser     = 0x0400
	wmTrayIcon = wmUser + 1

	wmLButtonUp = 0x0202
	wmRButtonUp = 0x0205

	nimAdd    = 0x00000000
	nimModify = 0x00000001
	nimDelete = 0x00000002

	nifMessage = 0x00000001
	nifIcon    = 0x00000002
	nifTip     = 0x00000004

	csOwnDC = 0x0020

	mfString    = 0x00000000
	mfSeparator = 0x00000800
	mfGrayed    = 0x00000001
	mfChecked   = 0x00000008

	tpmLeftAlign   = 0x0000
	tpmBottomAlign = 0x0020

	hwndMessage = ^uintptr(2) // HWND_MESSAGE = (HWND)-3

	idiApplication = 32512
)

// Menu item ID ranges.
const (
	idModeCorrect   = 2001
	idModeTranslate = 2002
	idModeRewrite   = 2003
	idExit          = 2099
	idLangBase      = 2100 // + language index
	idTemplBase     = 2200 // + template index
	idCancel        = 2098
	idSoundToggle   = 2300
	idSettings      = 2401
	idWizard        = 2402
)

// Win32 structs.
type wndClassExW struct {
	size       uint32
	style      uint32
	wndProc    uintptr
	clsExtra   int32
	wndExtra   int32
	instance   uintptr
	icon       uintptr
	cursor     uintptr
	background uintptr
	menuName   *uint16
	className  *uint16
	iconSm     uintptr
}

// notifyIconData matches the full Win32 NOTIFYICONDATAW (V3, Vista+).
// cbSize must equal unsafe.Sizeof of this struct for Shell_NotifyIconW to accept it.
type notifyIconData struct {
	size             uint32
	hwnd             uintptr
	id               uint32
	flags            uint32
	callbackMessage  uint32
	icon             uintptr
	tip              [128]uint16
	state            uint32
	stateMask        uint32
	info             [256]uint16
	versionOrTimeout uint32
	infoTitle        [64]uint16
	infoFlags        uint32
	guidItem         [16]byte
	balloonIcon      uintptr
}

type bitmapInfoHeader struct {
	size          uint32
	width         int32
	height        int32
	planes        uint16
	bitCount      uint16
	compression   uint32
	sizeImage     uint32
	xPelsPerMeter int32
	yPelsPerMeter int32
	clrUsed       uint32
	clrImportant  uint32
}

type iconInfo struct {
	icon     uint32  // BOOL: 1 = icon, 0 = cursor
	xHotspot uint32
	yHotspot uint32
	maskBM   uintptr // HBITMAP
	colorBM  uintptr // HBITMAP
}

type point struct {
	x, y int32
}

type msg struct {
	hwnd    uintptr
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	pt      point
}

// Win32 DLL procs.
var (
	user32   = syscall.NewLazyDLL("user32.dll")
	shell32  = syscall.NewLazyDLL("shell32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")

	procRegisterClassExW    = user32.NewProc("RegisterClassExW")
	procCreateWindowExW     = user32.NewProc("CreateWindowExW")
	procDestroyWindow       = user32.NewProc("DestroyWindow")
	procDefWindowProcW      = user32.NewProc("DefWindowProcW")
	procGetMessageW         = user32.NewProc("GetMessageW")
	procTranslateMessage    = user32.NewProc("TranslateMessage")
	procDispatchMessageW    = user32.NewProc("DispatchMessageW")
	procPostMessageW        = user32.NewProc("PostMessageW")
	procCreatePopupMenu     = user32.NewProc("CreatePopupMenu")
	procAppendMenuW         = user32.NewProc("AppendMenuW")
	procDestroyMenu         = user32.NewProc("DestroyMenu")
	procTrackPopupMenu      = user32.NewProc("TrackPopupMenu")
	procCheckMenuRadioItem  = user32.NewProc("CheckMenuRadioItem")
	procSetForegroundWindow = user32.NewProc("SetForegroundWindow")
	procGetCursorPos        = user32.NewProc("GetCursorPos")
	procLoadIconW           = user32.NewProc("LoadIconW")
	procPostQuitMessage     = user32.NewProc("PostQuitMessage")

	procGetDC               = user32.NewProc("GetDC")
	procReleaseDC           = user32.NewProc("ReleaseDC")
	procCreateIconIndirect  = user32.NewProc("CreateIconIndirect")
	procDestroyIcon         = user32.NewProc("DestroyIcon")

	procShellNotifyIconW = shell32.NewProc("Shell_NotifyIconW")

	procGetModuleHandleW = kernel32.NewProc("GetModuleHandleW")

	gdi32              = syscall.NewLazyDLL("gdi32.dll")
	procCreateDIBSection = gdi32.NewProc("CreateDIBSection")
	procCreateBitmap     = gdi32.NewProc("CreateBitmap")
	procDeleteObject     = gdi32.NewProc("DeleteObject")
)

// Config holds tray configuration and callbacks.
type Config struct {
	// IconPNG is the raw PNG bytes for the tray icon.
	// If nil, falls back to the default Windows application icon.
	IconPNG []byte

	// Callbacks — called on the tray's OS thread.
	OnModeChange   func(modeName string) // "correct", "translate", "rewrite"
	OnTargetSelect func(idx int)
	OnTemplSelect  func(idx int)
	OnSoundToggle  func(enabled bool)
	OnCancel       func()
	OnSettings     func()
	OnWizard       func()
	OnExit         func()

	// State readers — called to build the menu each time.
	GetActiveMode    func() string // returns "correct", "translate", or "rewrite"
	GetTargetIdx     func() int
	GetTemplateIdx   func() int
	GetSoundEnabled  func() bool
	GetIsProcessing  func() bool

	// Static data for building menu items.
	TargetLabels  []string // translate target display labels
	TemplateNames []string // rewrite template display names
}

// trayState holds the runtime state of the tray icon.
type trayState struct {
	cfg        Config
	hwnd       uintptr
	nid        notifyIconData
	customIcon uintptr // HICON created from PNG; destroyed on cleanup
	wg         sync.WaitGroup
	stopCh     chan struct{}
}

// global state — only one tray per process.
var (
	globalTray   *trayState
	globalTrayMu sync.Mutex
)

// Start launches the system tray icon in a background goroutine.
// Returns a stop function that removes the icon and stops the message loop.
func Start(cfg Config) (stop func()) {
	globalTrayMu.Lock()
	defer globalTrayMu.Unlock()

	ts := &trayState{
		cfg:    cfg,
		stopCh: make(chan struct{}),
	}
	globalTray = ts

	ts.wg.Add(1)
	go ts.run()

	return func() {
		select {
		case <-ts.stopCh:
			return // already stopped
		default:
		}
		// Post WM_DESTROY to break the message loop.
		if ts.hwnd != 0 {
			procPostMessageW.Call(ts.hwnd, wmDestroy, 0, 0)
		}
		ts.wg.Wait()
	}
}

func (ts *trayState) run() {
	defer ts.wg.Done()
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	hInstance, _, _ := procGetModuleHandleW.Call(0)

	// Load icon — try custom PNG first, fall back to default.
	var hIcon uintptr
	if len(ts.cfg.IconPNG) > 0 {
		var err error
		hIcon, err = loadIconFromPNG(ts.cfg.IconPNG)
		if err != nil {
			slog.Error("Failed to load custom icon, using default", "error", err)
			hIcon = 0
		} else {
			ts.customIcon = hIcon
		}
	}
	if hIcon == 0 {
		hIcon, _, _ = procLoadIconW.Call(0, uintptr(idiApplication))
	}

	// Register window class.
	className := utf16Ptr("GhostTypeTray")
	wc := wndClassExW{
		size:      uint32(unsafe.Sizeof(wndClassExW{})),
		style:     csOwnDC,
		wndProc:   wndProcCallback(),
		instance:  hInstance,
		icon:      hIcon,
		iconSm:    hIcon,
		className: className,
	}
	atom, _, err := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))
	if atom == 0 {
		slog.Error("RegisterClassExW failed", "error", err)
		return
	}

	// Create message-only window.
	ts.hwnd, _, err = procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(utf16Ptr("GhostType"))),
		0, 0, 0, 0, 0,
		hwndMessage, 0, hInstance, 0,
	)
	if ts.hwnd == 0 {
		slog.Error("CreateWindowExW failed", "error", err)
		return
	}

	// Add tray icon.
	nidSize := unsafe.Sizeof(notifyIconData{})
	slog.Debug("NOTIFYICONDATAW size", "bytes", nidSize)
	ts.nid = notifyIconData{
		size:            uint32(nidSize),
		hwnd:            ts.hwnd,
		id:              1,
		flags:           nifMessage | nifIcon | nifTip,
		callbackMessage: wmTrayIcon,
		icon:            hIcon,
	}
	ts.setTooltip()
	ret, _, err := procShellNotifyIconW.Call(nimAdd, uintptr(unsafe.Pointer(&ts.nid)))
	if ret == 0 {
		slog.Error("Shell_NotifyIconW(NIM_ADD) failed", "error", err, "nid_size", nidSize, "hwnd", ts.hwnd, "icon", hIcon)
		return
	}
	slog.Info("System tray icon added")

	// Message loop.
	var m msg
	for {
		ret, _, _ := procGetMessageW.Call(
			uintptr(unsafe.Pointer(&m)),
			0, 0, 0,
		)
		// GetMessageW returns 0 for WM_QUIT, -1 for error.
		if ret == 0 || int32(ret) == -1 {
			break
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
	}

	// Cleanup.
	procShellNotifyIconW.Call(nimDelete, uintptr(unsafe.Pointer(&ts.nid)))
	procDestroyWindow.Call(ts.hwnd)
	if ts.customIcon != 0 {
		procDestroyIcon.Call(ts.customIcon)
	}
	close(ts.stopCh)
}

func (ts *trayState) setTooltip() {
	tip := fmt.Sprintf("GhostType v%s - %s", version.Version, ts.cfg.GetActiveMode())
	tipUTF16 := utf16Slice(tip)
	n := len(tipUTF16)
	if n > 127 {
		n = 127
	}
	copy(ts.nid.tip[:n], tipUTF16[:n])
	ts.nid.tip[n] = 0
}

func (ts *trayState) updateTooltip() {
	ts.setTooltip()
	ts.nid.flags = nifTip
	procShellNotifyIconW.Call(nimModify, uintptr(unsafe.Pointer(&ts.nid)))
	ts.nid.flags = nifMessage | nifIcon | nifTip
}

func (ts *trayState) showMenu() {
	hMenu, _, _ := procCreatePopupMenu.Call()
	if hMenu == 0 {
		return
	}

	// Version header (grayed).
	header := fmt.Sprintf("GhostType v%s", version.Version)
	procAppendMenuW.Call(hMenu, mfString|mfGrayed, 0, uintptr(unsafe.Pointer(utf16Ptr(header))))
	procAppendMenuW.Call(hMenu, mfSeparator, 0, 0)

	// Mode items.
	activeMode := ts.cfg.GetActiveMode()
	procAppendMenuW.Call(hMenu, mfString, idModeCorrect, uintptr(unsafe.Pointer(utf16Ptr("Correct"))))
	procAppendMenuW.Call(hMenu, mfString, idModeTranslate, uintptr(unsafe.Pointer(utf16Ptr("Translate"))))
	procAppendMenuW.Call(hMenu, mfString, idModeRewrite, uintptr(unsafe.Pointer(utf16Ptr("Rewrite"))))

	// Radio-check the active mode.
	activeID := uint32(idModeCorrect)
	switch activeMode {
	case "translate":
		activeID = idModeTranslate
	case "rewrite":
		activeID = idModeRewrite
	}
	procCheckMenuRadioItem.Call(hMenu,
		uintptr(idModeCorrect), uintptr(idModeRewrite),
		uintptr(activeID), 0) // 0 = MF_BYCOMMAND

	// Language/target section.
	if len(ts.cfg.TargetLabels) > 0 {
		procAppendMenuW.Call(hMenu, mfSeparator, 0, 0)
		procAppendMenuW.Call(hMenu, mfString|mfGrayed, 0, uintptr(unsafe.Pointer(utf16Ptr("Language:"))))

		targetIdx := ts.cfg.GetTargetIdx()
		for i, name := range ts.cfg.TargetLabels {
			label := "  " + name
			procAppendMenuW.Call(hMenu, mfString, uintptr(idLangBase+i), uintptr(unsafe.Pointer(utf16Ptr(label))))
		}
		procCheckMenuRadioItem.Call(hMenu,
			uintptr(idLangBase), uintptr(idLangBase+len(ts.cfg.TargetLabels)-1),
			uintptr(idLangBase+targetIdx), 0)
	}

	// Template section.
	if len(ts.cfg.TemplateNames) > 0 {
		procAppendMenuW.Call(hMenu, mfSeparator, 0, 0)
		procAppendMenuW.Call(hMenu, mfString|mfGrayed, 0, uintptr(unsafe.Pointer(utf16Ptr("Template:"))))

		templIdx := ts.cfg.GetTemplateIdx()
		for i, name := range ts.cfg.TemplateNames {
			label := "  " + name
			procAppendMenuW.Call(hMenu, mfString, uintptr(idTemplBase+i), uintptr(unsafe.Pointer(utf16Ptr(label))))
		}
		procCheckMenuRadioItem.Call(hMenu,
			uintptr(idTemplBase), uintptr(idTemplBase+len(ts.cfg.TemplateNames)-1),
			uintptr(idTemplBase+templIdx), 0)
	}

	// Sound toggle.
	procAppendMenuW.Call(hMenu, mfSeparator, 0, 0)
	soundFlags := uintptr(mfString)
	if ts.cfg.GetSoundEnabled != nil && ts.cfg.GetSoundEnabled() {
		soundFlags |= mfChecked
	}
	procAppendMenuW.Call(hMenu, soundFlags, idSoundToggle, uintptr(unsafe.Pointer(utf16Ptr("Sound"))))

	// Cancel — only enabled when an LLM call is in progress.
	cancelFlags := uintptr(mfString | mfGrayed)
	if ts.cfg.GetIsProcessing != nil && ts.cfg.GetIsProcessing() {
		cancelFlags = uintptr(mfString)
	}
	procAppendMenuW.Call(hMenu, cancelFlags, idCancel, uintptr(unsafe.Pointer(utf16Ptr("Cancel LLM"))))

	// Settings & Wizard.
	procAppendMenuW.Call(hMenu, mfSeparator, 0, 0)
	procAppendMenuW.Call(hMenu, mfString, idSettings, uintptr(unsafe.Pointer(utf16Ptr("Settings..."))))
	procAppendMenuW.Call(hMenu, mfString, idWizard, uintptr(unsafe.Pointer(utf16Ptr("Setup Wizard..."))))

	// Exit.
	procAppendMenuW.Call(hMenu, mfSeparator, 0, 0)
	procAppendMenuW.Call(hMenu, mfString, idExit, uintptr(unsafe.Pointer(utf16Ptr("Exit"))))

	// Show menu at cursor.
	var pt point
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	procSetForegroundWindow.Call(ts.hwnd)
	procTrackPopupMenu.Call(hMenu, tpmLeftAlign|tpmBottomAlign, uintptr(pt.x), uintptr(pt.y), 0, ts.hwnd, 0)

	procDestroyMenu.Call(hMenu)
}

func (ts *trayState) handleMenuCommand(id int) {
	// Exact-ID matches first, then open-ended ranges last.
	// This prevents ranges (e.g. id >= idTemplBase) from swallowing
	// higher fixed IDs like idSoundToggle, idSettings, idWizard.
	switch {
	case id == idModeCorrect:
		if ts.cfg.OnModeChange != nil {
			ts.cfg.OnModeChange("correct")
		}
		ts.updateTooltip()

	case id == idModeTranslate:
		if ts.cfg.OnModeChange != nil {
			ts.cfg.OnModeChange("translate")
		}
		ts.updateTooltip()

	case id == idModeRewrite:
		if ts.cfg.OnModeChange != nil {
			ts.cfg.OnModeChange("rewrite")
		}
		ts.updateTooltip()

	case id == idCancel:
		if ts.cfg.OnCancel != nil {
			ts.cfg.OnCancel()
		}

	case id == idSoundToggle:
		if ts.cfg.OnSoundToggle != nil && ts.cfg.GetSoundEnabled != nil {
			newState := !ts.cfg.GetSoundEnabled()
			ts.cfg.OnSoundToggle(newState)
		}

	case id == idSettings:
		if ts.cfg.OnSettings != nil {
			ts.cfg.OnSettings()
		}

	case id == idWizard:
		if ts.cfg.OnWizard != nil {
			ts.cfg.OnWizard()
		}

	case id == idExit:
		if ts.cfg.OnExit != nil {
			ts.cfg.OnExit()
		}
		procPostMessageW.Call(ts.hwnd, wmDestroy, 0, 0)

	// Open-ended ranges last — these must not swallow fixed IDs above.
	case id >= idLangBase && id < idTemplBase:
		idx := id - idLangBase
		if ts.cfg.OnTargetSelect != nil {
			ts.cfg.OnTargetSelect(idx)
		}
		ts.updateTooltip()

	case id >= idTemplBase:
		idx := id - idTemplBase
		if ts.cfg.OnTemplSelect != nil {
			ts.cfg.OnTemplSelect(idx)
		}
		ts.updateTooltip()
	}
}

// wndProcCallback returns a syscall callback for the window procedure.
func wndProcCallback() uintptr {
	return syscall.NewCallback(func(hwnd, umsg, wParam, lParam uintptr) uintptr {
		globalTrayMu.Lock()
		ts := globalTray
		globalTrayMu.Unlock()

		switch umsg {
		case wmTrayIcon:
			switch lParam {
			case wmRButtonUp, wmLButtonUp:
				if ts != nil {
					ts.showMenu()
				}
			}
			return 0

		case wmCommand:
			id := int(wParam & 0xFFFF)
			if ts != nil {
				ts.handleMenuCommand(id)
			}
			return 0

		case wmDestroy:
			procPostQuitMessage.Call(0)
			return 0
		}

		ret, _, _ := procDefWindowProcW.Call(hwnd, umsg, wParam, lParam)
		return ret
	})
}

// loadIconFromPNG decodes a PNG from raw bytes and creates a Win32 HICON.
// The caller owns the returned HICON and must call DestroyIcon when done.
func loadIconFromPNG(data []byte) (uintptr, error) {
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return 0, fmt.Errorf("decode PNG: %w", err)
	}

	// Convert to RGBA regardless of source format.
	bounds := img.Bounds()
	rgba := image.NewRGBA(bounds)
	draw.Draw(rgba, bounds, img, bounds.Min, draw.Src)

	w := bounds.Dx()
	h := bounds.Dy()

	// Create a 32-bit top-down DIB section (BGRA with pre-multiplied alpha).
	bi := bitmapInfoHeader{
		size:     40, // sizeof(BITMAPINFOHEADER)
		width:    int32(w),
		height:   -int32(h), // negative = top-down
		planes:   1,
		bitCount: 32,
	}

	hdc, _, _ := procGetDC.Call(0)
	var bits uintptr
	hBitmap, _, _ := procCreateDIBSection.Call(
		hdc,
		uintptr(unsafe.Pointer(&bi)),
		0, // DIB_RGB_COLORS
		uintptr(unsafe.Pointer(&bits)),
		0, 0,
	)
	procReleaseDC.Call(0, hdc)

	if hBitmap == 0 {
		return 0, fmt.Errorf("CreateDIBSection failed")
	}

	// Copy RGBA pixels to pre-multiplied BGRA.
	pixelCount := w * h
	src := rgba.Pix
	dst := unsafe.Slice((*byte)(unsafe.Pointer(bits)), pixelCount*4)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			si := y*rgba.Stride + x*4
			di := (y*w + x) * 4
			a := uint16(src[si+3])
			if a == 0 {
				dst[di+0] = 0
				dst[di+1] = 0
				dst[di+2] = 0
				dst[di+3] = 0
			} else if a == 255 {
				dst[di+0] = src[si+2] // B
				dst[di+1] = src[si+1] // G
				dst[di+2] = src[si+0] // R
				dst[di+3] = 255
			} else {
				dst[di+0] = byte(uint16(src[si+2]) * a / 255) // B
				dst[di+1] = byte(uint16(src[si+1]) * a / 255) // G
				dst[di+2] = byte(uint16(src[si+0]) * a / 255) // R
				dst[di+3] = byte(a)
			}
		}
	}

	// Create monochrome mask bitmap (all zeros = fully opaque; alpha handles transparency).
	hMask, _, _ := procCreateBitmap.Call(uintptr(w), uintptr(h), 1, 1, 0)
	if hMask == 0 {
		procDeleteObject.Call(hBitmap)
		return 0, fmt.Errorf("CreateBitmap (mask) failed")
	}

	// Combine into HICON.
	ii := iconInfo{
		icon:    1, // TRUE = icon
		maskBM:  hMask,
		colorBM: hBitmap,
	}
	hIcon, _, _ := procCreateIconIndirect.Call(uintptr(unsafe.Pointer(&ii)))

	// Clean up intermediate bitmaps (CreateIconIndirect copies them).
	procDeleteObject.Call(hBitmap)
	procDeleteObject.Call(hMask)

	if hIcon == 0 {
		return 0, fmt.Errorf("CreateIconIndirect failed")
	}

	return hIcon, nil
}

// utf16Ptr converts a Go string to a null-terminated UTF-16 pointer.
func utf16Ptr(s string) *uint16 {
	u := utf16Slice(s)
	u = append(u, 0)
	return &u[0]
}

// utf16Slice converts a Go string to a UTF-16 slice (no null terminator).
func utf16Slice(s string) []uint16 {
	result := make([]uint16, 0, len(s)+1)
	for _, r := range s {
		if r <= 0xFFFF {
			result = append(result, uint16(r))
		} else {
			r -= 0x10000
			result = append(result, uint16(0xD800+(r>>10)), uint16(0xDC00+(r&0x3FF)))
		}
	}
	return result
}
