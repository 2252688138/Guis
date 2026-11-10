//go:build darwin && !ios

// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

// NSDraggingDestination：把文件从 Finder / 其它 App 拖进窗口。
//
// 注册的拖动类型是 `public.file-url` 与老的 `NSFilenamesPboardType`：
// 前者是现代 API，后者在拖多选文件时最省事（直接给 NSArray<NSString*> 路径）。
// 只实现「进入 / 更新 / 松手 / 结束」四个回调，够框架拿到路径即可；
// 悬停高亮交给业务用 App.SetOnDropFiles 自己处理（本层不做预览）。
package cocoa

import (
	"guis/internal/objc"

	"github.com/ebitengine/purego"
)

// 拖动操作类型（NSEvent.h）。
const (
	dragOperationNone = 0
	dragOperationCopy = 1
)

// 额外的拖入事件种类（与 cocoa.Event.Kind 对齐）。
const EvDropFiles = 100

// AddDropSupport 给视图类注册拖动类型与回调。
//
// 必须在 ensureViewClass 之后、创建窗口之前调用（只需一次）。
func addDropSupport(cls uintptr) {
	if cls == 0 {
		return
	}
	// NSArray *types = @[@"public.file-url", @"NSFilenamesPboardType"]
	types := objc.Send(objc.Class("NSMutableArray"), "array")
	add := func(s string) {
		objc.Send1(types, "addObject:", objc.NSString(s))
	}
	add("public.file-url")
	add("NSFilenamesPboardType")

	returnCopy := purego.NewCallback(func(self, cmd, sender uintptr) uintptr {
		return dragOperationCopy
	})
	perform := purego.NewCallback(func(self, cmd, sender uintptr) uintptr {
		if w := windowOf(self); w != nil {
			if paths := pasteboardPaths(sender); len(paths) > 0 {
				w.emit(Event{Kind: EvDropFiles, Paths: paths})
			}
		}
		return 1
	})
	conclude := purego.NewCallback(func(self, cmd, sender uintptr) uintptr {
		return 0
	})
	callbacks = append(callbacks, returnCopy, perform, conclude)
	objc.AddMethod(cls, "draggingEntered:", callbacks[len(callbacks)-3], "Q@:@")
	objc.AddMethod(cls, "draggingUpdated:", callbacks[len(callbacks)-3], "Q@:@")
	objc.AddMethod(cls, "performDragOperation:", callbacks[len(callbacks)-2], "c@:@")
	objc.AddMethod(cls, "concludeDragOperation:", callbacks[len(callbacks)-1], "v@:@")

	// 视图实例上注册类型：类方法不行，必须对每个实例调用
	registerTypes = types
}

// registerTypes 拖动类型集合（创建窗口时对视图实例 setDraggingTypes）。
var registerTypes uintptr

// ApplyDropTypes 对刚创建的视图实例注册拖动类型（NewWindow 内部调用）。
func applyDropTypes(view uintptr) {
	if view == 0 || registerTypes == 0 {
		return
	}
	objc.Send1(view, "registerForDraggedTypes:", registerTypes)
}

// pasteboardPaths 从 NSDraggingInfo 的粘贴板里取出文件路径列表。
func pasteboardPaths(sender uintptr) []string {
	if sender == 0 {
		return nil
	}
	pb := objc.Send(sender, "draggingPasteboard")
	if pb == 0 {
		return nil
	}
	// 先试老式列表（一次拿到全部路径）
	if arr := objc.Send1(pb, "propertyListForType:", objc.NSString("NSFilenamesPboardType")); arr != 0 {
		if paths := stringArray(arr); len(paths) > 0 {
			return paths
		}
	}
	// 再试单个 file-url
	if s := objc.Send1(pb, "stringForType:", objc.NSString("public.file-url")); s != 0 {
		if p := objc.UTF8(s); p != "" {
			return []string{p}
		}
	}
	return nil
}

// stringArray NSArray<NSString*> → []string。
func stringArray(arr uintptr) []string {
	if arr == 0 {
		return nil
	}
	n := int(objc.IntOf(arr, "count"))
	if n <= 0 {
		return nil
	}
	out := make([]string, 0, n)
	for i := 0; i < n; i++ {
		if s := objc.Send1(arr, "objectAtIndex:", uintptr(i)); s != 0 {
			if p := objc.UTF8(s); p != "" {
				out = append(out, p)
			}
		}
	}
	return out
}
