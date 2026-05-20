//go:build !darwin

package main

func setDockIconVisible(visible bool) {
	_ = visible
}
