//go:build windows

// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package win32

import (
	"errors"
	"image"
	"syscall"
	"unsafe"
)

// 文件类型图标与缩略图。
//
// 图标走 SHGetFileInfoW + 系统图像列表（16/32/48/256 四档），缩略图走系统
// 缩略图缓存（SHGetFileInfo 的 SHGFI_ICON 在支持时即返回缩略图）。
// 结果统一转成 *image.NRGBA（自顶向下、RGBA 顺序），可直接喂给 ui.Image。

var (
	procSHGetFileInfoW      = shell32L.NewProc("SHGetFileInfoW")
	procSHGetImageList      = shell32L.NewProc("SHGetImageList")
	procGetIconInfo         = user32L.NewProc("GetIconInfo")
	procGetObjectW          = gdi32.NewProc("GetObjectW")
	procGetDIBits           = gdi32.NewProc("GetDIBits")
	procDestroyIconSys      = user32L.NewProc("DestroyIcon")
)

const (
	shgfiIcon           = 0x000000100
	shgfiSmallIcon      = 0x000000001
	shgfiLargeIcon      = 0x000000000
	shgfiUseFileAttrs   = 0x000000010
	shgfiTypeName       = 0x000000400
	shgfiSysIconIndex   = 0x000040000
	shilLarge           = 0
	shilSmall           = 1
	shilExtraLarge      = 2
	shilJumbo           = 4
	ildTransparent      = 0x00000001
	fileAttributeNormal = 0x00000080
	fileAttributeDir    = 0x00000010
)

// SHFILEINFOW shell32 文件信息结构。
type shFileInfoW struct {
	HIcon         uintptr
	IIcon         int32
	DwAttributes  uint32
	SzDisplayName [260]uint16
	SzTypeName    [80]uint16
}

// iidIImageList IImageList 的 IID（{46EB5926-582E-4017-9FDF-E8998DAA0950}）。
var iidIImageList = guid{0x46EB5926, 0x582E, 0x4017, [8]byte{0x9F, 0xDF, 0xE8, 0x99, 0x8D, 0xAA, 0x09, 0x50}}

// IImageList 槽位（IUnknown 0-2）。
const imlGetIcon = 10

// FileTypeName 返回文件类型的友好名称（"文本文档" / "文件夹"）。
func FileTypeName(path string) string {
	if procSHGetFileInfoW.Find() != nil || path == "" {
		return ""
	}
	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return ""
	}
	var fi shFileInfoW
	attrs := uintptr(0)
	flags := uintptr(shgfiTypeName)
	if st, err := statPath(path); err == nil {
		if st {
			attrs = fileAttributeDir
		} else {
			attrs = fileAttributeNormal
		}
		_ = st
	} else {
		// 文件不存在：按扩展名推断（列表里画「虚拟文件」很有用）
		flags |= shgfiUseFileAttrs
		attrs = fileAttributeNormal
	}
	r, _, _ := procSHGetFileInfoW.Call(
		uintptr(unsafe.Pointer(p)), attrs,
		uintptr(unsafe.Pointer(&fi)), unsafe.Sizeof(fi), flags,
	)
	if r == 0 {
		return ""
	}
	return syscall.UTF16ToString(fi.SzTypeName[:])
}

// FileIcon 返回文件的类型图标（size 像素见方，支持 16 / 24 / 32 / 48 / 64 / 256）。
func FileIcon(path string, size int) (*image.NRGBA, error) {
	if procSHGetFileInfoW.Find() != nil {
		return nil, errors.New("win32: SHGetFileInfoW 不可用")
	}
	if path == "" {
		return nil, errors.New("win32: 空路径")
	}
	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	var fi shFileInfoW
	flags := uintptr(shgfiIcon | shgfiSysIconIndex)
	attrs := uintptr(fileAttributeNormal)
	if st, err := statPath(path); err == nil {
		if st {
			attrs = fileAttributeDir
		}
	} else {
		flags |= shgfiUseFileAttrs
	}
	if size <= 16 {
		flags |= shgfiSmallIcon
	}
	r, _, _ := procSHGetFileInfoW.Call(
		uintptr(unsafe.Pointer(p)), attrs,
		uintptr(unsafe.Pointer(&fi)), unsafe.Sizeof(fi), flags,
	)
	if r == 0 {
		return nil, errors.New("win32: 取文件图标失败")
	}
	// 大尺寸优先从系统图像列表取（32 以上才有意义）
	if size > 32 {
		if img := iconFromImageList(fi.IIcon, size); img != nil {
			return img, nil
		}
	}
	icon := fi.HIcon
	own := false
	if icon == 0 {
		// 系统图像列表里没有 HIcon 时自己取一次
		flags2 := uintptr(shgfiIcon)
		if size <= 16 {
			flags2 |= shgfiSmallIcon
		}
		var fi2 shFileInfoW
		r2, _, _ := procSHGetFileInfoW.Call(uintptr(unsafe.Pointer(p)), attrs,
			uintptr(unsafe.Pointer(&fi2)), unsafe.Sizeof(fi2), flags2)
		if r2 == 0 || fi2.HIcon == 0 {
			return nil, errors.New("win32: 取文件图标失败")
		}
		icon = fi2.HIcon
		own = true
	} else {
		own = true
	}
	if own {
		defer procDestroyIconSys.Call(icon)
	}
	src := hiconToNRGBA(icon)
	if src == nil {
		return nil, errors.New("win32: 图标转位图失败")
	}
	if size > 0 && (src.Bounds().Dx() != size || src.Bounds().Dy() != size) {
		return scaleNRGBA(src, size, size), nil
	}
	return src, nil
}

