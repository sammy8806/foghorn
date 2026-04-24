//go:build windows

package tray

import (
	"bytes"
	"crypto/md5"
	"encoding/binary"
	"encoding/hex"
	"image"
	_ "image/png"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	wmCommand     = 0x0111
	wmDestroy     = 0x0002
	wmUser        = 0x0400
	wmLButtonUp   = 0x0202
	wmRButtonUp   = 0x0205
	wmTrayMessage = wmUser + 1

	nimAdd    = 0x00000000
	nimModify = 0x00000001
	nimDelete = 0x00000002

	nifMessage = 0x00000001
	nifIcon    = 0x00000002
	nifTip     = 0x00000004

	imageIcon      = 1
	lrLoadFromFile = 0x00000010
	lrDefaultSize  = 0x00000040
	tpmBottomAlign = 0x0020
	tpmLeftAlign   = 0x0000
	mfString       = 0x00000000
	mfSeparator    = 0x00000800
	menuShowHide   = 1001
	menuQuit       = 1002
	trayID         = 100
)

var (
	kernel32 = windows.NewLazySystemDLL("Kernel32.dll")
	shell32  = windows.NewLazySystemDLL("Shell32.dll")
	user32   = windows.NewLazySystemDLL("User32.dll")

	procGetModuleHandle       = kernel32.NewProc("GetModuleHandleW")
	procShellNotifyIcon       = shell32.NewProc("Shell_NotifyIconW")
	procAppendMenu            = user32.NewProc("AppendMenuW")
	procCreatePopupMenu       = user32.NewProc("CreatePopupMenu")
	procCreateWindowEx        = user32.NewProc("CreateWindowExW")
	procDefWindowProc         = user32.NewProc("DefWindowProcW")
	procDestroyMenu           = user32.NewProc("DestroyMenu")
	procDestroyWindow         = user32.NewProc("DestroyWindow")
	procDestroyIcon           = user32.NewProc("DestroyIcon")
	procDispatchMessage       = user32.NewProc("DispatchMessageW")
	procGetCursorPos          = user32.NewProc("GetCursorPos")
	procGetMessage            = user32.NewProc("GetMessageW")
	procLoadCursor            = user32.NewProc("LoadCursorW")
	procLoadIcon              = user32.NewProc("LoadIconW")
	procLoadImage             = user32.NewProc("LoadImageW")
	procPostMessage           = user32.NewProc("PostMessageW")
	procPostQuitMessage       = user32.NewProc("PostQuitMessage")
	procRegisterClass         = user32.NewProc("RegisterClassExW")
	procRegisterWindowMessage = user32.NewProc("RegisterWindowMessageW")
	procSetForegroundWindow   = user32.NewProc("SetForegroundWindow")
	procShowWindow            = user32.NewProc("ShowWindow")
	procTrackPopupMenu        = user32.NewProc("TrackPopupMenu")
	procTranslateMessage      = user32.NewProc("TranslateMessage")
	procUnregisterClass       = user32.NewProc("UnregisterClassW")
	procUpdateWindow          = user32.NewProc("UpdateWindow")

	currentWindowsTray *windowsTray
)

type wndClassEx struct {
	Size, Style                        uint32
	WndProc                            uintptr
	ClsExtra, WndExtra                 int32
	Instance, Icon, Cursor, Background windows.Handle
	MenuName, ClassName                *uint16
	IconSm                             windows.Handle
}

type notifyIconData struct {
	Size                       uint32
	Wnd                        windows.Handle
	ID, Flags, CallbackMessage uint32
	Icon                       windows.Handle
	Tip                        [128]uint16
	State, StateMask           uint32
	Info                       [256]uint16
	Timeout, Version           uint32
	InfoTitle                  [64]uint16
	InfoFlags                  uint32
	GuidItem                   windows.GUID
	BalloonIcon                windows.Handle
}

type point struct {
	X, Y int32
}

