<!-- group: 媒体与文档 -->
# 办公文档与打印 · Excel / Word / PDF

数据层在 `internal/office`（纯标准库，无第三方依赖），渲染在 `ui/office.go`，打印在 `ui/print.go`。

```go
import (
    "guis/internal/office"
    "guis/ui"
)
```

## 1. 打开与渲染

```go
f, err := office.Open("报表.xlsx")     // .xlsx / .docx / .pdf 自动识别
if err != nil { log.Fatal(err) }
defer func(){}()                       // office.File 无需要关闭的资源

ui.OfficeView("doc", f)                // Excel→表格，Word→文档流，PDF→白纸+定位文本
```

按类型精确渲染：

```go
ui.SpreadsheetView("xl", f.Sheet, activeSheet, func(name string) { activeSheet = name })
ui.WordView("w", f.Word)
ui.PDFView("pdf", f.PDF, 1.0)          // zoom: pt → 逻辑像素
ui.SheetTable("sh", sh, ui.SheetViewOptions{
    MaxRows: 300, MaxCols: 24,          // 超大表截断（页脚提示剩余行列）
    RowH: 30, FontSize: 12,
    ShowColName: true,                  // 无表头时用 A/B/C
    OnCell: func(row, col int) {},      // row = -1 表示表头行
})
```

## 2. 文档模型

| 类型 | 关键字段 / 方法 |
| --- | --- |
| `office.File` | `Path / Kind / Sheet / Word / PDF`、`Summary()` |
| `office.Kind` | `KindUnknown / KindExcel / KindWord / KindPDF`、`String()`、`Ext()` |
| `office.Spreadsheet` | `Sheets []Sheet`、`First()`、`SheetAt(i)`、`SheetByName(name)` |
| `office.Sheet` | `Name / Rows [][]Cell / Cols / Header`、`Cell(r,c)`、`SetCell`、`Set`、`SetNum`、`SortRowsBy(col, asc)`、`SumColumn(col)` |
| `office.Cell` | `Text / Value / Num / Bold / Fill` |
| `office.WordDoc` | `Title / Blocks []Block`、`PlainText()` |
| `office.Block` | `Kind`（`BlockParagraph / Heading / ListItem / Table / Quote`）、`Level / Text / Bold / Rows / HeaderRows / Widths` |
| `office.PDFDoc` | `Pages []PDFPage`、`Title` |
| `office.PDFPage` | `W / H / Items`、`Text / TextColor / TextBold / Line / Rect`（原点左上、Y 向下、单位 pt） |
| `office.PDFItem` | `Kind`（`PDFText / PDFLine / PDFRect / PDFImage`）、坐标、颜色、加粗 |

构造助手：`office.Paragraph / Heading(level, text) / Bullet / Table(rows)`。

## 3. 读写

```go
// Excel
wb, err := office.ReadXLSX("a.xlsx")
err = office.WriteXLSX("b.xlsx", wb)
b, err := office.WriteXLSXBytes(wb)
wb2, err := office.ReadXLSXBytes(b)

// Word
doc, err := office.ReadDOCX("a.docx")
err = office.WriteDOCX("b.docx", doc)
b, err = office.WriteDOCXBytes(doc)
doc2, err := office.ReadDOCXBytes(b)

// PDF（读 + 写）
pdf, err := office.ReadPDF("a.pdf")
err = office.WritePDF("b.pdf", pdf)
n, err := office.PDFPageCount("a.pdf")     // 只数页数，不解析内容
```

PDF 写入选项：

```go
err := office.WritePDFWith("out.pdf", doc, office.PDFOptions{
    Title: "月度报表", Author: "Mr.Shu",
    FontPath:  `C:\Windows\Fonts\msyh.ttc`,  // 空 = 自动找系统中文字体
    FaceIndex: 0,                            // TTC 里的第几个 face
    NoCompress: false,
    NoEmbed:    false,                       // true = 只内置 Helvetica（中文变 '?'）
})
```

* 中文自动子集化嵌入（只嵌入用到的字形）+ 写 `/ToUnicode`，**PDF 里的中文可复制、可检索**。
* `GUIS_PDF_FONT` 可全局指定字体文件。

## 4. 格式转换

```go
blk  := office.TableFromSheet(sh, 200)        // 工作表 → Word 表格块
sh2  := office.SheetFromBlock(blk, "Sheet1")  // Word 表格 → 工作表
doc  := office.DocxFromSpreadsheet(wb, "月度报表")  // 工作簿 → Word（每表一节）
```

## 5. 打印与打印预览

```go
type Printable interface {
    Pages() int
    DrawPage(c *ui.PrintCanvas, page int)
}

// 办公文档直接适配
doc := ui.OfficePrintable(f)      // 任意 → Printable
doc  = ui.SheetPrintable(sh, "工作表1")
doc  = ui.WordPrintable(wd)
doc  = ui.PDFPrintable(pd)

ui.Print("报表.xlsx", doc)                     // 弹系统打印对话框（取消返回 false, nil）
ui.PrintPreview("pv", doc, 1).St(ui.St().Hp(100))  // 预览（A4 比例卡片，可缩放）

ui.Printers()                                  // 打印机列表（默认机排第一）
ui.DefaultPrinter()
ui.PrintTo("Microsoft Print to PDF", "报表", doc)  // 不弹系统框，直连指定打印机

ui.OpenPrintDialog("报表", doc, "系统打印对话框不可用")   // 内置兜底对话框
ui.ClosePrintDialog(); ui.PrintDialogOpen(); ui.PrintDialogLayer()
```

`PrintCanvas`（A4 = `PageW96 x PageH96` = 794×1123，96 DPI）：

```go
c.Text(x, y, size, "文本", "#111111")
c.Rect(x, y, w, h, strokeW, "#cccccc", "#f8fafc")   // fill 空 = 只描边；stroke<=0 = 只填充
c.Line(x1, y1, x2, y2, 1, "#999999")
```

自己实现 `Printable` 即可打印任意内容（报表、标签、票据）。

## 6. 导出 PDF

```go
err := ui.ExportPDF("out.pdf", "标题", doc)   // 任意 Printable → PDF（自动嵌入中文字体子集）
```

因为 `PrintCanvas` 与 `PDFPage` 坐标一致（左上原点、Y 向下、pt），
**打印稿和 PDF 导出是同一份绘制代码**，不会出现「预览对了导出错位」。

## 7. 平台差异

| 能力 | Windows | Linux/macOS | 浏览器 | Android |
| --- | --- | --- | --- | --- |
| 读写 xlsx/docx/pdf | ✅ | ✅ | ✅ | ✅（纯 Go） |
| 渲染 | ✅ | ✅ | ✅ | ✅ |
| 系统打印对话框 | ✅ | ✗ | ✗ | ✗ |
| 内置打印对话框 | ✅ | ✗ | ✗ | ✗ |
| 导出 PDF | ✅ | ✅ | ✅ | ✅ |

用 `ui.HasCap(ui.CapPrint)` 做分支；不支持时按钮置灰并提示。

## 8. 坑位

* `MaxRows / MaxCols` 不设时默认 300 / 24——超大表会静默截断（页脚有提示），需要全量就分批渲染。
* PDF 预览**不走位图**：按文本项重排成绝对定位节点，缩放不糊、跟随主题色。
* 打印对话框在个别机器上 `PrintDlgW` 返回 `CDERR_DIALOGFAILURE`（系统级），
  框架会自动弹出内置对话框作为兜底，不要再把「失败」当成「用户取消」。
