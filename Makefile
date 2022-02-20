SHELL := /bin/bash

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

package: ## packages the application on the local platform
	$(GO_PATH)/bin/fyne package -appVersion '1.0' -release

.PHONY: clean
clean: ## clean up project
	-rm -rf bin
	-mkdir -p bin