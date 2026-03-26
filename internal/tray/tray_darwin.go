//go:build darwin

package tray

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa

#include <stdint.h>
#include <stdlib.h>
void *foghornTrayCreate(uintptr_t handle);
void foghornTrayUpdate(void *targetRef, void *bytes, int length, const char *tooltip);
void foghornTrayDispose(void *targetRef);
*/
import "C"

import (
	"runtime/cgo"
	"unsafe"
)

func Supported() bool {
	return true
}

type darwinTray struct {
	handle cgo.Handle
	target unsafe.Pointer
}

func newPlatformTray(m *Manager) platformTray {
	handle := cgo.NewHandle(m)
	target := C.foghornTrayCreate(C.uintptr_t(handle))
	return &darwinTray{
		handle: handle,
		target: target,
	}
}

func (d *darwinTray) update(icon []byte, tooltip string) error {
	if len(icon) == 0 || d == nil || d.target == nil {
		return nil
	}
	cTooltip := C.CString(tooltip)
	defer C.free(unsafe.Pointer(cTooltip))

	C.foghornTrayUpdate(
		d.target,
		unsafe.Pointer(&icon[0]),
		C.int(len(icon)),
		cTooltip,
	)
	return nil
}

func (d *darwinTray) dispose() {
	if d == nil {
		return
	}
	if d.target != nil {
		C.foghornTrayDispose(d.target)
		d.target = nil
	}
	if d.handle != 0 {
		d.handle.Delete()
		d.handle = 0
	}
}

//export trayHandleClick
func trayHandleClick(handle C.uintptr_t) {
	manager, ok := cgo.Handle(handle).Value().(*Manager)
	if !ok {
		return
	}
	manager.handleClick()
}

//export trayHandleQuit
func trayHandleQuit(handle C.uintptr_t) {
	manager, ok := cgo.Handle(handle).Value().(*Manager)
	if !ok {
		return
	}
	manager.handleQuit()
}
