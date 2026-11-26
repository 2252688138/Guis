// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game_test

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"testing"
)

// mustPNG 生成一张 2×2 PNG 字节（资源加载测试用）。
func mustPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	img.Set(1, 1, color.RGBA{B: 255, A: 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("生成 PNG 失败: %v", err)
	}
	return buf.Bytes()
}

// osStat 包装 os.Stat，供测试文件判断文件存在。
func osStat(path string) (os.FileInfo, error) { return os.Stat(path) }
