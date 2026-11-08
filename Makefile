SHELL := /bin/bash

# go requirements: fyne + fyne-cross

APP_VERSION_STR = "v1-14"
APP_VERSION_DOT := "1.14"

GO := GO111MODULE=on CGO_ENABLED=1 go
GO_PATH = $(shell $(GO) env GOPATH)

.PHONY: all
all: help
.DEFAULT_GOAL:=help

.PHONY: help
help: ## show this help
	@grep -F -h "##" $(MAKEFILE_LIST) | grep -F -v fgrep | sed -e 's/\\$$//' | sed -e 's/##//'

.PHONY: setup
setup: ## setup project tools
	$(GO) install fyne.io/tools/cmd/fyne@latest

.PHONY: build
build: bundle ## build project without development mode
	$(GO) build -ldflags "-s -w"

.PHONY: bundle
bundle: ## bundle resources
	cat assets/en.json | jq --sort-keys > en2.json
	cat assets/de.json | jq --sort-keys > de2.json
	mv en2.json assets/en.json
	mv de2.json assets/de.json
	$(GO_PATH)/bin/fyne bundle "assets/en.json" > bundled.go
	$(GO_PATH)/bin/fyne bundle -append "assets/de.json" >> bundled.go
	$(GO_PATH)/bin/fyne bundle -append "Icon.png" >> bundled.go

.PHONY: package
package: clean build ## packages the application on the local platform
	$(GO_PATH)/bin/fyne package --app-version $(APP_VERSION_DOT) --release

.PHONY: release
release: clean ## release build for all platforms
	$(GO_PATH)/bin/fyne-cross windows -name archivebox-quick-add-windows-amd64-$(APP_VERSION_STR).exe -arch "amd64" --app-id org.archivebox.go-quick-add --app-version $(APP_VERSION_DOT)
	$(GO_PATH)/bin/fyne-cross windows -name archivebox-quick-add-windows-386-$(APP_VERSION_STR).exe -arch "386" --app-id org.archivebox.go-quick-add --app-version $(APP_VERSION_DOT)

	# -tags x11: the default Wayland backend does not link against the FreeBSD sysroot
	$(GO_PATH)/bin/fyne-cross freebsd -name archivebox-quick-add-freebsd-arm64-$(APP_VERSION_STR) -arch "arm64" --app-id org.archivebox.go-quick-add --app-version $(APP_VERSION_DOT) -tags x11
	$(GO_PATH)/bin/fyne-cross freebsd -name archivebox-quick-add-freebsd-amd64-$(APP_VERSION_STR) -arch "amd64" --app-id org.archivebox.go-quick-add --app-version $(APP_VERSION_DOT) -tags x11
	# macOS builds must run on a macOS host, see the release-darwin target below
	$(GO_PATH)/bin/fyne-cross linux -name archivebox-quick-add-linux-386-$(APP_VERSION_STR) -arch "386" --app-id org.archivebox.go-quick-add --app-version $(APP_VERSION_DOT)
	$(GO_PATH)/bin/fyne-cross linux -name archivebox-quick-add-linux-amd64-$(APP_VERSION_STR) -arch "amd64" --app-version $(APP_VERSION_DOT)
	$(GO_PATH)/bin/fyne-cross linux -name archivebox-quick-add-linux-arm-$(APP_VERSION_STR) -arch "arm" --app-version $(APP_VERSION_DOT)
	$(GO_PATH)/bin/fyne-cross linux -name archivebox-quick-add-linux-arm64-$(APP_VERSION_STR) -arch "arm64" --app-version $(APP_VERSION_DOT)

	cp -f fyne-cross/dist/windows-amd64/archivebox-quick-add-windows-amd64-$(APP_VERSION_STR).exe.zip release/archivebox-quick-add-windows-amd64-$(APP_VERSION_STR).zip
	cp -f fyne-cross/dist/windows-386/archivebox-quick-add-windows-386-$(APP_VERSION_STR).exe.zip release/archivebox-quick-add-windows-386-$(APP_VERSION_STR).zip
	cp -f fyne-cross/dist/freebsd-arm64/archivebox-quick-add-freebsd-arm64-$(APP_VERSION_STR).tar.xz release
	cp -f fyne-cross/dist/freebsd-amd64/archivebox-quick-add-freebsd-amd64-$(APP_VERSION_STR).tar.xz release

	cp -f fyne-cross/dist/linux-386/archivebox-quick-add-linux-386-$(APP_VERSION_STR).tar.xz release
	cp -f fyne-cross/dist/linux-amd64/archivebox-quick-add-linux-amd64-$(APP_VERSION_STR).tar.xz release
	cp -f fyne-cross/dist/linux-arm/archivebox-quick-add-linux-arm-$(APP_VERSION_STR).tar.xz release
	cp -f fyne-cross/dist/linux-arm64/archivebox-quick-add-linux-arm64-$(APP_VERSION_STR).tar.xz release

.PHONY: release-darwin
release-darwin: ## build macOS .app bundles (must run on a macOS host with fyne installed)
	@mkdir -p release
	GOARCH=amd64 $(GO_PATH)/bin/fyne package -os darwin \
		-name archivebox-quick-add-darwin-amd64-$(APP_VERSION_STR) \
		--app-id org.archivebox.go-quick-add \
		--app-version $(APP_VERSION_DOT) \
		--release
	GOARCH=arm64 $(GO_PATH)/bin/fyne package -os darwin \
		-name archivebox-quick-add-darwin-arm64-$(APP_VERSION_STR) \
		--app-id org.archivebox.go-quick-add \
		--app-version $(APP_VERSION_DOT) \
		--release
	ditto -c -k --sequesterRsrc --keepParent archivebox-quick-add-darwin-amd64-$(APP_VERSION_STR).app release/archivebox-quick-add-darwin-amd64-$(APP_VERSION_STR).zip
	ditto -c -k --sequesterRsrc --keepParent archivebox-quick-add-darwin-arm64-$(APP_VERSION_STR).app release/archivebox-quick-add-darwin-arm64-$(APP_VERSION_STR).zip

.PHONY: version
version: ## populate the current version defined in this make file
	sed -r -i 's/AppVersion:      "([0-9]+.[0-9]+)"/AppVersion:      "'$(APP_VERSION_DOT)'"/g' main.go
	sed -r -i 's/Version: ([0-9]+.[0-9]+)/Version: '$(APP_VERSION_DOT)'/g' README.md

.PHONY: clean
clean: ## clean up project
	-rm -rf bin
	-rm -rf fyne-cross
	-mkdir -p release
