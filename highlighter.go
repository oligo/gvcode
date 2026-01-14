package gvcode

import (
	stdColor "image/color"

	"github.com/oligo/gvcode/color"
	"github.com/oligo/gvcode/textstyle/decoration"
)

const (
	wordHighlightSource      = "_word_highlight"
	selectionHighlightSource = "_selection_highlight"
)

// wordHighlighter finds the word at the current caret position and highlights
// all occurrences of that word in the document. If there's a text selection, no
// highlighting is performed.
//
// The highlightColor parameter sets the background color for the word occurrences.
// If highlightColor is zero, a default color will be used.
type wordHighlighter struct {
	editor       *Editor
	lastCaretPos int
	dirty        bool
}

func (wh *wordHighlighter) HighlightAtCaret(highlightColor color.Color) error {
	// Clear previous word highlights
	wh.editor.ClearDecorations(wordHighlightSource)

	// No highlighting if there's a selection
	if wh.editor.SelectionLen() > 0 {
		return nil
	}

	caretStart, _ := wh.editor.Selection()

	defer func() {
		wh.lastCaretPos = caretStart
		wh.dirty = false
	}()

	start, end := wh.editor.text.WordBoundariesAt(caretStart, false)
	if start >= end {
		// Caret is on a separator or empty document
		return nil
	}

	occurrences := wh.editor.text.FindAllWordOccurrences(start, end, false)
	if len(occurrences) == 0 {
		return nil
	}

	// Use provided color or default
	if !highlightColor.IsSet() {
		// Fallback to a light gray
		highlightColor = color.MakeColor(stdColor.NRGBA{R: 0xDD, G: 0xDD, B: 0xDD, A: 0x80})
	}

	decos := make([]decoration.Decoration, 0, len(occurrences))
	for _, occ := range occurrences {
		decos = append(decos, decoration.Decoration{
			Source: wordHighlightSource,
			Start:  occ[0],
			End:    occ[1],
			Background: &decoration.Background{
				Color: highlightColor,
			},
			Priority: 0, // use lowest priority.
		})
	}

	return wh.editor.AddDecorations(decos...)
}

func (wh *wordHighlighter) IsDirty() bool {
	caretStart, _ := wh.editor.Selection()
	return caretStart != wh.lastCaretPos || wh.dirty
}

// Clear removes all word highlight decorations.
func (wh *wordHighlighter) Clear() {
	wh.editor.ClearDecorations(wordHighlightSource)
}

// MarkDirty marks that word highlighting needs to be updated.
func (wh *wordHighlighter) MarkDirty() {
	wh.dirty = true
}

// selectionHighlighter finds the currently selected text and highlights
// all occurrences of that text in the document. If there's no text selection,
// no highlighting is performed.
//
// The highlightColor parameter sets the background color for the text occurrences.
// If highlightColor is zero, a default color will be used.
type selectionHighlighter struct {
	editor          *Editor
	lastSelection   string
	lastSelectionID int // track selection changes by comparing start/end positions
	dirty           bool
}

func (sh *selectionHighlighter) HighlightSelection(highlightColor color.Color) error {
	// Clear previous selection highlights
	sh.editor.ClearDecorations(selectionHighlightSource)

	// Compute selection ID for state tracking
	start, end := sh.editor.Selection()
	selectionID := start<<32 | end

	defer func() {
		sh.lastSelectionID = selectionID
		sh.lastSelection = sh.editor.SelectedText()
		sh.dirty = false
	}()

	// No highlighting if there's no selection
	if sh.editor.SelectionLen() == 0 {
		return nil
	}

	// Get selected text
	selectedText := sh.editor.SelectedText()
	if selectedText == "" {
		return nil
	}

	occurrences := sh.editor.text.FindAllTextOccurrences(start, end)
	if len(occurrences) == 0 {
		return nil
	}

	// Use provided color or default
	if !highlightColor.IsSet() {
		// Default to a lighter version of selection color
		selectColor := sh.editor.colorPalette.SelectColor
		if selectColor.IsSet() {
			// Use 30% alpha (lighter than selection's default 60%)
			highlightColor = selectColor.MulAlpha(0x30)
		} else {
			// Fallback to a light gray with some transparency
			highlightColor = color.MakeColor(stdColor.NRGBA{R: 0xDD, G: 0xDD, B: 0xDD, A: 0x60})
		}
	}

	decos := make([]decoration.Decoration, 0, len(occurrences))
	for _, occ := range occurrences {
		decos = append(decos, decoration.Decoration{
			Source: selectionHighlightSource,
			Start:  occ[0],
			End:    occ[1],
			Background: &decoration.Background{
				Color: highlightColor,
			},
			Priority: 0, // use lowest priority
		})
	}

	return sh.editor.AddDecorations(decos...)
}

func (sh *selectionHighlighter) IsDirty() bool {
	// Check if selection changed (text or position)
	start, end := sh.editor.Selection()
	selectionID := start<<32 | end // simple hash of selection positions
	if selectionID != sh.lastSelectionID {
		return true
	}
	return sh.dirty
}

// Clear removes all selection highlight decorations.
func (sh *selectionHighlighter) Clear() {
	sh.editor.ClearDecorations(selectionHighlightSource)
}

// MarkDirty marks that selection highlighting needs to be updated.
func (sh *selectionHighlighter) MarkDirty() {
	sh.dirty = true
}

// UpdateLastState updates the internal state after highlighting.
// Should be called after successful highlighting.
func (sh *selectionHighlighter) UpdateLastState() {
	start, end := sh.editor.Selection()
	sh.lastSelectionID = start<<32 | end
	sh.lastSelection = sh.editor.SelectedText()
	sh.dirty = false
}