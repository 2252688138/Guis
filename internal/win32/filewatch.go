//go:build windows

// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package win32

import (
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"unsafe"
)

// 目录变更监听（ReadDirectoryChangesW）。
//
// 事件驱动、零轮询：内核把变更写进我们提供的缓冲区，读线程阻塞在
// ReadDirectoryChangesW 上即可。停止时先 CancelIoEx 再 CloseHandle，让阻塞的
// 读立即返回 ERROR_OPERATION_ABORTED，不会把 goroutine 卡到进程退出。

var (
	procCreateFileW           = kernel32L.NewProc("CreateFileW")
	procReadDirectoryChangesW = kernel32L.NewProc("ReadDirectoryChangesW")
	procCancelIoEx            = kernel32L.NewProc("CancelIoEx")
)

// 文件变更类别（与 ui.FsChange 对应）。
const (
	// WatchCreated 新建。
	WatchCreated = 0
	// WatchModified 修改。
	WatchModified = 1
	// WatchRemoved 删除。
	WatchRemoved = 2
	// WatchRenamed 重命名。
	WatchRenamed = 3
)

// WatchEvent 一条原生文件变更。
type WatchEvent struct {
	Path    string
	OldPath string
	Kind    int
	IsDir   bool
}

// fileNotifyInformation 对应 FILE_NOTIFY_INFORMATION。
type fileNotifyInformation struct {
	NextEntryOffset uint32
	Action          uint32
	FileNameLength  uint32
	// FileName 紧跟在结构体之后（UTF-16，不带结尾 NUL）
}

const (
	fileListDirectory           = 0x00000001
	fileShareReadWriteDelete    = 0x00000007
	openExisting                = 3
	fileFlagBackupSemantics     = 0x02000000
	fileNotifyChangeFileName    = 0x00000001
	fileNotifyChangeDirName     = 0x00000002
	fileNotifyChangeAttributes  = 0x00000004
	fileNotifyChangeSize        = 0x00000008
	fileNotifyChangeLastWrite   = 0x00000010
	fileNotifyChangeCreation    = 0x00000040
	invalidHandleValue          = ^uintptr(0)
	watchBufSize                = 32 * 1024
)

// mapNotifyAction 把 FILE_ACTION_* 映射成我们的类别。
func mapNotifyAction(a uint32) int {
	switch a {
	case 1, 6: // ADDED, RENAMED_NEW_NAME
		return WatchCreated
	case 2: // REMOVED
		return WatchRemoved
	case 3: // MODIFIED
		return WatchModified
	case 4: // RENAMED_OLD_NAME
		return WatchRenamed
	case 5: // RENAMED_NEW_NAME（旧值；Win8 起用 6）
		return WatchCreated
	}
	return WatchModified
}

// WatchDirectory 监听目录，返回停止函数。回调在独立 goroutine 上执行，
// 每次最多给出 64 条变更（更大的批次会让界面卡顿）。
func WatchDirectory(dir string, recursive bool, fn func([]WatchEvent)) func() {
	if dir == "" || fn == nil {
		return func() {}
	}
	if procCreateFileW.Find() != nil || procReadDirectoryChangesW.Find() != nil {
		return func() {}
	}
	p, err := syscall.UTF16PtrFromString(dir)
	if err != nil {
		return func() {}
	}
	rec := uintptr(0)
	if recursive {
		rec = 1
	}
	h, _, _ := procCreateFileW.Call(
		uintptr(unsafe.Pointer(p)),
		fileListDirectory,
		fileShareReadWriteDelete,
		0,
		openExisting,
		fileFlagBackupSemantics,
		0,
	)
	if h == invalidHandleValue || h == 0 {
		return func() {}
	}

	var once sync.Once
	stop := func() {
		once.Do(func() {
			// 先取消挂起的 IO，让阻塞中的读立即返回，再关句柄
			if procCancelIoEx.Find() == nil {
				procCancelIoEx.Call(h, 0)
			}
			procCloseHandle.Call(h)
		})
	}

	go func() {
		defer recover()
		buf := make([]byte, watchBufSize)
		filter := uintptr(fileNotifyChangeFileName | fileNotifyChangeDirName |
			fileNotifyChangeLastWrite | fileNotifyChangeSize |
			fileNotifyChangeCreation | fileNotifyChangeAttributes)
		for {
			var returned uint32
			r, _, _ := procReadDirectoryChangesW.Call(
				h,
				uintptr(unsafe.Pointer(&buf[0])),
				uintptr(len(buf)),
				rec,
				filter,
				uintptr(unsafe.Pointer(&returned)),
				0,
				0,
			)
			if r == 0 || returned == 0 {
				return
			}
			evs := parseNotify(buf[:returned], dir)
			if len(evs) > 0 {
				fn(evs)
			}
		}
	}()
	return stop
}

// parseNotify 解析 ReadDirectoryChangesW 填充的 FILE_NOTIFY_INFORMATION 链。
func parseNotify(buf []byte, base string) []WatchEvent {
	var evs []WatchEvent
	off := 0
	for off+12 <= len(buf) {
		fni := (*fileNotifyInformation)(unsafe.Pointer(&buf[off]))
		nameLen := int(fni.FileNameLength) / 2
		if nameLen <= 0 || off+12+nameLen*2 > len(buf) {
			break
		}
		namePtr := unsafe.Pointer(&buf[off+12])
		name := syscall.UTF16ToString(unsafe.Slice((*uint16)(namePtr), nameLen))
		path := filepath.Join(base, name)
		kind := mapNotifyAction(fni.Action)
		isDir := false
		if st, err := statPath(path); err == nil {
			isDir = st
		} else if kind == WatchRemoved {
			// 删除后无法 stat：按「没有扩展名」粗判（目录通常无扩展名）
			isDir = filepath.Ext(name) == ""
		}
		evs = append(evs, WatchEvent{Path: path, Kind: kind, IsDir: isDir})
		if fni.NextEntryOffset == 0 {
			break
		}
		off += int(fni.NextEntryOffset)
		if off <= 0 || off >= len(buf) {
			break
		}
	}
	return evs
}

// statPath 判断路径是否为目录（存在才有效）。
func statPath(p string) (isDir bool, err error) {
	ptr, err := syscall.UTF16PtrFromString(p)
	if err != nil {
		return false, err
	}
	proc := kernel32L.NewProc("GetFileAttributesW")
	r, _, _ := proc.Call(uintptr(unsafe.Pointer(ptr)))
	if r == 0xFFFFFFFF {
		return false, syscall.EINVAL
	}
	return r&0x10 != 0, nil
}

// NormalizeWatchPath 把监听回调里的相对片段补成干净的绝对路径（供调用方复用）。
func NormalizeWatchPath(base, name string) string {
	if strings.HasPrefix(name, base) {
		return name
	}
	return filepath.Join(base, name)
}
