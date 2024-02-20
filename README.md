## Introduction
[![Build Release](https://github.com/Magic-Xin/MacArthurGo/actions/workflows/release.yml/badge.svg)](https://github.com/Magic-Xin/MacArthurGo/actions/workflows/release.yml)
[![Build Dev](https://github.com/Magic-Xin/MacArthurGo/actions/workflows/dev.yml/badge.svg?branch=dev)](https://github.com/Magic-Xin/MacArthurGo/actions/workflows/dev.yml)

MacArthurGo is a chatbot developed for the **OneBot V11** protocol using Golang. It provides plugin loading to support various functions.

If you have any comments or suggestions, you are welcome to discuss and provide feedback in the [issues](https://github.com/Magic-Xin/MacArthurGo/issues) section

**Highly recommend using [OpenShamrock](https://github.com/whitechi73/OpenShamrock) as the OneBot server**

## How to use

- Stable version: Download the compressed package and `config.json.default` for the corresponding system and architecture from the [release](https://github.com/Magic-Xin/MacArthurGo/releases),fill in the `config.json.default` and rename to `config.json` then run the program. 
- Dev version: Download compressed package from the newest [github actions](https://github.com/Magic-Xin/MacArthurGo/actions/workflows/dev.yml)

**Attention: Cannot guarantee the availability of the Dev version**

## Plugins
- Essential Plugins
  - Help
  - Info
  - Database (sqlite3)
  - Update (dev version only)
- Chat AI
  - ChatGPT
  - Alibaba QianWen
  - Google Gemini Pro (with picture search)
  - New Bing Chat
- Music url parser
  - Netease Cloud Music
  - QQ Music
- BiliBili
  - Url parser (video, live)
  - AI Summarize
- Picture Search
  - SauceNao
  - Ascii2d
- Poke
- Roll
- Repeat
- Corpus reply

## TODO
- [ ] Add more plugins

## Thanks to the following projects or services
- [cq-picsearcher-bot](https://github.com/Tsuk1ko/cq-picsearcher-bot)
- [saucenao](https://saucenao.com/)
- [ascii2d](https://ascii2d.net)
- [go-cqhttp](https://github.com/Mrs4s/go-cqhttp)
- [OpenShamrock](https://github.com/whitechi73/OpenShamrock)
- [bilibili-API-collect](https://github.com/SocialSisterYi/bilibili-API-collect)
- [go-proxy-bingai](https://github.com/Harry-zklcdc/go-proxy-bingai)

## Special thanks
![JetBrains](https://resources.jetbrains.com/storage/products/company/brand/logos/jb_beam.svg)

**Special thanks to [JetBrains](https://jb.gg/OpenSourceSupport) for providing the open source license for this project.**