# 列模式编辑 (Column Edit Mode) 实现说明

## 概述

列模式编辑(又称块编辑、垂直编辑)允许用户选择一个跨越多行的矩形区域,并同时在所有行上进行相同的编辑操作。这是现代编辑器(如GoLand、VS Code等)的重要功能。

## 实现架构

### 核心数据结构

#### 1. EditorMode 扩展 (mode.go)

```go
const (
    ModeNormal     // 普通模式
    ModeReadOnly   // 只读模式
    ModeSnippet    // 代码片段模式
    ModeColumnEdit // 列编辑模式 (新增)
)
```

#### 2. columnEditState 结构体 (editor.go)

```go
type columnEditState struct {
    enabled     bool           // 列编辑模式是否启用
    selections  []columnCursor // 多个光标位置
    anchor      image.Point    // 鼠标拖动起点
}

type columnCursor struct {
    line   int  // 行号(0-based)
    col    int  // 列号(以rune计,0-based)
    startX int  // 起始像素X坐标
    endX   int  // 结束像素X坐标
}
```

## 功能实现

### 1. 模式切换 (mode.go)

#### 启用/禁用列编辑模式

```go
func (e *Editor) SetColumnEditMode(enabled bool) {
    if enabled {
        e.mode = ModeColumnEdit
        e.columnEdit.enabled = true
    } else {
        e.clearColumnEdit()
    }
}

func (e *Editor) clearColumnEdit() {
    e.columnEdit.enabled = false
    e.columnEdit.selections = nil
    if e.mode == ModeColumnEdit {
        e.mode = ModeNormal
    }
}

func (e *Editor) ColumnEditEnabled() bool {
    return e.columnEdit.enabled || e.mode == ModeColumnEdit
}
```

### 2. 鼠标事件处理 (event.go)

#### 点击事件处理

关键点:
- 区分点击启动和拖动更新
- 防止重复启动选择
- 支持Alt+Click快捷方式

```go
case gesture.ClickEvent:
    if e.ColumnEditEnabled() {
        // 如果已有selections,只设置dragging标志
        // 否则启动新的列选择
        if len(e.columnEdit.selections) == 0 {
            e.startColumnSelection(gtx, pos)
        }
        e.dragging = true
        return nil, true
    }

    // Alt+Click 也启动列选择
    if evt.Modifiers.Contain(key.ModAlt) {
        e.startColumnSelection(gtx, pos)
        e.dragging = true
        return nil, true
    }
```

#### 拖动事件处理

```go
case pointer.Event:
    if e.ColumnEditEnabled() && e.dragging {
        e.updateColumnSelection(gtx, pos)
        if release {
            e.dragging = false
        }
    }
```

### 3. 列选择管理 (event.go)

#### 启动列选择

```go
func (e *Editor) startColumnSelection(gtx layout.Context, pos image.Point) {
    e.SetColumnEditMode(true)
    e.columnEdit.anchor = pos

    // 查询点击位置对应的行和列
    line, col, runeOff := e.text.QueryPos(pos)

    if runeOff >= 0 {
        e.columnEdit.selections = []columnCursor{{
            line:   line,
            col:    col,
            startX: pos.X,
            endX:   pos.X,
        }}
    }
}
```

#### 更新列选择

核心算法:
1. 根据anchor和当前鼠标位置计算矩形范围
2. 将屏幕Y坐标转换为行号
3. 对每行创建一个光标位置

```go
func (e *Editor) updateColumnSelection(gtx layout.Context, pos image.Point) {
    anchor := e.columnEdit.anchor

    // 计算选择范围
    startX := min(anchor.X, pos.X)
    endX := max(anchor.X, pos.X)
    startY := min(anchor.Y, pos.Y)
    endY := max(anchor.Y, pos.Y)

    // 获取行高(注意: fixed.Int26_6需要Round())
    lineHeight := e.text.GetLineHeight().Round()
    scrollOff := e.text.ScrollOff()

    // 转换Y坐标为行号
    startLine := (startY + scrollOff.Y) / lineHeight
    endLine := (endY + scrollOff.Y) / lineHeight

    e.columnEdit.selections = nil
    totalLines := e.text.Paragraphs()

    // 遍历每行创建光标
    for lineNum := startLine; lineNum <= endLine; lineNum++ {
        if lineNum < 0 || lineNum >= totalLines {
            continue
        }

        screenY := lineNum*lineHeight - scrollOff.Y
        startPos := image.Point{X: startX, Y: screenY}

        // 查询该行的列位置
        _, col, off := e.text.QueryPos(startPos)

        if off >= 0 {
            e.columnEdit.selections = append(e.columnEdit.selections,
                columnCursor{
                    line:   lineNum,
                    col:    col,
                    startX: startX,
                    endX:   endX,
                })
        }
    }
}
```

### 4. 视觉渲染 (editor.go)

#### 绘制列选择区域

```go
func (e *Editor) paintColumnSelection(gtx layout.Context, material color.Color) {
    lineHeight := e.text.GetLineHeight().Round()
    scrollOff := e.text.ScrollOff()

    for _, cursor := range e.columnEdit.selections {
        lineY := cursor.line * lineHeight
        screenY := lineY - scrollOff.Y

        // 计算可见性
        if screenY < -lineHeight || screenY > gtx.Constraints.Max.Y {
            continue
        }

        // 绘制矩形
        startX := cursor.startX - scrollOff.X
        endX := cursor.endX - scrollOff.X
        width := max(endX-startX, 2) // 最小宽度

        material.Op(gtx.Ops).Add(gtx.Ops)
        stack := clip.Rect(image.Rect(startX, screenY,
            startX+width, screenY+lineHeight)).Push(gtx.Ops)
        paint.PaintOp{}.Add(gtx.Ops)
        stack.Pop()
    }
}
```

