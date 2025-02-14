package editor

import (
	"image"
	"math"
	"unicode"
	"unicode/utf8"

	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"
	"github.com/oligo/gvcode/buffer"
	"golang.org/x/exp/slices"
	"golang.org/x/image/math/fixed"
)

// TextRange contains the range of text of interest in the document. It can used for
// search, styling text, or any other purposes.
type TextRange struct {
	// offset of the start rune in the document.
	Start int
	// offset of the end rune in the document.
	End int
}

// TextStyle defines style for a range of text in the document.
type TextStyle struct {
	TextRange
	// Color of the text..
	Color op.CallOp
	// Background color of the painted text in the range.
	Background op.CallOp
}

type caretPos struct {
	// xoff is the offset to the current position when moving between lines.
	xoff fixed.Int26_6
	// start is the current caret position in runes, and also the start position of
	// selected text. end is the end position of selected text. If start
	// == end, then there's no selection. Note that it's possible (and
	// common) that the caret (start) is after the end, e.g. after
	// Shift-DownArrow.
	start int
	end   int
}

// textView provides efficient shaping and indexing of interactive text. When provided
// with a TextSource, textView will shape and cache the runes within that source.
// It provides methods for configuring a viewport onto the shaped text which can
// be scrolled, and for configuring and drawing text selection boxes.
type textView struct {
	Alignment text.Alignment
	// LineHeight controls the distance between the baselines of lines of text.
	// If zero, the font size will be used.
	LineHeight unit.Sp
	// LineHeightScale applies a scaling factor to the LineHeight. If zero, a default
	// value 1.2 will be used.
	LineHeightScale float32
	// WrapPolicy configures how displayed text will be broken into lines.
	WrapPolicy text.WrapPolicy
	CaretWidth unit.Dp

	// TabWidth set how many spaces to represent a tab character .
	TabWidth int

	src    buffer.TextSource
	params text.Parameters
	shaper *text.Shaper
	// dimensions of the layouted document.
	dims layout.Dimensions
	// viewport size
	viewSize image.Point
	// line height used by shaper.
	lineHeight fixed.Int26_6
	// scrolled offset relative to the start of dims.
	scrollOff image.Point

	layouter textLayout

	//index glyphIndex
	// graphemes tracks the indices of grapheme cluster boundaries within text source.
	//graphemes []int
	//seg       segmenter.Segmenter

	// // paragraphReader is used to populate graphemes.
	//paragraphReader graphemeReader

	// The layout is valid or not. Invalid layout requires a re-layout.
	valid bool
	// caret position in the view.
	caret   caretPos
	regions []Region
}

// SetSource initializes the underlying data source for the Text. This
// must be done before invoking any other methods on Text.
func (e *textView) SetSource(source buffer.TextSource) {
	e.src = source
	e.layouter = newTextLayout(e.src)
	e.invalidate()
}

func (e *textView) Changed() bool {
	return e.src.Changed()
}

// Dimensions returns the dimensions of the visible text.
func (e *textView) Dimensions() layout.Dimensions {
	basePos := e.dims.Size.Y - e.dims.Baseline
	return layout.Dimensions{Size: e.viewSize, Baseline: e.viewSize.Y - basePos}
}

// FullDimensions returns the dimensions of all shaped text, including
// text that isn't visible within the current viewport.
func (e *textView) FullDimensions() layout.Dimensions {
	return e.dims
}

func (e *textView) makeValid() {
	if e.valid {
		return
	}
	e.layoutText(e.shaper)
	e.valid = true
}

func (e *textView) closestToRune(runeIdx int) combinedPos {
	e.makeValid()
	pos, _ := e.layouter.closestToRune(runeIdx)
	return pos
}

func (e *textView) closestToLineCol(line, col int) combinedPos {
	e.makeValid()
	return e.layouter.closestToLineCol(screenPos{line: line, col: col})
}

func (e *textView) closestToXY(x fixed.Int26_6, y int) combinedPos {
	e.makeValid()
	return e.layouter.closestToXY(x, y)
}

