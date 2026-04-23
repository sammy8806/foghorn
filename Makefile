APP_NAME := foghorn
VERSION  := $(shell scripts/version.sh)

.PHONY: build build-macos dmg appimage version clean

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

appimage:
	FOGHORN_VERSION=$(VERSION) ./scripts/build-appimage.sh

version:
	@echo $(VERSION)

clean:
	rm -rf build/bin
