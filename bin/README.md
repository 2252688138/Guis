# bin/ —— 各端组件库演示二进制（通过 GitHub Releases 分发）

`bin/` 目录**不入库二进制**：编译产物统一挂到 GitHub Releases，避免每次 clone 都拉
近百 MB 的不可压缩文件。这里只保留本说明。

## 下载

最新预发布：**v1.2.0-b150582**
<https://github.com/2252688138/Guis/releases/tag/v1.2.0-b150582>

| 端 | 直接下载 |
| --- | --- |
| Windows amd64 | [guis-gallery-windows-amd64.exe](https://github.com/2252688138/Guis/releases/download/v1.2.0-b150582/guis-gallery-windows-amd64.exe) |
| Linux amd64 | [guis-gallery-linux-amd64](https://github.com/2252688138/Guis/releases/download/v1.2.0-b150582/guis-gallery-linux-amd64) |
| Linux arm64 | [guis-gallery-linux-arm64](https://github.com/2252688138/Guis/releases/download/v1.2.0-b150582/guis-gallery-linux-arm64) |
| macOS arm64 | [guis-gallery-darwin-arm64](https://github.com/2252688138/Guis/releases/download/v1.2.0-b150582/guis-gallery-darwin-arm64) |
| macOS amd64 | [guis-gallery-darwin-amd64](https://github.com/2252688138/Guis/releases/download/v1.2.0-b150582/guis-gallery-darwin-amd64) |
| Web (wasm) | [guis-gallery.wasm](https://github.com/2252688138/Guis/releases/download/v1.2.0-b150582/guis-gallery.wasm) + [wasm_exec.js](https://github.com/2252688138/Guis/releases/download/v1.2.0-b150582/wasm_exec.js) |
| 校验和 | [SHA256SUMS.txt](https://github.com/2252688138/Guis/releases/download/v1.2.0-b150582/SHA256SUMS.txt) |

## 这些二进制是什么

`cmd/gallery` 是 guis 唯一的演示入口：一份 `main.go` 覆盖桌面 / 浏览器 / 安卓三端，
内容覆盖 `ui` 包里的全部组件——布局、图表、表格、树表、3D 视口、虚拟摇杆、
JSON 查看器、WebView、伪终端等。想快速看框架长什么样，下这个就行。

构建参数：`-trimpath -buildvcs=false -ldflags "-s -w -buildid="`，全部 `CGO_ENABLED=0`，
macOS / Linux 的窗口后端通过 purego 动态加载系统框架。

## 自己重新生成

```powershell
./scripts/build-bin.ps1              # 构建 6 个端到 bin/（本地）
./scripts/build-bin.ps1 -Only darwin # 只构建 macOS 两个架构
```

发布到 Releases（需要带 `repo` scope 的 token）：

```powershell
$env:GITHUB_TOKEN = 'ghp_xxx'
./scripts/release-bin.ps1            # 构建 + 建 release + 上传资产
./scripts/release-bin.ps1 -Tag v1.2.1 -Stable   # 指定 tag 并作为正式版
```

## 各端注意事项

- **macOS**：裸二进制没有 `.app` bundle，Dock 图标与名称不完整，首次运行还会被
  Gatekeeper 拦（`xattr -dr com.apple.quarantine guis-gallery-darwin-arm64` 放行）。
  要可分发的 bundle 用仓库根目录的 `build-macos.ps1` 生成。
- **Web (wasm)**：必须用静态服务器打开（wasm 不能 `file://`），`wasm_exec.js` 需与
  编译用的 Go 版本一致（本仓库 `go.mod` 指定 Go 1.26）。
- **Linux**：无 `libx11` 之类依赖（purego 动态加载），但运行环境需要图形会话；
  无显示服务器时请传 `-headless` 之类参数按需处理。