func (e *textView) closestToXYGraphemes(x fixed.Int26_6, y int) combinedPos {
	// Find the closest existing rune position to the provided coordinates.
	pos := e.closestToXY(x, y)
	// Resolve cluster boundaries on either side of the rune position.
	firstOption := e.moveByGraphemes(pos.runes, 0)
	distance := 1
	if firstOption > pos.runes {
		distance = -1
	}
	secondOption := e.moveByGraphemes(firstOption, distance)
	// Choose the closest grapheme cluster boundary to the desired point.
	first := e.closestToRune(firstOption)
	firstDist := absFixed(first.x - x)
	second := e.closestToRune(secondOption)
	secondDist := absFixed(second.x - x)
	if firstDist > secondDist {
		return second
	} else {
		return first
	}
}

// MaxLines moves the cursor the specified number of lines vertically, ensuring
// that the resulting position is aligned to a grapheme cluster.
func (e *textView) MoveLines(distance int, selAct selectionAction) {
	caretStart := e.closestToRune(e.caret.start)
	x := caretStart.x + e.caret.xoff
	// Seek to line.
	pos := e.closestToLineCol(caretStart.lineCol.line+distance, 0)
	pos = e.closestToXYGraphemes(x, pos.y)
	e.caret.start = pos.runes
	e.caret.xoff = x - pos.x
	e.updateSelection(selAct)
}

// Layout the text, reshaping it as necessary.
func (e *textView) Layout(gtx layout.Context, lt *text.Shaper, font font.Font, size unit.Sp) {
	e.params.DisableSpaceTrim = true

	if e.params.Locale != gtx.Locale {
		e.params.Locale = gtx.Locale
		e.invalidate()
	}
	textSize := fixed.I(gtx.Sp(size))
	if e.params.Font != font || e.params.PxPerEm != textSize {
		e.invalidate()
		e.params.Font = font
		e.params.PxPerEm = textSize
	}
	maxWidth := gtx.Constraints.Max.X

	minWidth := gtx.Constraints.Min.X
	if maxWidth != e.params.MaxWidth {
		e.params.MaxWidth = maxWidth
		e.invalidate()
	}
	if minWidth != e.params.MinWidth {
		e.params.MinWidth = minWidth
		e.invalidate()
	}
	if lt != e.shaper {
		e.shaper = lt
		e.invalidate()
	}
	if e.Alignment != e.params.Alignment {
		e.params.Alignment = e.Alignment
		e.invalidate()
	}

	if e.WrapPolicy != e.params.WrapPolicy {
		e.params.WrapPolicy = e.WrapPolicy
		e.invalidate()
	}
	if lh := fixed.I(gtx.Sp(e.LineHeight)); lh != e.params.LineHeight {
		e.params.LineHeight = lh
		e.invalidate()
	}
	if e.LineHeightScale != e.params.LineHeightScale {
		e.params.LineHeightScale = e.LineHeightScale
		e.invalidate()
	}

	// calculate the final line height used by Shaper
	e.lineHeight = e.calcLineHeight()
	e.makeValid()

	if viewSize := e.calculateViewSize(gtx); viewSize != e.viewSize {
		e.viewSize = viewSize
		e.invalidate()
	}
	e.makeValid()
}

// Calculate line height. Maybe there's a better way?
func (tv *textView) calcLineHeight() fixed.Int26_6 {
	lineHeight := tv.params.LineHeight
	// align with how text.Shaper handles default value of tv.params.LineHeight.
	if lineHeight == 0 {
		lineHeight = tv.params.PxPerEm
	}
	lineHeightScale := tv.params.LineHeightScale
	// align with how text.Shaper handles default value of tv.params.LineHeightScale.
	if lineHeightScale == 0 {
		lineHeightScale = 1.2
	}

	return floatToFixed(fixedToFloat(lineHeight) * lineHeightScale)
}

// ByteOffset returns the start byte of the rune at the given
// rune offset, clamped to the size of the text.
func (e *textView) ByteOffset(runeOffset int) int64 {
	pos := e.closestToRune(runeOffset)
	return int64(e.src.RuneOffset(pos.runes))
}

// Len is the length of the editor contents, in runes.
func (e *textView) Len() int {
	e.makeValid()
	return e.closestToRune(math.MaxInt).runes
}

