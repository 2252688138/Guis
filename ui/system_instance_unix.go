//go:build !windows && !js

// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

// 单实例锁（Linux / macOS / BSD）。
//
// 旧实现在非 Windows 端恒返回 true —— 双开会各开一个窗口、两个托盘图标，
// 配置还可能互相覆盖。这里用 flock（BSD 锁，进程退出自动释放，不会留死锁）
// 实现真正的互斥：
//
//	if !ui.SingleInstance("com.example.app") {
//	    ui.NotifyFirstInstance("com.example.app", os.Args[1:])
//	    return
//	}
//	go ui.WatchSecondInstance("com.example.app", func(args []string) { show(args) })
//
// 参数传递走「锁文件旁边的 .args 文件」：flock 本身没有消息通道，
// 首个实例轮询读取后清空（最多延迟 500ms，可接受）。

var (
	instanceLock     *os.File
	instanceLockPath string
	instanceOnce     sync.Once
)

// SingleInstance 申请单实例锁；返回 true 表示本进程是首个实例。
//
// 锁文件放在临时目录（/tmp 或 $TMPDIR），不污染用户配置目录；
// 打不开锁文件时按「首个实例」处理（宁可双开也不要启动失败）。
func SingleInstance(key string) bool {
	if key == "" {
		key = "guis"
	}
	name := "guis-" + sanitizeInstanceKey(key) + ".lock"
	path := filepath.Join(os.TempDir(), name)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return true
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = f.Close()
		return false
	}
	instanceOnce.Do(func() {
		instanceLock = f
		instanceLockPath = path
		_ = f.Truncate(0)
		_, _ = f.WriteAt([]byte(strconv.Itoa(os.Getpid())), 0)
	})
	return true
}

// sanitizeInstanceKey 把 key 归一化成安全文件名。
func sanitizeInstanceKey(key string) string {
	var b strings.Builder
	for _, r := range key {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_', r == '.':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	if b.Len() == 0 {
		return "app"
	}
	return b.String()
}

// instanceArgsPath 参数投递文件路径。
func instanceArgsPath(key string) string {
	return filepath.Join(os.TempDir(), "guis-"+sanitizeInstanceKey(key)+".args")
}

// NotifyFirstInstance 把本次启动参数投递给已运行的实例。
func NotifyFirstInstance(key string, args []string) {
	if len(args) == 0 {
		return
	}
	data, err := json.Marshal(args)
	if err != nil {
		return
	}
	f, err := os.OpenFile(instanceArgsPath(key), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.Write(append(data, '\n'))
}

// WatchSecondInstance 轮询二次启动参数（首个实例调用）。
func WatchSecondInstance(key string, fn func(args []string)) (stop func()) {
	if fn == nil {
		return func() {}
	}
	var once sync.Once
	quit := make(chan struct{})
	closeFn := func() { once.Do(func() { close(quit) }) }
	path := instanceArgsPath(key)
	go func() {
		defer recoverReport("second-instance-watch")
		tk := time.NewTicker(500 * time.Millisecond)
		defer tk.Stop()
		for {
			select {
			case <-quit:
				return
			case <-tk.C:
				data, err := os.ReadFile(path)
				if err != nil || len(data) == 0 {
					continue
				}
				// 先清空再解析，避免同一条参数被处理两次
				_ = os.WriteFile(path, nil, 0o600)
				for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
					if line == "" {
						continue
					}
					var args []string
					if err := json.Unmarshal([]byte(line), &args); err != nil {
						continue
					}
					a := args
					postToUI(func() { fn(a) })
				}
			}
		}
	}()
	return closeFn
}
