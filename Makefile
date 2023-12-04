NAME=MacArthurGo
BINDIR=bin
GOBUILD=go build
LDFLAGS=-s -w

DARWIN_PLATFORM_LIST = \
	darwin-amd64 \
    darwin-arm64

LINUX_PLATFORM_LIST = \
	linux-386 \
	linux-amd64 \
	linux-arm64

WINDOWS_PLATFORM_LIST = \
	windows-386 \
    windows-amd64 \
    windows-arm64

all: linux-amd64 # Most used

darwin-amd64:
	xgo --targets=darwin/amd64 -ldflags="${LDFLAGS}" --out $(BINDIR)/MacArthurGo ./

darwin-arm64:
	xgo --targets=darwin/arm64 -ldflags="${LDFLAGS}" --out $(BINDIR)/MacArthurGo ./

linux-386:
	xgo --targets=linux/386 -ldflags="${LDFLAGS}" --out $(BINDIR)/MacArthurGo ./

linux-amd64:
	xgo --targets=linux/amd64 -ldflags="${LDFLAGS}" --out $(BINDIR)/MacArthurGo ./

linux-arm64:
	xgo --targets=linux/arm64 -ldflags="${LDFLAGS}" --out $(BINDIR)/MacArthurGo ./

windows-386:
	xgo --targets=windows/386 -ldflags="${LDFLAGS}" --out $(BINDIR)/MacArthurGo ./

windows-amd64:
	xgo --targets=windows/amd64 -ldflags="${LDFLAGS}" --out $(BINDIR)/MacArthurGo ./

windows-arm64:
	xgo --targets=windows/arm64 -ldflags="${LDFLAGS}" --out $(BINDIR)/MacArthurGo ./

darwin_releases=$(addsuffix .tar, $(DARWIN_PLATFORM_LIST))

$(darwin_releases): %.tar : %
	chmod +x $(BINDIR)/MacArthurGo-*
	tar -zcvf $(BINDIR)/$(NAME)-$(basename $@).tar.gz -C $(BINDIR) MacArthurGo-*
	rm -rf $(BINDIR)/MacArthurGo-*

linux_releases=$(addsuffix .tar, $(LINUX_PLATFORM_LIST))

$(linux_releases): %.tar : %
	chmod +x $(BINDIR)/MacArthurGo
	-${upx} --lzma --best $(BINDIR)/MacArthurGo
	tar -zcvf $(BINDIR)/$(NAME)-$(basename $@).tar.gz -C $(BINDIR) MacArthurGo-*
	rm -rf $(BINDIR)/MacArthurGo-*

windows_releases=$(addsuffix .zip, $(WINDOWS_PLATFORM_LIST))

$(windows_releases): %.zip : %
	-${upx} --lzma --best $(BINDIR)/MacArthurGo-*.exe
	zip -v9 $(BINDIR)/$(NAME)-$(basename $@).zip $(BINDIR)/MacArthurGo-*.exe
	rm -rf $(BINDIR)/MacArthurGo-*.exe

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