func (e *textView) ScrollBounds() image.Rectangle {
	return image.Rectangle{Max: image.Point{Y: e.dims.Size.Y - e.viewSize.Y}}
}

func (e *textView) ScrollRel(dx, dy int) {
	e.scrollAbs(e.scrollOff.X+dx, e.scrollOff.Y+dy)
}

// ScrollOff returns the scroll offset of the text viewport.
func (e *textView) ScrollOff() image.Point {
	return e.scrollOff
}

func (e *textView) scrollAbs(x, y int) {
	e.scrollOff.X = x
	e.scrollOff.Y = y
	b := e.ScrollBounds()
	if e.scrollOff.X > b.Max.X {
		e.scrollOff.X = b.Max.X
	}
	if e.scrollOff.X < b.Min.X {
		e.scrollOff.X = b.Min.X
	}
	if e.scrollOff.Y > b.Max.Y {
		e.scrollOff.Y = b.Max.Y
	}
	if e.scrollOff.Y < b.Min.Y {
		e.scrollOff.Y = b.Min.Y
	}
}

// MoveCoord moves the caret to the position closest to the provided
// point that is aligned to a grapheme cluster boundary.
func (e *textView) MoveCoord(pos image.Point) {
	x := fixed.I(pos.X + e.scrollOff.X)
	y := pos.Y + e.scrollOff.Y
	e.caret.start = e.closestToXYGraphemes(x, y).runes
	e.caret.xoff = 0
}

// CaretPos returns the line & column numbers of the caret.
func (e *textView) CaretPos() (line, col int) {
	pos := e.closestToRune(e.caret.start)
	return pos.lineCol.line, pos.lineCol.col
}

// CaretCoords returns the coordinates of the caret, relative to the
// editor itself.
func (e *textView) CaretCoords() f32.Point {
	pos := e.closestToRune(e.caret.start)
	return f32.Pt(float32(pos.x)/64-float32(e.scrollOff.X), float32(pos.y-e.scrollOff.Y))
}

// invalidate mark the layout as invalid.
func (e *textView) invalidate() {
	e.valid = false
}

// Set the text of the buffer. It returns the number of runes inserted.
func (e *textView) SetText(s string) int {
	e.src.SetText([]byte(s))
	sc := e.src.Len()

	// e.SetCaret(0, 0)
	e.invalidate()
	return sc
}

// Replace the text between start and end with s. Indices are in runes.
// It returns the number of runes inserted.
func (e *textView) Replace(start, end int, s string) int {
	if start > end {
		start, end = end, start
	}
	startPos := e.closestToRune(start)
	endPos := e.closestToRune(end)
	startOff := startPos.runes
	sc := utf8.RuneCountInString(s)
	newEnd := startPos.runes + sc

	e.src.Replace(startOff, endPos.runes, s)
	adjust := func(pos int) int {
		switch {
		case newEnd < pos && pos <= endPos.runes:
			pos = newEnd
		case endPos.runes < pos:
			diff := newEnd - endPos.runes
			pos = pos + diff
		}
		return pos
	}
	e.caret.start = adjust(e.caret.start)
	e.caret.end = adjust(e.caret.end)
	e.invalidate()
	return sc
}

// MovePages moves the caret position by vertical pages of text, ensuring that
// the final position is aligned to a grapheme cluster boundary.
func (e *textView) MovePages(pages int, selAct selectionAction) {
	caret := e.closestToRune(e.caret.start)
	x := caret.x + e.caret.xoff
	y := caret.y + pages*e.viewSize.Y
	pos := e.closestToXYGraphemes(x, y)
	e.caret.start = pos.runes
	e.caret.xoff = x - pos.x
	e.updateSelection(selAct)
}

// moveByGraphemes returns the rune index resulting from moving the
// specified number of grapheme clusters from startRuneidx.
func (e *textView) moveByGraphemes(startRuneidx, graphemes int) int {
	if len(e.layouter.graphemes) == 0 {
		return startRuneidx
	}
	startGraphemeIdx, _ := slices.BinarySearch(e.layouter.graphemes, startRuneidx)
	startGraphemeIdx = max(startGraphemeIdx+graphemes, 0)
	startGraphemeIdx = min(startGraphemeIdx, len(e.layouter.graphemes)-1)
	startRuneIdx := e.layouter.graphemes[startGraphemeIdx]
	return e.closestToRune(startRuneIdx).runes
}

