//go:build !darwin

package main

import wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

func layoutPopupWindow(width, height, rightMargin, topMargin, _ int) {
	app := activeApp()
	if app == nil || app.ctx == nil {
		return
	}
	wailsruntime.WindowSetSize(app.ctx, width, height)
	wailsruntime.WindowSetPosition(app.ctx, rightMargin, topMargin)
}
