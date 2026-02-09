package gvcode

import (
	"image"
	"io"
	"log"
	"math"
	"strings"
	"unicode"
	"unicode/utf8"

	"gioui.org/gesture"
	"gioui.org/io/clipboard"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/io/transfer"
	"gioui.org/layout"
	gestureExt "github.com/oligo/gvcode/internal/gesture"
	"github.com/oligo/gvcode/textview"
)

func (e *Editor) processEvents(gtx layout.Context) (ev EditorEvent, ok bool) {
	if len(e.pending) > 0 {
		out := e.pending[0]
		e.pending = e.pending[:copy(e.pending, e.pending[1:])]
		return out, true
	}
	selStart, selEnd := e.Selection()
	defer func() {
		afterSelStart, afterSelEnd := e.Selection()
		if selStart != afterSelStart || selEnd != afterSelEnd {
			// Selection changed - mark word highlighter active regardless
			// of whether SelectEvent is returned now or queued for later
			e.wordHighlighter.MarkActive(true)
			if ok {
				e.pending = append(e.pending, SelectEvent{})
			} else {
				ev = SelectEvent{}
				ok = true
			}
		}

		switch ev.(type) {
		case ChangeEvent:
			e.wordHighlighter.MarkActive(false)
		}
	}()

	ev, ok = e.processPointer(gtx)
	if ok {
		return ev, ok
	}
	ev, ok = e.processKey(gtx)
	if ok {
		return ev, ok
	}
	return nil, false
}

func (e *Editor) processPointer(gtx layout.Context) (EditorEvent, bool) {
	var scrollX, scrollY pointer.ScrollRange
	textDims := e.text.FullDimensions()
	visibleDims := e.text.Dimensions()

	scrollOffX := e.text.ScrollOff().X
	scrollX.Min = -scrollOffX
	scrollX.Max = max(0, textDims.Size.X-(scrollOffX+visibleDims.Size.X))

	scrollOffY := e.text.ScrollOff().Y
	scrollY.Min = -scrollOffY
	scrollY.Max = max(0, textDims.Size.Y-(scrollOffY+visibleDims.Size.Y))
	sbounds := e.text.ScrollBounds()

	var soff, smin, smax int
	sdist := e.scroller.Update(gtx.Metric, gtx.Source, gtx.Now, scrollX, scrollY)
	if e.scroller.Direction() == gestureExt.Horizontal {
		e.text.ScrollRel(sdist, 0)
		soff = e.text.ScrollOff().X
		smin, smax = sbounds.Min.X, sbounds.Max.X
	} else {
		e.text.ScrollRel(0, sdist)
		soff = e.text.ScrollOff().Y
		smin, smax = sbounds.Min.Y, sbounds.Max.Y
	}

	for {
		evt, ok := e.clicker.Update(gtx.Source)
		if !ok {
			break
		}
		ev, ok := e.processPointerEvent(gtx, evt)
		if ok {
			return ev, ok
		}
	}
	for {
		evt, ok := e.dragger.Update(gtx.Metric, gtx.Source, gesture.Both)
		if !ok {
			break
		}
		ev, ok := e.processPointerEvent(gtx, evt)
		if ok {
			return ev, ok
		}
	}

	if (sdist > 0 && soff >= smax) || (sdist < 0 && soff <= smin) {
		e.scroller.Stop()
	}

	// detects hover event.
	hoverEvent, ok := e.hover.Update(gtx)
	if ok {
		switch hoverEvent.Kind {
		case gestureExt.KindHovered:
			line, col, runeOff := e.text.QueryPos(hoverEvent.Position)
			if runeOff >= 0 {
				return HoverEvent{PixelOff: hoverEvent.Position, Pos: Position{Line: line, Column: col, Runes: runeOff}}, ok
			}
		case gestureExt.KindCancelled:
			return HoverEvent{IsCancel: true}, ok
		}
	}

	return nil, false
}

