// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

import "testing"

// TestPostKeyCoalesce 同 key 投递只执行最后一次（中间态合并掉）。
func TestPostKeyCoalesce(t *testing.T) {
	a := &App{}
	var got []string
	a.PostKey("panel", func() { got = append(got, "old") })
	a.PostKey("panel", func() { got = append(got, "new") })
	a.PostKey("other", func() { got = append(got, "other") })
	if len(a.pending) != 2 {
		t.Fatalf("同 key 应合并为 1 个任务，实际队列 %d", len(a.pending))
	}
	a.runPending()
	if len(got) != 2 {
		t.Fatalf("应执行 2 个任务，实际 %v", got)
	}
	seen := map[string]bool{}
	for _, s := range got {
		seen[s] = true
	}
	if !seen["new"] || !seen["other"] || seen["old"] {
		t.Fatalf("同 key 应保留最后一次投递，实际 %v", got)
	}
}

// TestRunPendingBatch 单帧分批执行：剩余任务留到下一帧，不把一帧拉长。
func TestRunPendingBatch(t *testing.T) {
	old := pendingPerFrame
	pendingPerFrame = 2
	defer func() { pendingPerFrame = old }()

	a := &App{}
	count := 0
	for i := 0; i < 5; i++ {
		a.Post(func() { count++ })
	}
	a.runPending()
	if count != 2 {
		t.Fatalf("首帧应只执行 2 个，实际 %d", count)
	}
	a.runPending()
	if count != 4 {
		t.Fatalf("第二帧应执行到 4，实际 %d", count)
	}
	a.runPending()
	if count != 5 {
		t.Fatalf("第三帧应清空队列，实际 %d", count)
	}
	if len(a.pending) != 0 {
		t.Fatalf("队列应清空，实际 %d", len(a.pending))
	}
}

// TestPostHardLimitDrops 超过硬上限的投递被丢弃，队列不无界增长。
func TestPostHardLimitDrops(t *testing.T) {
	oldLimit, oldSoft := pendingHardLimit, pendingSoftLimit
	pendingHardLimit = 2
	pendingSoftLimit = 1 << 30 // 关掉软告警噪声
	defer func() { pendingHardLimit, pendingSoftLimit = oldLimit, oldSoft }()

	a := &App{}
	for i := 0; i < 5; i++ {
		a.Post(func() {})
	}
	if len(a.pending) != 2 {
		t.Fatalf("队列应停在硬上限 2，实际 %d", len(a.pending))
	}
}

// TestRunPendingKeepsKeyIndex key 索引在分批平移后仍然指向正确任务。
func TestRunPendingKeepsKeyIndex(t *testing.T) {
	old := pendingPerFrame
	pendingPerFrame = 1
	defer func() { pendingPerFrame = old }()

	a := &App{}
	var got []string
	a.Post(func() { got = append(got, "a") })
	a.PostKey("later", func() { got = append(got, "first") })
	a.runPending() // 只跑 "a"，"later" 平移成下标 0
	a.PostKey("later", func() { got = append(got, "second") })
	a.runPending()
	if len(got) != 2 || got[0] != "a" || got[1] != "second" {
		t.Fatalf("同 key 覆盖应命中平移后的下标，实际 %v", got)
	}
}