type message struct {
	WindowHandle windows.Handle
	Message      uint32
	Wparam       uintptr
	Lparam       uintptr
	Time         uint32
	Pt           point
}

func Supported() bool {
	return true
}

func StartHiddenByDefault() bool {
	return false
}

type windowsTray struct {
	mu             sync.Mutex
	manager        *Manager
	ready          bool
	disposed       bool
	icon           []byte
	iconHash       string
	tooltip        string
	instance       windows.Handle
	window         windows.Handle
	menu           windows.Handle
	className      *uint16
	taskbarCreated uint32
	nid            notifyIconData
}

func newPlatformTray(m *Manager) platformTray {
	wt := &windowsTray{manager: m}
	go wt.run()
	return wt
}

func (w *windowsTray) update(icon []byte, tooltip string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.disposed {
		return nil
	}
	w.icon = icon
	w.tooltip = tooltip
	if !w.ready {
		return nil
	}
	return w.updateNotifyIconLocked(nimModify)
}

func (w *windowsTray) dispose() {
	w.mu.Lock()
	if w.disposed {
		w.mu.Unlock()
		return
	}
	w.disposed = true
	window := w.window
	w.mu.Unlock()

	if window != 0 {
		procPostMessage.Call(uintptr(window), wmDestroy, 0, 0)
	}
}

func (w *windowsTray) run() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	currentWindowsTray = w
	if err := w.init(); err != nil {
		log.Printf("tray: Windows tray initialization failed: %v", err)
		w.cleanup()
		return
	}
	defer w.cleanup()

	w.mu.Lock()
	w.ready = true
	_ = w.updateNotifyIconLocked(nimAdd)
	w.mu.Unlock()

	msg := &message{}
	for {
		ret, _, _ := procGetMessage.Call(uintptr(unsafe.Pointer(msg)), 0, 0, 0)
		if int32(ret) <= 0 {
			return
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(msg)))
		procDispatchMessage.Call(uintptr(unsafe.Pointer(msg)))
	}
}

func (w *windowsTray) init() error {
	const (
		idiApplication = 32512
		idcArrow       = 32512
		swHide         = 0
	)

	instance, _, err := procGetModuleHandle.Call(0)
	if instance == 0 {
		return err
	}
	w.instance = windows.Handle(instance)

	icon, _, err := procLoadIcon.Call(0, uintptr(idiApplication))
	if icon == 0 {
		return err
	}
	cursor, _, err := procLoadCursor.Call(0, uintptr(idcArrow))
	if cursor == 0 {
		return err
	}

	className, err := windows.UTF16PtrFromString("FoghornTrayWindow")
	if err != nil {
		return err
	}
	w.className = className

	wcex := wndClassEx{
		Style:      0x0002 | 0x0001,
		WndProc:    windows.NewCallback(windowsTrayWndProc),
		Instance:   w.instance,
		Icon:       windows.Handle(icon),
		Cursor:     windows.Handle(cursor),
		Background: windows.Handle(6),
		ClassName:  className,
		IconSm:     windows.Handle(icon),
	}
	wcex.Size = uint32(unsafe.Sizeof(wcex))
	if res, _, err := procRegisterClass.Call(uintptr(unsafe.Pointer(&wcex))); res == 0 {
		return err
	}

	window, _, err := procCreateWindowEx.Call(
		0,
		uintptr(unsafe.Pointer(className)),
		0,
		0,
		0,
		0,
		0,
		0,
		0,
		0,
		uintptr(w.instance),
		0,
	)
	if window == 0 {
		return err
	}
	w.window = windows.Handle(window)

	procShowWindow.Call(uintptr(w.window), swHide)
	procUpdateWindow.Call(uintptr(w.window))

	taskbarEventName, _ := windows.UTF16PtrFromString("TaskbarCreated")
	taskbarCreated, _, _ := procRegisterWindowMessage.Call(uintptr(unsafe.Pointer(taskbarEventName)))
	w.taskbarCreated = uint32(taskbarCreated)

	menu, _, err := procCreatePopupMenu.Call()
	if menu == 0 {
		return err
	}
	w.menu = windows.Handle(menu)
	if err := appendMenu(w.menu, mfString, menuShowHide, "Show or Hide Window"); err != nil {
		return err
	}
	if err := appendMenu(w.menu, mfSeparator, 0, ""); err != nil {
		return err
	}
	if err := appendMenu(w.menu, mfString, menuQuit, "Quit Foghorn"); err != nil {
		return err
	}

	w.nid = notifyIconData{
		Wnd:             w.window,
		ID:              trayID,
		Flags:           nifMessage,
		CallbackMessage: wmTrayMessage,
	}
	w.nid.Size = uint32(unsafe.Sizeof(w.nid))
	return nil
}

