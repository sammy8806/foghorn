APP_NAME := foghorn
MACOS_APP := build/bin/$(APP_NAME).app

.PHONY: build build-macos

build:
ifeq ($(shell uname -s),Darwin)
	./scripts/build-macos-app.sh
else
	wails build
endif

build-macos:
	./scripts/build-macos-app.sh
