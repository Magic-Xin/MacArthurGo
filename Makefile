NAME=MacArthurGo
BINDIR=build
GOBUILD=go build

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
	CGO_ENABLED=1 GOARCH=amd64 GOOS=darwin $(GOBUILD) -o $(BINDIR)/MacArthurGo

darwin-arm64:
	CGO_ENABLED=1 GOARCH=arm64 GOOS=darwin $(GOBUILD) -o $(BINDIR)/MacArthurGo

linux-386:
	CGO_ENABLED=1 GOARCH=386 GOOS=linux $(GOBUILD) -o $(BINDIR)/MacArthurGo

linux-amd64:
	CGO_ENABLED=1 GOARCH=amd64 GOOS=linux $(GOBUILD) -o $(BINDIR)/MacArthurGo

linux-arm64:
	CGO_ENABLED=1 GOARCH=arm64 GOOS=linux $(GOBUILD) -o $(BINDIR)/MacArthurGo

windows-386:
	CGO_ENABLED=1 GOARCH=386 GOOS=windows $(GOBUILD) -o $(BINDIR)/MacArthurGo.exe

windows-amd64:
	CGO_ENABLED=1 GOARCH=amd64 GOOS=windows $(GOBUILD) -o $(BINDIR)/MacArthurGo.exe

windows-arm64:
	CGO_ENABLED=1 GOARCH=arm64 GOOS=windows $(GOBUILD) -o $(BINDIR)/MacArthurGo.exe

darwin_releases=$(addsuffix .tar, $(DARWIN_PLATFORM_LIST))

$(darwin_releases): %.tar : %
	chmod +x $(BINDIR)/MacArthurGo
	tar -zcvf $(BINDIR)/$(NAME)-$(basename $@).tar.gz -C $(BINDIR) MacArthurGo
	rm -rf $(BINDIR)/MacArthurGo

linux_releases=$(addsuffix .tar, $(LINUX_PLATFORM_LIST))

$(linux_releases): %.tar : %
	chmod +x $(BINDIR)/MacArthurGo
	-${upx} --lzma --best $(BINDIR)/MacArthurGo
	tar -zcvf $(BINDIR)/$(NAME)-$(basename $@).tar.gz -C $(BINDIR) MacArthurGo
	rm -rf $(BINDIR)/MacArthurGo

windows_releases=$(addsuffix .zip, $(WINDOWS_PLATFORM_LIST))

$(windows_releases): %.zip : %
	-${upx} --lzma --best $(BINDIR)/MacArthurGo.exe
	zip -v9 $(BINDIR)/$(NAME)-$(basename $@).zip $(BINDIR)/MacArthurGo.exe
	rm -rf $(BINDIR)/MacArthurGo.exe

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