func (w *windowsTray) cleanup() {
	w.mu.Lock()
	if w.nid.Wnd != 0 {
		procShellNotifyIcon.Call(nimDelete, uintptr(unsafe.Pointer(&w.nid)))
		w.nid.Wnd = 0
	}
	if w.nid.Icon != 0 {
		destroyIcon(w.nid.Icon)
		w.nid.Icon = 0
		w.iconHash = ""
	}
	if w.menu != 0 {
		procDestroyMenu.Call(uintptr(w.menu))
		w.menu = 0
	}
	if w.window != 0 {
		procDestroyWindow.Call(uintptr(w.window))
		w.window = 0
	}
	if w.className != nil && w.instance != 0 {
		procUnregisterClass.Call(uintptr(unsafe.Pointer(w.className)), uintptr(w.instance))
	}
	w.mu.Unlock()
}

func (w *windowsTray) updateNotifyIconLocked(action uintptr) error {
	var oldIcon windows.Handle
	var newIcon windows.Handle
	oldIconHash := w.iconHash
	newIconHash := oldIconHash

	if len(w.icon) > 0 {
		newIconHash = hashIconBytes(w.icon)
		if w.nid.Icon == 0 || newIconHash != w.iconHash {
			iconPath, err := iconBytesToFilePath(w.icon)
			if err != nil {
				return err
			}
			icon, err := loadIcon(iconPath)
			if err != nil {
				return err
			}
			oldIcon = w.nid.Icon
			newIcon = icon
			w.nid.Icon = newIcon
		}
		w.nid.Flags |= nifIcon
	}

	if w.tooltip != "" {
		tooltip, err := windows.UTF16FromString(w.tooltip)
		if err != nil {
			return err
		}
		clear(w.nid.Tip[:])
		copy(w.nid.Tip[:], tooltip)
		w.nid.Flags |= nifTip
	}

	w.nid.Size = uint32(unsafe.Sizeof(w.nid))
	if res, _, err := procShellNotifyIcon.Call(action, uintptr(unsafe.Pointer(&w.nid))); res == 0 {
		if newIcon != 0 {
			destroyIcon(newIcon)
			w.nid.Icon = oldIcon
			w.iconHash = oldIconHash
		}
		return err
	}
	if newIcon != 0 {
		w.iconHash = newIconHash
		if oldIcon != 0 {
			destroyIcon(oldIcon)
		}
	}
	return nil
}

func (w *windowsTray) showMenu() {
	cursor := point{}
	if res, _, _ := procGetCursorPos.Call(uintptr(unsafe.Pointer(&cursor))); res == 0 {
		return
	}
	procSetForegroundWindow.Call(uintptr(w.window))
	procTrackPopupMenu.Call(
		uintptr(w.menu),
		tpmBottomAlign|tpmLeftAlign,
		uintptr(cursor.X),
		uintptr(cursor.Y),
		0,
		uintptr(w.window),
		0,
	)
}