// clampCursorToGraphemes ensures that the final start/end positions of
// the cursor are on grapheme cluster boundaries.
func (e *textView) clampCursorToGraphemes() {
	e.caret.start = e.moveByGraphemes(e.caret.start, 0)
	e.caret.end = e.moveByGraphemes(e.caret.end, 0)
}

// MoveCaret moves the caret (aka selection start) and the selection end
// relative to their current positions. Positive distances moves forward,
// negative distances moves backward. Distances are in grapheme clusters which
// better match the expectations of users than runes.
func (e *textView) MoveCaret(startDelta, endDelta int) {
	e.caret.xoff = 0
	e.caret.start = e.moveByGraphemes(e.caret.start, startDelta)
	e.caret.end = e.moveByGraphemes(e.caret.end, endDelta)
}

// MoveTextStart moves the caret to the start of the text.
func (e *textView) MoveTextStart(selAct selectionAction) {
	caret := e.closestToRune(e.caret.end)
	e.caret.start = 0
	e.caret.end = caret.runes
	e.caret.xoff = -caret.x
	e.updateSelection(selAct)
	e.clampCursorToGraphemes()
}

// MoveTextEnd moves the caret to the end of the text.
func (e *textView) MoveTextEnd(selAct selectionAction) {
	caret := e.closestToRune(math.MaxInt)
	e.caret.start = caret.runes
	e.caret.xoff = fixed.I(e.params.MaxWidth) - caret.x
	e.updateSelection(selAct)
	e.clampCursorToGraphemes()
}

// MoveLineStart moves the caret to the start of the current line, ensuring that the resulting
// cursor position is on a grapheme cluster boundary.
func (e *textView) MoveLineStart(selAct selectionAction) {
	caret := e.closestToRune(e.caret.start)
	caret = e.closestToLineCol(caret.lineCol.line, 0)
	e.caret.start = caret.runes
	e.caret.xoff = -caret.x
	e.updateSelection(selAct)
	e.clampCursorToGraphemes()
}

// MoveLineEnd moves the caret to the end of the current line, ensuring that the resulting
// cursor position is on a grapheme cluster boundary.
func (e *textView) MoveLineEnd(selAct selectionAction) {
	caret := e.closestToRune(e.caret.start)
	caret = e.closestToLineCol(caret.lineCol.line, math.MaxInt)
	e.caret.start = caret.runes
	e.caret.xoff = fixed.I(e.params.MaxWidth) - caret.x
	e.updateSelection(selAct)
	e.clampCursorToGraphemes()
}

// MoveWord moves the caret to the next word in the specified direction.
// Positive is forward, negative is backward.
// Absolute values greater than one will skip that many words.
// The final caret position will be aligned to a grapheme cluster boundary.
// BUG(whereswaldon): this method's definition of a "word" is currently
// whitespace-delimited. Languages that do not use whitespace to delimit
// words will experience counter-intuitive behavior when navigating by
// word.
func (e *textView) MoveWord(distance int, selAct selectionAction) {
	// split the distance information into constituent parts to be
	// used independently.
	words, direction := distance, 1
	if distance < 0 {
		words, direction = distance*-1, -1
	}
	// atEnd if caret is at either side of the buffer.
	caret := e.closestToRune(e.caret.start)
	atEnd := func() bool {
		return caret.runes == 0 || caret.runes == e.Len()
	}
	// next returns the appropriate rune given the direction.
	next := func() (r rune) {
		off := e.src.RuneOffset(caret.runes)
		if direction < 0 {
			r, _, _ = e.src.ReadRuneBeforeBytes(int64(off))
		} else {
			r, _, _ = e.src.ReadRuneAtBytes(int64(off))
		}
		return r
	}
	for ii := 0; ii < words; ii++ {
		for r := next(); unicode.IsSpace(r) && !atEnd(); r = next() {
			e.MoveCaret(direction, 0)
			caret = e.closestToRune(e.caret.start)
		}
		e.MoveCaret(direction, 0)
		caret = e.closestToRune(e.caret.start)
		for r := next(); !unicode.IsSpace(r) && !atEnd(); r = next() {
			e.MoveCaret(direction, 0)
			caret = e.closestToRune(e.caret.start)
		}
	}
	e.updateSelection(selAct)
	e.clampCursorToGraphemes()
}

