APP_NAME := foghorn
VERSION  := $(shell scripts/version.sh)
MACOS_APP := build/bin/$(APP_NAME).app
MACOS_DMG := build/dist/$(APP_NAME).dmg

.PHONY: build build-macos dmg dmg-existing appimage version clean

build:
ifeq ($(shell uname -s),Darwin)
	./scripts/build-macos-app.sh
else
	wails build -tags "linux_tray" -ldflags "-X main.version=$(VERSION)"
endif

build-macos:
	./scripts/build-macos-app.sh

dmg:
	FOGHORN_VERSION=$(VERSION) ./scripts/build-dmg.sh

dmg-existing:
	./scripts/build-macos-dmg.sh --skip-build --app-path "$(MACOS_APP)" --dmg-path "$(MACOS_DMG)"

appimage:
	FOGHORN_VERSION=$(VERSION) ./scripts/build-appimage.sh

version:
	@echo $(VERSION)

clean:
	rm -rf build/bin