func windowsTrayWndProc(hwnd windows.Handle, msg uint32, wParam, lParam uintptr) uintptr {
	w := currentWindowsTray
	if w == nil {
		result, _, _ := procDefWindowProc.Call(uintptr(hwnd), uintptr(msg), wParam, lParam)
		return result
	}

	switch msg {
	case wmCommand:
		switch uint16(wParam) {
		case menuShowHide:
			w.manager.handleClick()
		case menuQuit:
			w.manager.handleQuit()
		}
		return 0
	case wmTrayMessage:
		switch uint32(lParam) {
		case wmLButtonUp:
			w.manager.handleClick()
		case wmRButtonUp:
			w.showMenu()
		}
		return 0
	case w.taskbarCreated:
		w.mu.Lock()
		_ = w.updateNotifyIconLocked(nimAdd)
		w.mu.Unlock()
		return 0
	case wmDestroy:
		procPostQuitMessage.Call(0)
		return 0
	default:
		result, _, _ := procDefWindowProc.Call(uintptr(hwnd), uintptr(msg), wParam, lParam)
		return result
	}
}

func appendMenu(menu windows.Handle, flags uintptr, id uintptr, title string) error {
	var titlePtr *uint16
	var err error
	if title != "" {
		titlePtr, err = windows.UTF16PtrFromString(title)
		if err != nil {
			return err
		}
	}
	if res, _, err := procAppendMenu.Call(uintptr(menu), flags, id, uintptr(unsafe.Pointer(titlePtr))); res == 0 {
		return err
	}
	return nil
}

func loadIcon(path string) (windows.Handle, error) {
	pathPtr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}
	res, _, err := procLoadImage.Call(
		0,
		uintptr(unsafe.Pointer(pathPtr)),
		imageIcon,
		0,
		0,
		lrLoadFromFile|lrDefaultSize,
	)
	if res == 0 {
		return 0, err
	}
	return windows.Handle(res), nil
}

func destroyIcon(icon windows.Handle) {
	procDestroyIcon.Call(uintptr(icon))
}

func hashIconBytes(icon []byte) string {
	hash := md5.Sum(icon)
	return hex.EncodeToString(hash[:])
}

func iconBytesToFilePath(icon []byte) (string, error) {
	icoBytes, err := pngToICO(icon)
	if err != nil {
		return "", err
	}

	hash := md5.Sum(icoBytes)
	path := filepath.Join(os.TempDir(), "foghorn_tray_icon_"+hex.EncodeToString(hash[:])+".ico")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := os.WriteFile(path, icoBytes, 0644); err != nil {
			return "", err
		}
	}
	return path, nil
}

func pngToICO(pngBytes []byte) ([]byte, error) {
	if len(pngBytes) == 0 {
		return nil, nil
	}

	cfg, _, err := image.DecodeConfig(bytes.NewReader(pngBytes))
	if err != nil {
		return nil, err
	}

	width := byte(cfg.Width)
	if cfg.Width >= 256 {
		width = 0
	}
	height := byte(cfg.Height)
	if cfg.Height >= 256 {
		height = 0
	}

	var out bytes.Buffer
	// ICONDIR.
	if err := binary.Write(&out, binary.LittleEndian, uint16(0)); err != nil {
		return nil, err
	}
	if err := binary.Write(&out, binary.LittleEndian, uint16(1)); err != nil {
		return nil, err
	}
	if err := binary.Write(&out, binary.LittleEndian, uint16(1)); err != nil {
		return nil, err
	}

	// ICONDIRENTRY. Windows supports PNG-compressed ICO images.
	out.WriteByte(width)
	out.WriteByte(height)
	out.WriteByte(0)
	out.WriteByte(0)
	if err := binary.Write(&out, binary.LittleEndian, uint16(1)); err != nil {
		return nil, err
	}
	if err := binary.Write(&out, binary.LittleEndian, uint16(32)); err != nil {
		return nil, err
	}
	if err := binary.Write(&out, binary.LittleEndian, uint32(len(pngBytes))); err != nil {
		return nil, err
	}
	if err := binary.Write(&out, binary.LittleEndian, uint32(22)); err != nil {
		return nil, err
	}

	out.Write(pngBytes)
	return out.Bytes(), nil
}
