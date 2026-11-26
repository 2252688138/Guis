# bin/ —— 各端组件库演示产物（通过 GitHub Releases 分发）

`bin/` 目录**不入库二进制**：编译产物统一挂到 GitHub Releases，避免每次 clone 都拉
近百 MB 的不可压缩文件。这里只保留本说明。

## 下载

最新预发布：**v1.2.0-b150582**
<https://github.com/2252688138/Guis/releases/tag/v1.2.0-b150582>

| 端 | 产物 | 直接下载 |
| --- | --- | --- |
| Windows amd64 | 裸 exe | [guis-gallery-windows-amd64.exe](https://github.com/2252688138/Guis/releases/download/v1.2.0-b150582/guis-gallery-windows-amd64.exe) |
| Linux amd64 | 裸二进制 | [guis-gallery-linux-amd64](https://github.com/2252688138/Guis/releases/download/v1.2.0-b150582/guis-gallery-linux-amd64) |
| Linux arm64 | 裸二进制 | [guis-gallery-linux-arm64](https://github.com/2252688138/Guis/releases/download/v1.2.0-b150582/guis-gallery-linux-arm64) |
| macOS arm64 | **`.app` zip** | [guis-gallery-macos-arm64.app.zip](https://github.com/2252688138/Guis/releases/download/v1.2.0-b150582/guis-gallery-macos-arm64.app.zip) |
| macOS amd64 | **`.app` zip** | [guis-gallery-macos-amd64.app.zip](https://github.com/2252688138/Guis/releases/download/v1.2.0-b150582/guis-gallery-macos-amd64.app.zip) |
| Web (wasm) | wasm + 胶水 | [guis-gallery.wasm](https://github.com/2252688138/Guis/releases/download/v1.2.0-b150582/guis-gallery.wasm) · [wasm_exec.js](https://github.com/2252688138/Guis/releases/download/v1.2.0-b150582/wasm_exec.js) |
| 校验和 | — | [SHA256SUMS.txt](https://github.com/2252688138/Guis/releases/download/v1.2.0-b150582/SHA256SUMS.txt) |

## 这些是什么

`cmd/gallery` 是 guis 唯一的演示入口：一份 `main.go` 覆盖桌面 / 浏览器 / 安卓三端，
内容覆盖 `ui` 包里的全部组件——布局、图表、表格、树表、3D 视口、虚拟摇杆、
JSON 查看器、WebView、伪终端等。想快速看框架长什么样，下这个就行。

构建参数：`-trimpath -buildvcs=false -ldflags "-s -w -buildid="`，全部 `CGO_ENABLED=0`，
macOS / Linux 的窗口后端通过 purego 动态加载系统框架。

## macOS：`.app` bundle（双击即用）

macOS 发的是 **`.app` bundle 的 zip**，不是裸二进制。裸二进制在 Mac 上 Dock 名称与
图标不完整，Gatekeeper 还会直接拒绝运行。

```bash
unzip guis-gallery-macos-arm64.app.zip     # 得到 GUIS-arm64.app
open GUIS-arm64.app
xattr -dr com.apple.quarantine GUIS-arm64.app   # 首次被拦时放行
```

zip 里已经写好 Unix 权限位（`Contents/MacOS/GUIS` 为 `0755`），解压后不需要
`chmod +x`。目录结构：

```
GUIS-arm64.app/
└── Contents/
    ├── Info.plist              CFBundleIdentifier / 版本 / LSMinimumSystemVersion 11.0
    ├── PkgInfo                 APPL????
    ├── MacOS/GUIS              darwin 可执行文件（Mach-O，0755）
    └── Resources/icon.icns      由 ui/assets/icon_*.png 合成的 icns
```

未签名、未公证，所以首次运行会被拦一次；正式对外分发需在 Mac 上
`codesign` + `notarytool`（需要 Apple 开发者账号）。

## 自己重新生成

```powershell
./scripts/build-bin.ps1               # 六端产物到 bin/（macOS 自动出 .app.zip）
./scripts/build-bin.ps1 -Only darwin  # 只做两个 macOS .app.zip
./build-macos.ps1 -Arch arm64 -Zip    # 只做单个 .app + zip（到 dist/）
```

发布到 Releases（需要带 `repo` scope 的 token）：

```powershell
$env:GITHUB_TOKEN = 'ghp_xxx'
./scripts/release-bin.ps1                    # 构建 + 建 release + 上传资产
./scripts/release-bin.ps1 -Tag v1.2.1 -Stable  # 指定 tag 并作为正式版
```

## Web (wasm)

```bash
python -m http.server 8089      # wasm 不能用 file:// 打开
# 页面里用 wasm_exec.js 载入 guis-gallery.wasm
```

`wasm_exec.js` 需与编译用的 Go 版本一致（本仓库 `go.mod` 指定 Go 1.26）。