func (e *Editor) processPointerEvent(gtx layout.Context, ev event.Event) (EditorEvent, bool) {
	switch evt := ev.(type) {
	case gesture.ClickEvent:
		switch {
		case evt.Kind == gesture.KindPress && evt.Source == pointer.Mouse,
			evt.Kind == gesture.KindClick && evt.Source != pointer.Mouse:
			// Debug log for mouse click
			println("[ColumnEdit] Mouse click detected, Modifiers:", evt.Modifiers, "HasAlt:", evt.Modifiers.Contain(key.ModAlt), "NumClicks:", evt.NumClicks, "ColumnEditEnabled:", e.ColumnEditEnabled())

			// Calculate click position
			pos := image.Point{
				X: int(math.Round(float64(evt.Position.X))),
				Y: int(math.Round(float64(evt.Position.Y))),
			}

			// Handle column selection
			if e.ColumnEditEnabled() {
				// Check if we already have an active column selection
				// If yes, don't restart - let the drag handler update it
				if len(e.columnEdit.selections) == 0 {
					println("[ColumnEdit] Column edit mode active, starting column selection")
					e.startColumnSelection(gtx, pos)
					e.dragging = true // Set dragging flag for column mode
				} else {
					println("[ColumnEdit] Column edit mode active, existing selections, skipping restart")
					e.dragging = true
				}
				return nil, true
			}

			// Check for Alt+Click to start column selection (when column edit mode is not yet enabled)
			if evt.Modifiers.Contain(key.ModAlt) {
				println("[ColumnEdit] Alt+Click detected, starting column selection")
				e.startColumnSelection(gtx, pos)
				e.dragging = true
				return nil, true
			}

			prevCaretPos, _ := e.text.Selection()
			e.blinkStart = gtx.Now
			e.text.MoveCoord(image.Point{
				X: int(math.Round(float64(evt.Position.X))),
				Y: int(math.Round(float64(evt.Position.Y))),
			})
			gtx.Execute(key.FocusCmd{Tag: e})
			if e.mode != ModeReadOnly {
				gtx.Execute(key.SoftKeyboardCmd{Show: true})
			}
			if e.scroller.State() != gestureExt.StateFlinging {
				e.scrollCaret = true
			}

			if evt.Modifiers == key.ModShift {
				start, end := e.text.Selection()
				// If they clicked closer to the end, then change the end to
				// where the caret used to be (effectively swapping start & end).
				if abs(end-start) < abs(start-prevCaretPos) {
					e.text.SetCaret(start, prevCaretPos)
				}
			} else {
				e.text.ClearSelection()
			}
			e.dragging = true

			// Process multi-clicks.
			switch {
			case evt.NumClicks == 2:
				e.text.MoveWords(-1, textview.SelectionClear)
				e.text.MoveWords(1, textview.SelectionExtend)
				e.dragging = false
			case evt.NumClicks >= 3:
				e.text.MoveLineStart(textview.SelectionClear)
				e.text.MoveLineEnd(textview.SelectionExtend)
				e.dragging = false
			}

			if e.completor != nil {
				e.completor.Cancel()
			}
			// switch to normal mode when clicked.
			if e.mode == ModeSnippet {
				e.setMode(ModeNormal)
			}
		}
	case pointer.Event:
		release := false
		switch {
		case evt.Kind == pointer.Release && evt.Source == pointer.Mouse:
			release = true
			fallthrough
		case evt.Kind == pointer.Drag && evt.Source == pointer.Mouse:
			// Handle column selection drag
			if e.ColumnEditEnabled() && e.dragging {
				e.updateColumnSelection(gtx, image.Point{
					X: int(math.Round(float64(evt.Position.X))),
					Y: int(math.Round(float64(evt.Position.Y))),
				})
				if release {
					e.dragging = false
				}
			} else if e.dragging {
				e.blinkStart = gtx.Now
				e.text.MoveCoord(image.Point{
					X: int(math.Round(float64(evt.Position.X))),
					Y: int(math.Round(float64(evt.Position.Y))),
				})
				e.scrollCaret = true

				if release {
					e.dragging = false
				}
			}
		}
	}
	return nil, false
}

func (e *Editor) processKey(gtx layout.Context) (EditorEvent, bool) {
	if e.text.Changed() {
		return ChangeEvent{}, true
	}

	if evt := e.processEditEvents(gtx); evt != nil {
		return evt, true
	}

	if evt := e.processCommands(gtx); evt != nil {
		return evt, true
	}

	if e.text.Changed() {
		return ChangeEvent{}, true
	}

	return nil, false
}

