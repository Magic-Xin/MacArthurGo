## MacArthurGo
[![Go Report Card](https://goreportcard.com/badge/github.com/Magic-Xin/MacArthurGo)](https://goreportcard.com/report/github.com/Magic-Xin/MacArthurGo)
[![GitHub release (latest by date)](https://img.shields.io/github/v/release/Magic-Xin/MacArthurGo)](https://github.com/Magic-Xin/MacArthurGo/releases/latest)
[![Build Release](https://github.com/Magic-Xin/MacArthurGo/actions/workflows/release.yml/badge.svg)](https://github.com/Magic-Xin/MacArthurGo/actions/workflows/release.yml)
[![Build Dev](https://github.com/Magic-Xin/MacArthurGo/actions/workflows/dev.yml/badge.svg?branch=dev)](https://github.com/Magic-Xin/MacArthurGo/actions/workflows/dev.yml)
[![FOSSA Status](https://app.fossa.com/api/projects/git%2Bgithub.com%2FMagic-Xin%2FMacArthurGo.svg?type=shield)](https://app.fossa.com/projects/git%2Bgithub.com%2FMagic-Xin%2FMacArthurGo?ref=badge_shield)

**MacArthurGo** is a chatbot developed for the **OneBot V11** protocol using Golang. It provides plugin loading to support various functions.

If you have any comments or suggestions, you are welcome to discuss and provide feedback in the [issues](https://github.com/Magic-Xin/MacArthurGo/issues) section

**Highly recommend using [NapCatQQ](https://github.com/NapNeko/NapCatQQ) as the OneBot server**

## How to use

- Stable version: Download the compressed package and `config.json.default` for the corresponding system and architecture from the [release](https://github.com/Magic-Xin/MacArthurGo/releases), fill in the `config.json.default` and rename to `config.json` then run the program. 
- Dev version: Download compressed package from the newest [github actions](https://github.com/Magic-Xin/MacArthurGo/actions/workflows/dev.yml)

**Attention: Cannot guarantee the availability of the Dev version**

## Architecture

Startup is explicit and ordered: configuration is loaded and validated first,
then shared infrastructure is opened, plugins are registered, background
workers are started, and finally the OneBot WebSocket client begins running.
Plugins no longer depend on package `init()` side effects.

The WebSocket client keeps a bounded event queue, a bounded worker pool, and a
persistent outbound queue. A disconnected OneBot server is reconnected with
bounded exponential backoff. Plugin callbacks are serialized per plugin while
different plugins can still run concurrently, which keeps stateful plugins safe
without allowing unbounded goroutine growth. Shutdown cancellation is shared by
the WebSocket client, plugin schedulers, cache cleanup, database cleanup, and
statistics flushing.

## Development

CGO is required. On Windows with an MSYS2 UCRT64 toolchain, configure the
compiler in the same PowerShell session before building or testing:

```powershell
$env:CGO_ENABLED = "1"
$env:PATH = "C:\msys64\ucrt64\bin;$env:PATH"
$env:CC = "C:\msys64\ucrt64\bin\gcc.exe"
$env:CXX = "C:\msys64\ucrt64\bin\g++.exe"
go test ./...
go build ./...
```

All tests live in the top-level `test/` directory and exercise packages through their public behavior. Keep new test fixtures and helpers there rather than alongside production files.

## Plugins
- Essential Plugins
  - Help
  - Info
  - Database (sqlite3)
  - Update (dev version only)
- Chat AI
  - ChatGPT
  - Alibaba QianWen
  - Google Gemini
  - Github Models
- Music url parser
  - Netease Cloud Music
  - QQ Music
- BiliBili
  - Url parser (video, live)
  - AI Summarize
- Picture Search
  - SauceNao
  - Ascii2d
  - Google Lens (SerpApi)
- Poke
- Roll
- Repeat
- Corpus reply
- Daily waifu

### ascii2d setup

The ascii2d flow follows [cq-picsearcher-bot](https://github.com/Tsuk1ko/cq-picsearcher-bot): it fetches both the color and feature result pages and uses [CloudflareBypassForScraping](https://github.com/sarperavci/CloudflareBypassForScraping) to pass Cloudflare. Run the bypass service alongside MacArthurGo:

```powershell
docker run -d --name=cf-bypass -p 127.0.0.1:8000:8000 --restart unless-stopped ghcr.io/sarperavci/cloudflarebypassforscraping:latest
```

The default `plugins.picSearch.ascii2d.cloudflareBypassUrl` is `http://127.0.0.1:8000`. When this value is non-empty it takes priority and is also used to download protected thumbnails through mirror mode. Set `proxyUrl` only when the bypass service must use an HTTP or SOCKS proxy; that proxy must be reachable from the service container. Existing configurations can leave `cloudflareBypassUrl` empty and keep `flareSolverrUrl` as a compatibility fallback, although FlareSolverr is no longer the recommended ascii2d backend.

### Google Lens image-search setup

Google Lens results use the official [SerpApi Go client](https://github.com/serpapi/serpapi-golang) and its [Google Lens API](https://serpapi.com/google-lens-api). Set a SerpApi key in `plugins.picSearch.googleLens.apiKey`; the provider is disabled when the key is empty. Requests are fixed to `type=visual_matches` and `safe=off` without a language restriction. The reply contains one result's title, thumbnail, and link, selected by source priority (Pixiv, then Twitter/X, then other sites) and by the lowest result position within the same priority.

## TODO
- [ ] Add more plugins

## Thanks to the following projects or services
- [cq-picsearcher-bot](https://github.com/Tsuk1ko/cq-picsearcher-bot)
- [saucenao](https://saucenao.com/)
- [ascii2d](https://ascii2d.net)
- [go-cqhttp](https://github.com/Mrs4s/go-cqhttp)
- [Lagrange.Core](https://github.com/KonataDev/Lagrange.Core)
- [onebot-11](https://github.com/botuniverse/onebot-11)
- [OpenShamrock](https://github.com/whitechi73/OpenShamrock)
- [bilibili-API-collect](https://github.com/SocialSisterYi/bilibili-API-collect)
- [NapCatQQ](https://github.com/NapNeko/NapCatQQ)

## Special thanks
![JetBrains](https://resources.jetbrains.com/storage/products/company/brand/logos/jb_beam.svg)

**Special thanks to [JetBrains](https://jb.gg/OpenSourceSupport) for providing the open source license for this project.**

## License
[![FOSSA Status](https://app.fossa.com/api/projects/git%2Bgithub.com%2FMagic-Xin%2FMacArthurGo.svg?type=large)](https://app.fossa.com/projects/git%2Bgithub.com%2FMagic-Xin%2FMacArthurGo?ref=badge_large)
