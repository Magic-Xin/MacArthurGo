NAME := MacArthurGo
BINDIR := bin
OUTDIR := out

GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)
VERSION ?= $(shell git describe --tags --always --dirty)
BRANCH ?= Release
BUILD_TIME ?= $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')

PLATFORM := $(GOOS)-$(GOARCH)
EXE := $(if $(filter windows,$(GOOS)),.exe,)
BINARY := $(BINDIR)/$(NAME)-$(PLATFORM)$(EXE)
STAGE := $(OUTDIR)/.stage-$(PLATFORM)
ARCHIVE := $(OUTDIR)/$(NAME)-$(PLATFORM)$(if $(filter windows,$(GOOS)),.zip,.tar.gz)
LDFLAGS := -s -w -X MacArthurGo/base.Version=$(VERSION) -X MacArthurGo/base.Branch=$(BRANCH) -X MacArthurGo/base.BuildTime=$(BUILD_TIME)

.PHONY: build package releases clean lint

build:
	mkdir -p $(BINDIR)
	CGO_ENABLED=1 GOOS=$(GOOS) GOARCH=$(GOARCH) go build -ldflags "$(LDFLAGS)" -o $(BINARY) ./

package: build
	rm -rf $(STAGE)
	mkdir -p $(STAGE)
	cp $(BINARY) config.json.default $(STAGE)/
	cp -R jieba_dict $(STAGE)/
ifeq ($(GOOS),windows)
	powershell.exe -NoProfile -Command "Compress-Archive -Path '$(STAGE)/*' -DestinationPath '$(ARCHIVE)' -Force"
else
	tar -C $(STAGE) -czf $(ARCHIVE) .
endif
	rm -rf $(STAGE)

releases: package

lint:
	golangci-lint run ./...

clean:
	rm -rf $(BINDIR) $(OUTDIR)