func (e *Editor) processEditEvents(gtx layout.Context) EditorEvent {
	filters := []event.Filter{
		key.FocusFilter{Target: e},
		transfer.TargetFilter{Target: e, Type: "application/text"},
	}

	for {
		evt, ok := gtx.Event(filters...)
		if !ok {
			break
		}

		e.blinkStart = gtx.Now

		switch ke := evt.(type) {
		case key.FocusEvent:
			// Reset IME state.
			e.ime.imeState = imeState{}
			if ke.Focus && e.mode != ModeReadOnly {
				gtx.Execute(key.SoftKeyboardCmd{Show: true})
			}
		case key.SnippetEvent:
			e.updateSnippet(gtx, ke.Start, ke.End)
		case key.EditEvent:
			e.onTextInput(ke)
		case key.SelectionEvent:
			e.scrollCaret = true
			e.scroller.Stop()
			e.text.SetCaret(ke.Start, ke.End)

			// Complete a paste event, initiated by Shortcut-V in Editor.command().
		case transfer.DataEvent:
			if evt := e.onPasteEvent(ke); evt != nil {
				return evt
			}
		}
	}
	if e.text.Changed() {
		return ChangeEvent{}
	}

	return nil
}

// updateSnippet queues a key.SnippetCmd if the snippet content or position
// have changed. off and len are in runes.
func (e *Editor) updateSnippet(gtx layout.Context, start, end int) {
	if start > end {
		start, end = end, start
	}
	length := e.text.Len()
	if start > length {
		start = length
	}
	if end > length {
		end = length
	}
	e.ime.start = start
	e.ime.end = end
	startOff := e.text.ByteOffset(start)
	endOff := e.text.ByteOffset(end)
	n := endOff - startOff
	if n > int64(len(e.ime.scratch)) {
		e.ime.scratch = make([]byte, n)
	}
	scratch := e.ime.scratch[:n]
	read, _ := e.buffer.ReadAt(scratch, startOff)

	if read != len(scratch) {
		panic("e.rr.Read truncated data")
	}
	newSnip := key.Snippet{
		Range: key.Range{
			Start: e.ime.start,
			End:   e.ime.end,
		},
		Text: e.ime.snippet.Text,
	}
	if string(scratch) != newSnip.Text {
		newSnip.Text = string(scratch)
	}
	if newSnip == e.ime.snippet {
		return
	}
	e.ime.snippet = newSnip
	gtx.Execute(key.SnippetCmd{Tag: e, Snippet: newSnip})
}

func (e *Editor) onCopyCut(gtx layout.Context, k key.Event) EditorEvent {
	lineOp := false
	if e.text.SelectionLen() == 0 {
		lineOp = true
		e.scratch, _, _ = e.text.SelectedLineText(e.scratch)
		if len(e.scratch) > 0 && e.scratch[len(e.scratch)-1] != '\n' {
			e.scratch = append(e.scratch, '\n')
		}
	} else {
		e.scratch = e.text.SelectedText(e.scratch)
	}

	if text := string(e.scratch); text != "" {
		gtx.Execute(clipboard.WriteCmd{Type: "application/text", Data: io.NopCloser(strings.NewReader(text))})
		if k.Name == "X" && e.mode != ModeReadOnly {
			if !lineOp {
				if e.Delete(1) != 0 {
					return ChangeEvent{}
				}
			} else {
				if e.DeleteLine() != 0 {
					return ChangeEvent{}
				}
			}
		}
	}

	return nil
}

// onTab handles tab key event. If there is no selection of lines, intert a tab character
// at position of the cursor, else indent or unindent the selected lines, depending on if
// the event contains the shift modifier.
func (e *Editor) onTab(k key.Event) EditorEvent {
	if e.mode == ModeReadOnly {
		return nil
	}

	shiftPressed := k.Modifiers.Contain(key.ModShift)

	if e.mode == ModeSnippet {
		if shiftPressed {
			e.snippetCtx.PrevTabStop()
		} else {
			e.snippetCtx.NextTabStop()
		}
		return nil
	}

	if e.text.IndentLines(shiftPressed) > 0 {
		// Reset xoff.
		e.text.MoveCaret(0, 0)
		e.scrollCaret = true
		return ChangeEvent{}
	}

	return nil

}