func (e *textView) ScrollToCaret() {
	caret := e.closestToRune(e.caret.start)

	miny := caret.y - caret.ascent.Ceil()
	maxy := caret.y + caret.descent.Ceil()
	var dist int
	if d := miny - e.scrollOff.Y; d < 0 {
		dist = d
	} else if d := maxy - (e.scrollOff.Y + e.viewSize.Y); d > 0 {
		dist = d
	}
	e.ScrollRel(0, dist)
}

// SelectionLen returns the length of the selection, in runes; it is
// equivalent to utf8.RuneCountInString(e.SelectedText()).
func (e *textView) SelectionLen() int {
	return abs(e.caret.start - e.caret.end)
}

// Selection returns the start and end of the selection, as rune offsets.
// start can be > end.
func (e *textView) Selection() (start, end int) {
	return e.caret.start, e.caret.end
}

// SetCaret moves the caret to start, and sets the selection end to end. Then
// the two ends are clamped to the nearest grapheme cluster boundary. start
// and end are in runes, and represent offsets into the editor text.
func (e *textView) SetCaret(start, end int) {
	e.caret.start = e.closestToRune(start).runes
	e.caret.end = e.closestToRune(end).runes
	e.clampCursorToGraphemes()
}

// SelectedText returns the currently selected text (if any) from the editor,
// filling the provided byte slice if it is large enough or allocating and
// returning a new byte slice if the provided one is insufficient.
// Callers can guarantee that the buf is large enough by providing a buffer
// with capacity e.SelectionLen()*utf8.UTFMax.
func (e *textView) SelectedText(buf []byte) []byte {
	startOff := e.src.RuneOffset(e.caret.start)
	endOff := e.src.RuneOffset(e.caret.end)
	start := min(startOff, endOff)
	end := max(startOff, endOff)
	if cap(buf) < end-start {
		buf = make([]byte, end-start)
	}
	buf = buf[:end-start]
	n, _ := e.src.ReadAt(buf, int64(start))
	// There is no way to reasonably handle a read error here. We rely upon
	// implementations of textSource to provide other ways to signal errors
	// if the user cares about that, and here we use whatever data we were
	// able to read.
	return buf[:n]
}

func (e *textView) updateSelection(selAct selectionAction) {
	if selAct == selectionClear {
		e.ClearSelection()
	}
}

// ClearSelection clears the selection, by setting the selection end equal to
// the selection start.
func (e *textView) ClearSelection() {
	e.caret.end = e.caret.start
}

// Undo revert the last operation(s) and mark the textview invalid.
func (e *textView) Undo() ([]buffer.CursorPos, bool) {
	cursors, ok := e.src.Undo()
	if ok {
		e.invalidate()
	}

	return cursors, ok
}

// Redo revert the last undo operation(s) and mark the textview invalid.
func (e *textView) Redo() ([]buffer.CursorPos, bool) {
	cursors, ok := e.src.Redo()
	if ok {
		e.invalidate()
	}

	return cursors, ok
}

// Regions returns visible regions covering the rune range [start,end).
func (e *textView) Regions(start, end int, regions []Region) []Region {
	viewport := image.Rectangle{
		Min: e.scrollOff,
		Max: e.viewSize.Add(e.scrollOff),
	}
	return e.layouter.locate(viewport, start, end, regions)
}

func absFixed(i fixed.Int26_6) fixed.Int26_6 {
	if i < 0 {
		return -i
	}
	return i
}

func fixedToFloat(i fixed.Int26_6) float32 {
	return float32(i) / 64.0
}

func floatToFixed(f float32) fixed.Int26_6 {
	return fixed.Int26_6(f * 64)
}
