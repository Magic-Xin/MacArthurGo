NAME=MacArthurGo
BINDIR=bin
OUTDIR=out
LDFLAGS=-s -w -checklinkname=0 -X MacArthurGo/base.Version=$(shell git describe --tags --always --dirty)  -X MacArthurGo/base.Branch=Release -X MacArthurGo/base.BuildTime=$(shell date +'%Y-%m-%dT%H:%M:%SZ' -u)

DARWIN_PLATFORM_LIST = \
	darwin-amd64 \
    darwin-arm64

LINUX_PLATFORM_LIST = \
	linux-386 \
	linux-amd64 \
	linux-arm64 \
	android-arm64

WINDOWS_PLATFORM_LIST = \
	windows-386 \
    windows-amd64

darwin-amd64:
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags "${LDFLAGS}" -o ${BINDIR}/MacArthurGo-darwin-amd64 ./

darwin-arm64:
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -ldflags "${LDFLAGS}" -o ${BINDIR}/MacArthurGo-darwin-arm64 ./

linux-386:
	CGO_ENABLED=0 GOOS=linux GOARCH=386 go build -ldflags "${LDFLAGS}" -o ${BINDIR}/MacArthurGo-linux-386 ./

linux-amd64:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "${LDFLAGS}" -o ${BINDIR}/MacArthurGo-linux-amd64 ./

linux-arm64:
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags "${LDFLAGS}" -o ${BINDIR}/MacArthurGo-linux-arm64 ./

android-arm64:
	CGO_ENABLED=0 GOOS=android GOARCH=arm64 go build -ldflags "${LDFLAGS}" -o ${BINDIR}/MacArthurGo-android-arm64 ./

windows-386:
	CGO_ENABLED=0 GOOS=windows GOARCH=386 go build -ldflags "${LDFLAGS}" -o ${BINDIR}/MacArthurGo-windows-386 ./

windows-amd64:
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags "${LDFLAGS}" -o ${BINDIR}/MacArthurGo-windows-amd64 ./

darwin_releases=$(addsuffix .tar, $(DARWIN_PLATFORM_LIST))

$(darwin_releases): %.tar : %
	chmod +x $(BINDIR)/MacArthurGo-*
	cd $(BINDIR) && tar -zcvf ../$(OUTDIR)/$(NAME)-$(basename $@).tar.gz MacArthurGo-*
	rm -rf $(BINDIR)/MacArthurGo-*

linux_releases=$(addsuffix .tar, $(LINUX_PLATFORM_LIST))

$(linux_releases): %.tar : %
	chmod +x $(BINDIR)/MacArthurGo-*
	-${upx} --lzma --best $(BINDIR)/MacArthurGo-*
	cd $(BINDIR) && tar -zcvf ../$(OUTDIR)/$(NAME)-$(basename $@).tar.gz MacArthurGo-*
	rm -rf $(BINDIR)/MacArthurGo-*

windows_releases=$(addsuffix .zip, $(WINDOWS_PLATFORM_LIST))

$(windows_releases): %.zip : %
	mv $(BINDIR)/MacArthurGo-* $(BINDIR)/MacArthurGo-$(basename $@).exe
	-${upx} --lzma --best $(BINDIR)/MacArthurGo-*
	cd $(BINDIR) && zip -v9 ../$(OUTDIR)/$(NAME)-$(basename $@).zip MacArthurGo-*
	rm -rf $(BINDIR)/MacArthurGo-*

all-arch: $(PLATFORM_LIST)

releases: $(darwin_releases) $(linux_releases) $(windows_releases)

lint:
	golangci-lint run ./...

clean:
	rm $(BINDIR)/*

CLANG ?= clang-14
CFLAGS := -O2 -g -Wall -Werror $(CFLAGS)

ebpf: export BPF_CLANG := $(CLANG)
ebpf: export BPF_CFLAGS := $(CFLAGS)
ebpf:
	cd component/ebpf/ && go generate ./...