func (e *Editor) onTextInput(ke key.EditEvent) {
	if e.mode == ModeReadOnly || len(ke.Text) <= 0 {
		return
	}

	// Handle column editing mode
	if e.ColumnEditEnabled() && len(e.columnEdit.selections) > 0 {
		println("[ColumnEdit] onTextInput - Column edit mode active, inserting:", ke.Text, "into", len(e.columnEdit.selections), "positions")
		e.onColumnEditInput(ke)
		return
	}

	if e.autoInsertions == nil {
		e.autoInsertions = make(map[int]rune)
	}

	// check if the input character is a bracket or a quote.
	r := []rune(ke.Text)[0]
	counterpart, isOpening := e.text.BracketsQuotes.GetCounterpart(r)

	if counterpart > 0 && isOpening {
		// Assume we will auto-insert by default.
		shouldAutoInsert := true

		if counterpart != r {
			// only check the next char.
			if e.isNearWordChar(ke.Range.Start, false) {
				shouldAutoInsert = false
			}
		} else {
			// check both the previous and next char.
			if e.isNearWordChar(ke.Range.Start, true) || e.isNearWordChar(ke.Range.Start, false) {
				shouldAutoInsert = false
			}
		}

		replaced := ke.Text
		if shouldAutoInsert {
			replaced += string(counterpart)
		}

		e.replace(ke.Range.Start, ke.Range.End, replaced)
		if shouldAutoInsert {
			e.text.MoveCaret(-1, -1)
			start, _ := e.text.Selection() // start and end should be the same
			e.autoInsertions[start] = counterpart
		} else {
			// If only the opening char was inserted, ensure it's not tracked
			delete(e.autoInsertions, ke.Range.Start)
		}

	} else if counterpart > 0 {
		// The input character is a bracket or a quote, but it is a closing part.
		//
		// check if we can just move the cursor to the next position
		// if the input is a just inserted closing part.
		nextRune, err := e.text.ReadRuneAt(ke.Range.Start)

		if err == nil && nextRune == e.autoInsertions[ke.Range.Start] {
			e.text.MoveCaret(1, 1)
			delete(e.autoInsertions, ke.Range.Start)
		} else {
			e.replace(ke.Range.Start, ke.Range.End, ke.Text)
		}
	} else {
		delete(e.autoInsertions, ke.Range.Start)
		e.replace(ke.Range.Start, ke.Range.End, ke.Text)
	}

	e.scrollCaret = true
	e.scroller.Stop()
	// Reset caret xoff.
	e.text.MoveCaret(0, 0)
	// record lastInput for auto-complete.
	e.lastInput = &ke

	// If there is an ongoing snippet context, check if the edit is inside of
	// a tabstop.
	finalStart, finalEnd := e.Selection()
	e.snippetCtx.OnInsertAt(finalStart, finalEnd)

}

func (e *Editor) isNearWordChar(runeOff int, backward bool) bool {
	pos := runeOff
	if backward {
		pos = runeOff - 1
	}

	nearbyChar, err := e.text.ReadRuneAt(pos)
	if err == nil {
		return unicode.IsLetter(nearbyChar) || unicode.IsDigit(nearbyChar) || nearbyChar == '_'
	}

	return false

}

func (e *Editor) cancelCompletor() {
	if e.completor == nil {
		return
	}

	e.completor.Cancel()
}

func (e *Editor) currentCompletionCtx() CompletionContext {
	_, end := e.text.Selection()
	input := ""
	if e.lastInput != nil && e.lastInput.Range.End+utf8.RuneCountInString(e.lastInput.Text) == end {
		input = e.lastInput.Text
	}

	ctx := CompletionContext{Input: input}
	ctx.Position.Line, ctx.Position.Column = e.text.CaretPos()
	// scroll off will change after we update the position, so we use doc
	// view position instead of viewport position.
	ctx.Coords = e.text.CaretCoords().Round().Add(e.text.ScrollOff())
	ctx.Position.Runes = end
	e.lastInput = nil
	return ctx
}

// GetCompletionContext returns a context from the current caret position.
// This is usually used in the condition of a key triggered completion.
func (e *Editor) GetCompletionContext() CompletionContext {
	return e.currentCompletionCtx()
}