在Layout中调用:
```go
if e.ColumnEditEnabled() && len(e.columnEdit.selections) > 0 {
    e.paintColumnSelection(gtx, selectColor)
}
```

### 5. 编辑操作 (editor.go)

#### 删除操作

```go
func (e *Editor) Delete(graphemeClusters int) int {
    if e.ColumnEditEnabled() && len(e.columnEdit.selections) > 0 {
        return e.onColumnEditDelete(graphemeClusters)
    }
    // 普通删除逻辑...
}

func (e *Editor) onColumnEditDelete(graphemeClusters int) int {
    e.buffer.GroupOp() // 组合操作以便撤销

    for i := range e.columnEdit.selections {
        cursor := &e.columnEdit.selections[i]
        runeOff, _ := e.ConvertPos(cursor.line, cursor.col)

        start := runeOff
        end := runeOff

        if graphemeClusters > 0 {
            end = runeOff + graphemeClusters
        } else {
            start = runeOff + graphemeClusters
        }

        e.replace(start, end, "")

        if graphemeClusters < 0 {
            cursor.col += graphemeClusters
        }
    }

    e.buffer.UnGroupOp()
    return deletedRunes
}
```

#### 输入操作 (event.go)

```go
func (e *Editor) onColumnEditInput(ke key.EditEvent) {
    if len(e.columnEdit.selections) == 0 {
        return
    }

    e.buffer.GroupOp()

    for _, cursor := range e.columnEdit.selections {
        runeOff, _ := e.ConvertPos(cursor.line, cursor.col)
        e.replace(runeOff, runeOff, ke.Text)
    }

    e.buffer.UnGroupOp()

    // 更新光标位置
    for i := range e.columnEdit.selections {
        e.columnEdit.selections[i].col += utf8.RuneCountInString(ke.Text)
    }
}
```

## 关键技术点

### 1. fixed.Int26_6 类型处理

Gio的`text`包使用`fixed.Int26_6`定点数类型表示像素值。

```go
// 错误: 直接转换会得到错误的值
lineHeight := int(e.text.GetLineHeight()) // 可能得到1728

// 正确: 先Round()再转换
lineHeight := e.text.GetLineHeight().Round() // 得到~20
```

### 2. 坐标系统转换

```
屏幕坐标 ↔ 文档坐标
------------------------------------------------------
ScreenY = LineNum * lineHeight - scrollOff.Y
LineNum = (ScreenY + scrollOff.Y) / lineHeight
```

### 3. 查询位置

```go
// 从屏幕坐标查询文本位置
line, col, runeOff := e.text.QueryPos(screenPos)

// 从行列号查询rune偏移
runeOff, pixelPos := e.ConvertPos(line, col)
```

### 4. 操作组合

使用`GroupOp()`和`UnGroupOp()`组合多个编辑操作,使其作为一次撤销。

## 快捷键

| 快捷键 | 功能 |
|--------|------|
| Alt+C  | 切换列编辑模式 |
| Alt+Click | 快速启动列选择 |
| 鼠标拖动 | 创建矩形选择区域 |
| 输入文字 | 同时插入到所有选择位置 |
| Delete/Backspace | 同时删除所有选择位置的字符 |

## 使用示例

### 场景1: 多行添加注释

```
1. 按Alt+C启用列编辑模式
2. 在第一行行首按下鼠标
3. 拖动选择多行的起始位置
4. 输入 "//" 
5. 所有行同时添加注释
```

### 场景2: 修改变量名

```
1. 选中变量名
2. 按Alt+C
3. 在多行的相同位置拖动选择
4. 输入新变量名
5. 所有位置的变量同时修改
```

## 已知限制

1. **Tab键处理**: 目前列编辑模式下Tab键的行为需要进一步优化
2. **复杂文本**: 对于包含组合字符或特殊符号的文本,列边界计算可能不准确
3. **性能**: 对于大量行的文档,频繁更新列选择可能影响性能

## 调试日志

代码中包含详细的调试日志,格式为:

```
[ColumnEdit] Mouse click detected, Modifiers: X HasAlt: Y NumClicks: Z ColumnEditEnabled: W
[ColumnEdit] startColumnSelection called at pos: (x, y)
[ColumnEdit] Queried position - line: X col: Y runeOff: Z
[ColumnEdit] updateColumnSelection - anchor: (x1, y1) current: (x2, y2)
[ColumnEdit] Created N column selections
```

可通过日志追踪列选择的创建和更新过程。

## 测试建议

1. **基础功能**
   - 模式切换是否正常
   - 点击是否创建初始光标
   - 拖动是否更新选择区域

2. **边界情况**
   - 点击文档外区域
   - 选择跨超大范围
   - 空文档或单行文档

3. **编辑操作**
   - 输入单个字符
   - 输入多字符字符串
   - 删除操作
   - 撤销/重做

4. **视觉渲染**
   - 选择区域是否正确显示
   - 滚动时选择是否正确更新
   - 不同行高下的显示

## 未来改进

1. 支持使用Shift+方向键扩展列选择
2. 添加列选择复制/粘贴功能
3. 支持列选择的保存和加载
4. 优化大量行时的性能
5. 添加列选择的键盘快捷键(如Alt+Shift+方向键)
