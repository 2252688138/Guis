// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

//go:build !windows && !linux && !darwin && !android && !ios && !(js && wasm)

package game

// playerOpen 该平台暂无原生音频设备后端（freebsd / openbsd / netbsd / solaris 等），
// 返回空播放器：不报错、不阻塞，只是没有声音。
//
// 如果机器上有 aplay / ffplay / sox，可用环境变量指定命令出声：
//
//	GUIS_AUDIO_CMD="ffplay -nodisp -autoexit -f s16le -ar %rate% -ac 2 -" ./yourgame
func playerOpen(rate int) Player { return OpenPlayerWithCommand(rate, "") }