// OnTextEdit should be called after normal keyboard input to update the
// auto completion engine, usually when received a ChangeEvent.
func (e *Editor) OnTextEdit() {
	if e.completor == nil {
		return
	}

	ctx := e.currentCompletionCtx()
	if ctx == (CompletionContext{}) {
		return
	}

	e.completor.OnText(ctx)
}

func (e *Editor) onPasteEvent(ke transfer.DataEvent) EditorEvent {
	if e.mode == ModeReadOnly {
		return nil
	}

	e.scrollCaret = true
	e.scroller.Stop()
	content, err := io.ReadAll(ke.Open())
	if err != nil {
		return nil
	}

	text := string(content)
	if e.onPaste != nil {
		text = e.onPaste(text)
	}

	runes := 0
	if isSingleLine(text) {
		runes = e.InsertLine(text)
	} else {
		runes = e.Insert(text)
	}

	if runes != 0 {
		return ChangeEvent{}
	}

	return nil
}

func (e *Editor) onInsertLineBreak(ke key.Event) EditorEvent {
	if e.mode == ModeReadOnly {
		return nil
	}

	e.text.IndentOnBreak("\n")
	// Reset xoff.
	e.scrollCaret = true
	e.scroller.Stop()
	e.text.MoveCaret(0, 0)
	return ChangeEvent{}
}

// onDeleteBackward update the selection when we are deleting the indentation, or
// an auto inserted bracket/quote pair.
func (e *Editor) onDeleteBackward() {
	start, end := e.Selection()
	if start != end || start <= 0 {
		return
	}

	prev, err := e.text.ReadRuneAt(start - 1)
	if err != nil && err != io.EOF {
		panic("Read rune panic: " + err.Error())
	}

	space := ' '
	// When the leading of the line are spaces and tabs, delete up to the
	// number of tab width spaces before the cursor.
	if prev == space {
		// Find the current paragraph.
		var lineStart int
		e.scratch, lineStart, _ = e.text.SelectedLineText(e.scratch)
		leading := []rune(string(e.scratch))[:end-lineStart]
		hasNonSpaceOrTab := strings.ContainsFunc(string(leading), func(r rune) bool {
			return r != space && r != '\t'
		})
		if hasNonSpaceOrTab {
			return
		}

		moves := 0
		for i := len(leading) - 1; i >= 0; i-- {
			if leading[i] == space && moves < e.text.TabWidth {
				moves++
			} else {
				break
			}
		}
		if moves > 0 {
			e.text.MoveCaret(0, -moves)
		}

	} else {
		// when there is rencently auto-inserted brackets or quotes,
		// delete the auto inserted character and the previous character.
		if inserted, exists := e.autoInsertions[start]; exists {
			defer delete(e.autoInsertions, start)
			counterpart, isOpening := e.text.BracketsQuotes.GetCounterpart(inserted)
			if !isOpening && counterpart > 0 || inserted == counterpart {
				if prev == counterpart {
					e.text.MoveCaret(-1, 1)
				}
			}
		}
	}

}

// startColumnSelection initiates column selection mode from the given position
func (e *Editor) startColumnSelection(gtx layout.Context, pos image.Point) {
	log.Println("[ColumnEdit] startColumnSelection called at pos:", pos)
	e.blinkStart = gtx.Now
	e.SetColumnEditMode(true)
	println("[ColumnEdit] Column edit mode enabled, mode is now:", e.mode)

	// Store the anchor position
	e.columnEdit.anchor = pos
	log.Println("[ColumnEdit] Anchor set to:", pos)

	// Get the line and column at the anchor position
	line, col, runeOff := e.text.QueryPos(pos)
	println("[ColumnEdit] Queried position - line:", line, "col:", col, "runeOff:", runeOff)

	if runeOff >= 0 {
		e.columnEdit.selections = []columnCursor{
			{
				line:   line,
				col:    col,
				startX: pos.X,
				endX:   pos.X,
			},
		}
		e.scrollCaret = true
		println("[ColumnEdit] Created initial column selection for line:", line, "col:", col)
	}

	gtx.Execute(key.FocusCmd{Tag: e})
	if e.completor != nil {
		e.completor.Cancel()
	}
	println("[ColumnEdit] startColumnSelection completed")
}

