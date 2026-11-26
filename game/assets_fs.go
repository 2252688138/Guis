// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game

import "os"

// osReadFile 包一层 os.ReadFile（便于单测替换，不直接依赖文件系统细节）。
func osReadFile(path string) ([]byte, error) { return os.ReadFile(path) }
