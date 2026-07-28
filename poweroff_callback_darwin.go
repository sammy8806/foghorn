//go:build darwin

package main

// This file only holds the cgo-exported callback for poweroff_darwin.go:
// a file using //export may not define C functions in its preamble, so the
// export lives here, separate from the Objective-C implementation.

import "C"

//export foghornHandlePowerOff
func foghornHandlePowerOff() {
	if powerOffHandler != nil {
		powerOffHandler()
	}
}
