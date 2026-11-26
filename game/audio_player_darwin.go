// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

//go:build darwin && !ios

package game

// playerOpen macOS 后端：CoreAudio 的 AudioQueue 需要 C 回调，纯 Go + purego
// 无法注册 Go 函数作为 C 回调，因此这里走「外部命令管道」路线：
//
//	sox -q -t raw -r RATE -c 2 -e signed-integer -b 16 - -d
//	ffplay -nodisp -autoexit -f s16le -ar RATE -ac 2 -
//
// 两者任一存在即可出声（sox / ffmpeg 可用 brew 安装）；都不存在时返回 NopPlayer。
// 延迟约 50~200ms，适合音乐与节奏类音效；极低延迟场景请用 WASM/Web 版本。
func playerOpen(rate int) Player {
	return OpenPlayerWithCommand(rate, "")
}