// updateColumnSelection updates the column selection based on current mouse position
func (e *Editor) updateColumnSelection(gtx layout.Context, pos image.Point) {
	e.blinkStart = gtx.Now

	if len(e.columnEdit.selections) == 0 {
		println("[ColumnEdit] updateColumnSelection called but no selections exist")
		return
	}

	anchor := e.columnEdit.anchor
	log.Println("[ColumnEdit] updateColumnSelection - anchor:", anchor, "current:", pos)

	// Determine the selection range in screen coordinates
	startX := min(anchor.X, pos.X)
	endX := max(anchor.X, pos.X)
	startY := min(anchor.Y, pos.Y)
	endY := max(anchor.Y, pos.Y)
	println("[ColumnEdit] Selection range - X:", startX, "to", endX, "Y:", startY, "to", endY)

	// Clear current selections
	e.columnEdit.selections = nil

	// Get line height and scroll offset
	// fixed.Int26_6 needs to be rounded to get integer pixels
	lineHeight := e.text.GetLineHeight().Round()
	scrollOff := e.text.ScrollOff()
	log.Println("[ColumnEdit] lineHeight:", lineHeight, "scrollOff:", scrollOff)

	// Calculate line numbers from screen Y coordinates
	// Screen Y = LineNum * lineHeight - scrollOff.Y
	// LineNum = (ScreenY + scrollOff.Y) / lineHeight
	startLine := (startY + scrollOff.Y) / lineHeight
	endLine := (endY + scrollOff.Y) / lineHeight
	println("[ColumnEdit] Line range:", startLine, "to", endLine)

	selectionCount := 0
	totalLines := e.text.Paragraphs()

	// Iterate through each line in the range
	for lineNum := startLine; lineNum <= endLine; lineNum++ {
		// Skip lines beyond document bounds
		if lineNum < 0 || lineNum >= totalLines {
			continue
		}

		// Calculate screen Y for this line
		screenY := lineNum*lineHeight - scrollOff.Y

		// Query the start and end column positions on this line
		startPos := image.Point{X: startX, Y: screenY}
		endPos := image.Point{X: endX, Y: screenY}

		// Get column at start X position
		_, startCol, startOff := e.text.QueryPos(startPos)
		// Get column at end X position (unused but kept for clarity)
		_, _, endOff := e.text.QueryPos(endPos)

		// Ensure we have valid positions
		if startOff < 0 || endOff < 0 {
			// If QueryPos failed, try to use line-based calculation
			if startOff < 0 {
				startCol = 0
			}
			if endOff < 0 {
				// No action needed since endCol is not used
			}
		}

		// Use the smaller column for consistency
		col := startCol

		e.columnEdit.selections = append(e.columnEdit.selections, columnCursor{
			line:   lineNum,
			col:    col,
			startX: startX,
			endX:   endX,
		})
		selectionCount++
	}

	println("[ColumnEdit] Created", selectionCount, "column selections")
	e.scrollCaret = true
}

// onColumnEditInput handles text input in column editing mode
func (e *Editor) onColumnEditInput(ke key.EditEvent) {
	if len(e.columnEdit.selections) == 0 {
		println("[ColumnEdit] onColumnEditInput called but no selections exist")
		return
	}

	textToInsert := ke.Text
	println("[ColumnEdit] onColumnEditInput - inserting:", textToInsert, "into", len(e.columnEdit.selections), "cursor positions")

	// Group operations for undo
	e.buffer.GroupOp()

	// Insert text at each column cursor position
	for i := range e.columnEdit.selections {
		cursor := &e.columnEdit.selections[i]

		// Calculate the rune offset for this position
		runeOff, _ := e.ConvertPos(cursor.line, cursor.col)
		println("[ColumnEdit] Inserting at line:", cursor.line, "col:", cursor.col, "runeOff:", runeOff)

		// Insert the text at this position
		e.replace(runeOff, runeOff, textToInsert)

		// Update cursor position after insertion
		cursor.col += utf8.RuneCountInString(textToInsert)
	}

	e.buffer.UnGroupOp()
	println("[ColumnEdit] onColumnEditInput completed")

	e.scrollCaret = true
	e.scroller.Stop()
	e.text.MoveCaret(0, 0)
	e.lastInput = &ke
}
