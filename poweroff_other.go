//go:build !darwin

package main

func watchSystemPowerOff(handler func()) {
	_ = handler
}
