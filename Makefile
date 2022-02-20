SHELL := /bin/bash

# go requirements: fyne + fyne-cross

APP_VERSION = "v1-1"

GO := GO111MODULE=on CGO_ENABLED=1 go
GO_PATH = $(shell $(GO) env GOPATH)

.PHONY: all
all: help
.DEFAULT_GOAL:=help

.PHONY: help
help: ## show this help
	@fgrep -h "##" $(MAKEFILE_LIST) | fgrep -v fgrep | sed -e 's/\\$$//' | sed -e 's/##//'

.PHONY: build
build: bundle ## build project without development mode
	mkdir -p bin
	$(GO) build -ldflags "-s -w"

.PHONY: bundle
bundle: ## bundle resources
	cat assets/en.json | jq --sort-keys > en2.json
	cat assets/de.json | jq --sort-keys > de2.json
	mv en2.json assets/en.json
	mv de2.json assets/de.json
	$(GO_PATH)/bin/fyne bundle "assets/en.json" > bundled.go
	$(GO_PATH)/bin/fyne bundle -append "assets/de.json" >> bundled.go

.PHONY: package
package: clean build ## packages the application on the local platform
	$(GO_PATH)/bin/fyne package -appVersion '1.0' -release

.PHONY: release
release: clean ## release build for all platforms
	$(GO_PATH)/bin/fyne-cross windows -name archivebox-quick-add-windows-amd64-$(APP_VERSION).exe -arch "amd64"
	$(GO_PATH)/bin/fyne-cross windows -name archivebox-quick-add-windows-386-$(APP_VERSION).exe -arch "386"
	$(GO_PATH)/bin/fyne-cross freebsd -name archivebox-quick-add-freebsd-arm64-$(APP_VERSION) -arch "arm64"
	$(GO_PATH)/bin/fyne-cross freebsd -name archivebox-quick-add-freebsd-amd64-$(APP_VERSION) -arch "amd64"
	#$(GO_PATH)/bin/fyne-cross darwin -name archivebox-quick-add-$(APP_VERSION) -arch "*" -app-id "org.archivebox.quick-add"
	$(GO_PATH)/bin/fyne-cross linux -name archivebox-quick-add-linux-386-$(APP_VERSION) -arch "386"
	$(GO_PATH)/bin/fyne-cross linux -name archivebox-quick-add-linux-amd64-$(APP_VERSION) -arch "amd64"
	$(GO_PATH)/bin/fyne-cross linux -name archivebox-quick-add-linux-arm-$(APP_VERSION) -arch "arm"
	$(GO_PATH)/bin/fyne-cross linux -name archivebox-quick-add-linux-arm64-$(APP_VERSION) -arch "arm64"

	cp -f fyne-cross/dist/windows-amd64/archivebox-quick-add-windows-amd64-$(APP_VERSION).exe.zip release
	cp -f fyne-cross/dist/windows-386/archivebox-quick-add-windows-386-$(APP_VERSION).exe.zip release
	cp -f fyne-cross/dist/freebsd-arm64/archivebox-quick-add-freebsd-arm64-$(APP_VERSION).tar.xz release
	cp -f fyne-cross/dist/freebsd-amd64/archivebox-quick-add-freebsd-amd64-$(APP_VERSION).tar.xz release

	cp -f fyne-cross/dist/linux-386/archivebox-quick-add-linux-386-$(APP_VERSION).tar.xz release
	cp -f fyne-cross/dist/linux-amd64/archivebox-quick-add-linux-amd64-$(APP_VERSION).tar.xz release
	cp -f fyne-cross/dist/linux-arm/archivebox-quick-add-linux-arm-$(APP_VERSION).tar.xz release
	cp -f fyne-cross/dist/linux-arm64/archivebox-quick-add-linux-arm64-$(APP_VERSION).tar.xz release

.PHONY: clean
clean: ## clean up project
	-rm -rf bin
	-rm -rf fyne-cross
	-mkdir -p release