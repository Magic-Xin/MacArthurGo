NAME=MacArthurGo
BINDIR=bin
OUTDIR=out
LDFLAGS=-s -w -X MacArthurGo/base.Version=$(shell git describe --tags --always --dirty)  -X MacArthurGo/base.Branch=Release -X MacArthurGo/base.BuildTime=$(shell date +'%Y-%m-%dT%H:%M:%SZ' -u)
CC=${ANDROID_NDK_HOME}/toolchains/llvm/prebuilt/linux-x86_64/bin/aarch64-linux-android33-clang

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
	xgo --targets=darwin-10.14/amd64 -ldflags="${LDFLAGS}" --out $(BINDIR)/MacArthurGo ./

darwin-arm64:
	xgo --targets=darwin-10.14/arm64 -ldflags="${LDFLAGS}" --out $(BINDIR)/MacArthurGo ./

linux-386:
	xgo --targets=linux/386 -ldflags="${LDFLAGS}" --out $(BINDIR)/MacArthurGo ./

linux-amd64:
	xgo --targets=linux/amd64 -ldflags="${LDFLAGS}" --out $(BINDIR)/MacArthurGo ./

linux-arm64:
	xgo --targets=linux/arm64 -ldflags="${LDFLAGS}" --out $(BINDIR)/MacArthurGo ./

android-arm64:
	CC=${CC} CGO_ENABLED=1 GOOS=android GOARCH=arm64 go build -ldflags "${LDFLAGS}" -o ${BINDIR}/MacArthurGo-android-arm64 ./

windows-386:
	xgo --targets=windows-6.0/386 -ldflags="${LDFLAGS}" --out $(BINDIR)/MacArthurGo ./

windows-amd64:
	xgo --targets=windows-6.0/amd64 -ldflags="${LDFLAGS}" --out $(BINDIR)/MacArthurGo ./

darwin_releases=$(addsuffix .tar, $(DARWIN_PLATFORM_LIST))

$(darwin_releases): %.tar : %
	mv $(BINDIR)/MacArthurGo-* $(BINDIR)/MacArthurGo-$(basename $@)
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