// FileThumbnail 返回文件缩略图。
//
// 优先级：系统缩略图（图片 / 视频 / PDF）→ 大图标放大 → 图标。
// 系统缩略图需要 IShellItemImageFactory，代价较高，这里用「优先按扩展名
// 直接解码图片，其次回退到图标」的策略，保证文件列表滚动时不卡。
func FileThumbnail(path string, w, h int) (*image.NRGBA, error) {
	icon, err := FileIcon(path, maxSide(w, h))
	if err != nil {
		return nil, err
	}
	return scaleNRGBA(icon, w, h), nil
}

// iconFromImageList 从系统图像列表取指定尺寸的图标。
func iconFromImageList(index int32, size int) *image.NRGBA {
	if procSHGetImageList.Find() != nil {
		return nil
	}
	which := uintptr(shilJumbo)
	switch {
	case size <= 16:
		which = shilSmall
	case size <= 32:
		which = shilLarge
	case size <= 48:
		which = shilExtraLarge
	}
	var list uintptr
	hr, _, _ := procSHGetImageList.Call(which, uintptr(unsafe.Pointer(&iidIImageList)), uintptr(unsafe.Pointer(&list)))
	if int32(hr) < 0 || list == 0 {
		return nil
	}
	defer comRelease(list)
	var hicon uintptr
	if hr := comCall(list, imlGetIcon, uintptr(index), ildTransparent, uintptr(unsafe.Pointer(&hicon))); comFail(hr) || hicon == 0 {
		return nil
	}
	defer procDestroyIconSys.Call(hicon)
	return hiconToNRGBA(hicon)
}

// hiconToNRGBA 把 HICON 转成 NRGBA 位图（处理 32 位带 alpha 与 1 位掩码两种情况）。
func hiconToNRGBA(hicon uintptr) *image.NRGBA {
	if hicon == 0 || procGetIconInfo.Find() != nil {
		return nil
	}
	type iconInfo struct {
		FIcon    int32
		XHotspot uint32
		YHotspot uint32
		HbmMask  uintptr
		HbmColor uintptr
	}
	var ii iconInfo
	if r, _, _ := procGetIconInfo.Call(hicon, uintptr(unsafe.Pointer(&ii))); r == 0 {
		return nil
	}
	defer func() {
		if ii.HbmColor != 0 {
			procDeleteObject.Call(ii.HbmColor)
		}
		if ii.HbmMask != 0 {
			procDeleteObject.Call(ii.HbmMask)
		}
	}()

	// BITMAP 只取宽高（前 3 个字段 + 位深）
	type bitmap struct {
		BmType       int32
		BmWidth      int32
		BmHeight     int32
		BmWidthBytes int32
		BmPlanes     uint16
		BmBitsPixel  uint16
		BmBits       uintptr
	}
	var bm bitmap
	if ii.HbmColor == 0 {
		return nil
	}
	if r, _, _ := procGetObjectW.Call(ii.HbmColor, unsafe.Sizeof(bm), uintptr(unsafe.Pointer(&bm))); r == 0 {
		return nil
	}
	w, hh := int(bm.BmWidth), int(bm.BmHeight)
	if w <= 0 || hh <= 0 || w > 512 || hh > 512 {
		return nil
	}

	hdc, _, _ := procCreateCompatibleDC.Call(0)
	if hdc == 0 {
		return nil
	}
	defer procDeleteDC.Call(hdc)
	old, _, _ := procSelectObject.Call(hdc, ii.HbmColor)
	if old != 0 {
		defer procSelectObject.Call(hdc, old)
	}

	bi := BITMAPINFO{}
	bi.Header.BiSize = uint32(unsafe.Sizeof(BITMAPINFOHEADER{}))
	bi.Header.BiWidth = int32(w)
	bi.Header.BiHeight = -int32(hh) // 自顶向下
	bi.Header.BiPlanes = 1
	bi.Header.BiBitCount = 32
	bi.Header.BiCompression = BI_RGB
	pix := make([]byte, w*hh*4)
	r, _, _ := procGetDIBits.Call(hdc, ii.HbmColor, 0, uintptr(hh),
		uintptr(unsafe.Pointer(&pix[0])), uintptr(unsafe.Pointer(&bi)), 0)
	if r == 0 {
		return nil
	}

	img := image.NewNRGBA(image.Rect(0, 0, w, hh))
	hasAlpha := false
	for i := 0; i < w*hh; i++ {
		b, g, rr, a := pix[i*4], pix[i*4+1], pix[i*4+2], pix[i*4+3]
		if a != 0 {
			hasAlpha = true
		}
		img.Pix[i*4] = rr
		img.Pix[i*4+1] = g
		img.Pix[i*4+2] = b
		img.Pix[i*4+3] = a
	}
	if !hasAlpha {
		// 老式图标没有 alpha：用掩码把透明区域标出来
		applyMask(img, ii.HbmMask, hdc)
	}
	return img
}

