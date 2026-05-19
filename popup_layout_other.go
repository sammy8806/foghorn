//go:build !darwin

package main

import wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

func layoutPopupWindow(width, height, horizontalMargin, topMargin, bottomMargin int, position string) {
	app := activeApp()
	if app == nil || app.ctx == nil {
		return
	}

	screenWidth, screenHeight := currentScreenSize()
	width = clampInt(width, 240, maxInt(240, screenWidth-horizontalMargin*2))
	height = clampInt(height, 200, maxInt(200, screenHeight-topMargin-bottomMargin))

	alignLeft := position == "top_left" || position == "bottom_left"
	alignBottom := position == "bottom_left" || position == "bottom_right"
	x := screenWidth - width - horizontalMargin
	if alignLeft {
		x = horizontalMargin
	}
	y := topMargin
	if alignBottom {
		y = screenHeight - height - bottomMargin
	}

	wailsruntime.WindowSetSize(app.ctx, width, height)
	wailsruntime.WindowSetPosition(app.ctx, maxInt(0, x), maxInt(0, y))
}

func currentScreenSize() (int, int) {
	app := activeApp()
	if app == nil || app.ctx == nil {
		return 0, 0
	}

	screens, err := wailsruntime.ScreenGetAll(app.ctx)
	if err != nil || len(screens) == 0 {
		return wailsruntime.WindowGetSize(app.ctx)
	}

	screen := screens[0]
	for _, candidate := range screens {
		if candidate.IsCurrent {
			screen = candidate
			break
		}
		if candidate.IsPrimary {
			screen = candidate
		}
	}

	width, height := screen.Size.Width, screen.Size.Height
	if width == 0 {
		width = screen.Width
	}
	if height == 0 {
		height = screen.Height
	}
	if width == 0 || height == 0 {
		return wailsruntime.WindowGetSize(app.ctx)
	}
	return width, height
}

func clampInt(value, minimum, maximum int) int {
	if value < minimum {
		return minimum
	}
	if value > maximum {
		return maximum
	}
	return value
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
