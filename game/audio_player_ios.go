// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

//go:build ios

package game

// playerOpen iOS 后端：AVAudioEngine 需要 cgo / Objective-C 桥接，纯 Go 无法接入，
// 沙箱内也没有可执行的外部播放命令，因此返回 NopPlayer（静音、不报错）。
//
// 需要 iOS 出声请在应用层用 AVAudioSession + 原生播放器，或改用 WASM/Web 版本。
func playerOpen(rate int) Player { return NopPlayer{} }