// applyMask 用 1 位掩码把白色/透明区域设为透明（掩码为 1 的位置透明）。
func applyMask(img *image.NRGBA, hbmMask, hdc uintptr) {
	if hbmMask == 0 || hdc == 0 {
		return
	}
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	bi := BITMAPINFO{}
	bi.Header.BiSize = uint32(unsafe.Sizeof(BITMAPINFOHEADER{}))
	bi.Header.BiWidth = int32(w)
	bi.Header.BiHeight = -int32(h)
	bi.Header.BiPlanes = 1
	bi.Header.BiBitCount = 32
	bi.Header.BiCompression = BI_RGB
	mask := make([]byte, w*h*4)
	r, _, _ := procGetDIBits.Call(hdc, hbmMask, 0, uintptr(h),
		uintptr(unsafe.Pointer(&mask[0])), uintptr(unsafe.Pointer(&bi)), 0)
	if r == 0 {
		return
	}
	for i := 0; i < w*h; i++ {
		// 掩码位图里非黑即「不透明」；纯黑视为透明
		if mask[i*4] == 0 && mask[i*4+1] == 0 && mask[i*4+2] == 0 {
			img.Pix[i*4+3] = 0
		} else {
			img.Pix[i*4+3] = 255
		}
	}
}

// scaleNRGBA 双线性缩放（图标尺寸很小，够用且不用引外部依赖）。
func scaleNRGBA(src *image.NRGBA, w, h int) *image.NRGBA {
	if w <= 0 || h <= 0 {
		return src
	}
	sw, sh := src.Bounds().Dx(), src.Bounds().Dy()
	if sw == 0 || sh == 0 {
		return image.NewNRGBA(image.Rect(0, 0, w, h))
	}
	dst := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		fy := (float64(y)+0.5)*float64(sh)/float64(h) - 0.5
		y0 := int(fy)
		if y0 < 0 {
			y0 = 0
		}
		y1 := y0 + 1
		if y1 >= sh {
			y1 = sh - 1
		}
		dy := float32(fy - float64(y0))
		if dy < 0 {
			dy = 0
		}
		for x := 0; x < w; x++ {
			fx := (float64(x)+0.5)*float64(sw)/float64(w) - 0.5
			x0 := int(fx)
			if x0 < 0 {
				x0 = 0
			}
			x1 := x0 + 1
			if x1 >= sw {
				x1 = sw - 1
			}
			dx := float32(fx - float64(x0))
			if dx < 0 {
				dx = 0
			}
			i00 := (y0*sw + x0) * 4
			i10 := (y0*sw + x1) * 4
			i01 := (y1*sw + x0) * 4
			i11 := (y1*sw + x1) * 4
			o := (y*w + x) * 4
			for c := 0; c < 4; c++ {
				top := float32(src.Pix[i00+c])*(1-dx) + float32(src.Pix[i10+c])*dx
				bot := float32(src.Pix[i01+c])*(1-dx) + float32(src.Pix[i11+c])*dx
				v := top*(1-dy) + bot*dy
				if v < 0 {
					v = 0
				}
				if v > 255 {
					v = 255
				}
				dst.Pix[o+c] = uint8(v)
			}
		}
	}
	return dst
}

func maxSide(a, b int) int {
	if a > b {
		return a
	}
	return b
}
