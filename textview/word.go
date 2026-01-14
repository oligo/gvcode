package textview

import (
	"slices"
	"strings"
	"unicode"
)

const (
	// defaultWordSeperators defines a default set of word seperators. It is
	// used when no custom word seperators are set.
	defaultWordSeperators = "`~!@#$%^&*()-=+[{]}\\|;:'\",.<>/?"
)

// IsWordSeperator check r to see if it is a word seperator. A word seperator
// set the boundary when navigating by words, or deleting by words.
// TODO: does it make sence to use unicode space definition here?
func (e *TextView) IsWordSeperator(r rune) bool {
	seperators := e.WordSeperators

	if e.WordSeperators == "" {
		seperators = defaultWordSeperators
	}

	return strings.ContainsRune(seperators, r) || unicode.IsSpace(r)

}

// MoveWord moves the caret to the next few words in the specified direction.
// Positive is forward, negative is backward.
// The final caret position will be aligned to a grapheme cluster boundary.
func (e *TextView) MoveWords(distance int, selAct SelectionAction) {
	// split the distance information into constituent parts to be
	// used independently.
	words, direction := distance, 1
	if distance < 0 {
		words, direction = distance*-1, -1
	}
	// atEnd if caret is at either side of the buffer.
	caret := e.closestToRune(e.caret.start)
	atEnd := func() bool {
		return caret.Runes == 0 || caret.Runes == e.Len()
	}
	// next returns the appropriate rune given the direction.
	next := func() (r rune) {
		if direction < 0 {
			r, _ = e.src.ReadRuneAt(caret.Runes - 1)
		} else {
			r, _ = e.src.ReadRuneAt(caret.Runes)
		}
		return r
	}
	for ii := 0; ii < words; ii++ {
		for r := next(); e.IsWordSeperator(r) && !atEnd(); r = next() {
			e.MoveCaret(direction, 0)
			caret = e.closestToRune(e.caret.start)
		}
		e.MoveCaret(direction, 0)
		caret = e.closestToRune(e.caret.start)
		for r := next(); !e.IsWordSeperator(r) && !atEnd(); r = next() {
			e.MoveCaret(direction, 0)
			caret = e.closestToRune(e.caret.start)
		}
	}
	e.updateSelection(selAct)
	e.clampCursorToGraphemes()
}

// readBySeperator reads in the specified direction from caretOff until the seperator returns false.
// It returns the read text.
func (e *TextView) readBySeperator(direction int, caretOff int, seperator func(r rune) bool) []rune {
	buf := make([]rune, 0)
	for {
		if caretOff < 0 || caretOff > e.src.Len() {
			break
		}

		r, err := e.src.ReadRuneAt(caretOff)
		if seperator(r) || err != nil {
			break
		}

		if direction < 0 {
			buf = slices.Insert(buf, 0, r)
			caretOff--
		} else {
			buf = append(buf, r)
			caretOff++
		}
	}

	return buf
}

// ReadWord tries to read one word nearby the caret, returning the word if there's one,
// and the offset of the caret in the word.
//
// The word boundary is checked using the word boundary characters or just spaces.
func (e *TextView) ReadWord(bySpace bool) (string, int) {
	caret := max(e.caret.start, e.caret.end)
	buf := make([]rune, 0)

	seperator := func(r rune) bool {
		if bySpace {
			return unicode.IsSpace(r)
		}
		return e.IsWordSeperator(r)
	}

	left := e.readBySeperator(-1, caret-1, seperator)
	buf = append(buf, left...)
	right := e.readBySeperator(1, caret, seperator)
	buf = append(buf, right...)

	return string(buf), len(left)
}

// WordBoundariesAt returns the start and end rune offsets of the word at the given caret position.
// If caret is on a word separator, start and end will both equal caret (empty word).
// The bySpace parameter controls whether only spaces are considered separators (true) or
// custom word separators are used (false).
func (e *TextView) WordBoundariesAt(caret int, bySpace bool) (start, end int) {
	separator := func(r rune) bool {
		if bySpace {
			return unicode.IsSpace(r)
		}
		return e.IsWordSeperator(r)
	}

	// Read leftwards from caret-1
	left := e.readBySeperator(-1, caret-1, separator)
	// Read rightwards from caret
	right := e.readBySeperator(1, caret, separator)

	start = caret - len(left)
	end = caret + len(right)
	return start, end
}

// FindAllWordOccurrences returns the start and end rune offsets of all occurrences of the word
// spanning from start to end (exclusive). The bySpace parameter controls whether only spaces
// are considered separators (true) or custom word separators are used (false).
// This implementation scans the document once with O(n) complexity.
func (e *TextView) FindAllWordOccurrences(start, end int, bySpace bool) [][2]int {
	if start >= end {
		return nil
	}
	wordLen := end - start
	// Read the target word runes for comparison
	targetWord := make([]rune, wordLen)
	for i := 0; i < wordLen; i++ {
		r, err := e.src.ReadRuneAt(start + i)
		if err != nil {
			// Should not happen if start/end are valid, but bail out
			return nil
		}
		targetWord[i] = r
	}

	separator := func(r rune) bool {
		if bySpace {
			return unicode.IsSpace(r)
		}
		return e.IsWordSeperator(r)
	}

	var occurrences [][2]int
	totalLen := e.src.Len()

	// Helper to compare rune slices
	runeSliceEqual := func(a, b []rune) bool {
		if len(a) != len(b) {
			return false
		}
		for i := range a {
			if a[i] != b[i] {
				return false
			}
		}
		return true
	}

	i := 0
	for i < totalLen {
		// Skip separators
		for i < totalLen && separator(e.peekRune(i)) {
			i++
		}
		if i >= totalLen {
			break
		}

		wordStart := i
		// Collect runes for current word
		wordBuf := make([]rune, 0, wordLen)
		for i < totalLen && !separator(e.peekRune(i)) {
			r, _ := e.src.ReadRuneAt(i)
			wordBuf = append(wordBuf, r)
			i++
			// Early exit if word already longer than target
			if len(wordBuf) > wordLen {
				break
			}
		}

		// Check if this word matches the target
		if len(wordBuf) == wordLen && runeSliceEqual(wordBuf, targetWord) {
			occurrences = append(occurrences, [2]int{wordStart, wordStart + wordLen})
		}
		// i is already positioned at separator or end, loop continues
	}

	return occurrences
}

// peekRune safely reads a rune at offset, returning 0 on error.
func (e *TextView) peekRune(offset int) rune {
	r, _ := e.src.ReadRuneAt(offset)
	return r
}

// ReadUntil reads in the specified direction from the current caret position until the
// seperator returns false. It returns the read text.
func (e *TextView) ReadUntil(direction int, seperator func(r rune) bool) string {
	caret := max(e.caret.start, e.caret.end)
	var buf []rune

	if direction <= 0 {
		buf = e.readBySeperator(direction, caret-1, seperator)
	} else {
		buf = e.readBySeperator(1, caret, seperator)
	}

	return string(buf)
}
