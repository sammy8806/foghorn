//go:build !darwin

package main

import wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

func layoutPopupWindow(width, height, x, y, _ int, _ string) {
	app := activeApp()
	if app == nil || app.ctx == nil {
		return
	}
	wailsruntime.WindowSetSize(app.ctx, width, height)
	wailsruntime.WindowSetPosition(app.ctx, x, y)
}
