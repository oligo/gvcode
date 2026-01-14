package gvcode

import (
	stdColor "image/color"

	"github.com/oligo/gvcode/color"
	"github.com/oligo/gvcode/textstyle/decoration"
)

const wordHighlightSource = "_word_highlight"

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
