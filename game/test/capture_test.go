// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game_test

import (
	"image/gif"
	"os"
	"path/filepath"
	"testing"
	"time"

	"guis/game"
)

func TestSnapshotCopy(t *testing.T) {
	r := game.NewRenderer(32, 24)
	img := r.Image()
	img.Pix[0], img.Pix[1], img.Pix[2], img.Pix[3] = 10, 20, 30, 255
	snap := game.Snapshot(r)
	if snap == nil || snap.Rect.Dx() != 32 || snap.Rect.Dy() != 24 {
		t.Fatalf("快照尺寸错误: %v", snap)
	}
	// 改动渲染器缓冲不应影响快照
	img.Pix[0] = 200
	if snap.Pix[0] != 10 {
		t.Fatal("快照应独立于帧缓冲")
	}
	var nilR *game.Renderer
	if game.Snapshot(nilR) != nil {
		t.Fatal("nil 渲染器应返回 nil")
	}
}

func TestSnapshotPNGFile(t *testing.T) {
	r := game.NewRenderer(16, 16)
	path := filepath.Join(t.TempDir(), "shots", "a.png")
	if err := game.SnapshotPNG(r, path); err != nil {
		t.Fatalf("保存 PNG 失败: %v", err)
	}
	st, err := os.Stat(path)
	if err != nil || st.Size() == 0 {
		t.Fatalf("PNG 文件异常: %v", err)
	}
}

func TestGIFRecorder(t *testing.T) {
	r := game.NewRenderer(8, 8)
	g := game.NewGIFRecorder(3, 5)
	for i := 0; i < 3; i++ {
		r.Image().Pix[0] = uint8(i * 40)
		g.Add(game.Snapshot(r))
	}
	if g.Len() != 3 {
		t.Fatalf("帧数 = %d", g.Len())
	}
	path := filepath.Join(t.TempDir(), "anim.gif")
	if err := g.Save(path); err != nil {
		t.Fatalf("保存 GIF 失败: %v", err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("打开 GIF 失败: %v", err)
	}
	defer f.Close()
	anim, err := gif.DecodeAll(f)
	if err != nil {
		t.Fatalf("解码 GIF 失败: %v", err)
	}
	if len(anim.Image) != 3 {
		t.Fatalf("GIF 帧数 = %d，期望 3", len(anim.Image))
	}
	// 超过上限后只保留最新帧
	for i := 0; i < 5; i++ {
		g.Add(game.Snapshot(r))
	}
	if g.Len() != 3 {
		t.Fatalf("环形覆盖后帧数 = %d，期望 3", g.Len())
	}
}

func TestFrameLimiter(t *testing.T) {
	f := game.NewFrameLimiter(60)
	if f.Wait() != 0 {
		t.Fatal("首帧不应等待")
	}
	start := time.Now()
	f.Wait()
	el := time.Since(start)
	if el < 8*time.Millisecond {
		t.Fatalf("等待过短: %v", el)
	}
	if f.FPS() < 40 || f.FPS() > 90 {
		t.Fatalf("实测 FPS = %.1f，期望约 60", f.FPS())
	}
	f.Reset()
	if f.FPS() != 0 {
		t.Fatal("Reset 后 FPS 应为 0")
	}
	// 不限帧
	if game.NewFrameLimiter(0).Wait() != 0 {
		t.Fatal("不限帧时不应等待")
	}
}
