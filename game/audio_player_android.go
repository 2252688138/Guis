// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

//go:build android

package game

// playerOpen Android 后端：原生出声需要 JNI/cgo 调 AAudio 或 AudioTrack，
// 与「纯 Go、无 cgo」的包定位冲突，这里默认返回 NopPlayer（静音运行、不报错）。
//
// 在 Termux 等有可执行环境的情况下，可用环境变量指定外部播放命令出声：
//
//	GUIS_AUDIO_CMD="ffplay -nodisp -autoexit -f s16le -ar %rate% -ac 2 -" ./yourgame
func playerOpen(rate int) Player {
	return OpenPlayerWithCommand(rate, "")
}
