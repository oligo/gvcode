package layout

import (
	"fmt"
	"image"
	"iter"

	"gioui.org/text"
	"golang.org/x/image/math/fixed"
)

// Line contains various metrics of a line of text.
type Line struct {
	XOff    fixed.Int26_6
	YOff    int
	Width   fixed.Int26_6
	Ascent  fixed.Int26_6
	Descent fixed.Int26_6
	Glyphs  []*text.Glyph
	// runes is the number of runes represented by this line.
	Runes int
	// runeOff tracks the rune offset of the first rune of the line in the document.
	RuneOff int
}

func (li Line) String() string {
	return fmt.Sprintf("[line] xOff: %d, yOff: %d, width: %d, runes: %d, runeOff: %d, glyphs: %d",
		li.XOff.Round(), li.YOff, li.Width.Ceil(), li.Runes, li.RuneOff, len(li.Glyphs))
}

func (li *Line) append(glyphs ...text.Glyph) {
	for _, gl := range glyphs {
		li.YOff = int(gl.Y)
		if li.XOff > gl.X {
			li.XOff = gl.X
		}

		li.Width += gl.Advance
		// glyph ascent and descent are derived from the line ascent and descent,
		// so it is safe to just set them as the line's ascent and descent.
		li.Ascent = gl.Ascent
		li.Descent = gl.Descent
		li.Runes += int(gl.Runes)
		li.Glyphs = append(li.Glyphs, &gl)
	}

}

// recompute re-computes X position for Bidi text by processing runs of direction.
func (li *Line) recompute(alignOff fixed.Int26_6, runeOff int) {
	if len(li.Glyphs) == 0 {
		li.RuneOff = runeOff
		return
	}

	// Tracks the start X of the current run relative to the line start
	xOff := fixed.I(0)
	// Index of the first glyph in the current run
	runStart := 0

	for i := 0; i <= len(li.Glyphs); i++ {
		// Determine if the current run has ended (end of line or direction change)
		endOfRun := false
		if i == len(li.Glyphs) {
			endOfRun = true
		} else {
			// Check if direction changes compared to the start of the run
			// Gio uses FlagTowardOrigin to indicate RTL direction
			currentDir := li.Glyphs[i].Flags & text.FlagTowardOrigin
			startDir := li.Glyphs[runStart].Flags & text.FlagTowardOrigin
			if currentDir != startDir {
				endOfRun = true
			}
		}

		if endOfRun {
			// Calculate the total width of this specific run
			runWidth := fixed.I(0)
			for j := runStart; j < i; j++ {
				runWidth += li.Glyphs[j].Advance
			}

			// Layout the glyphs within this run based on direction
			isRTL := (li.Glyphs[runStart].Flags & text.FlagTowardOrigin) == text.FlagTowardOrigin

			if isRTL {
				// RTL Run: Layout Right-to-Left (assuming Logical Order input)
				// The run occupies space from [xOff] to [xOff + runWidth]
				// We start the cursor at the RIGHT edge and subtract advances.
				cursor := alignOff + xOff + runWidth
				for j := runStart; j < i; j++ {
					cursor -= li.Glyphs[j].Advance
					li.Glyphs[j].X = cursor

					// Ensure the last glyph in the line gets the break flag
					if j == len(li.Glyphs)-1 {
						li.Glyphs[j].Flags |= text.FlagLineBreak
					}
				}
			} else {
				// LTR Run: Layout Left-to-Right
				// We start the cursor at the LEFT edge (xOff) and add advances.
				cursor := alignOff + xOff
				for j := runStart; j < i; j++ {
					li.Glyphs[j].X = cursor
					cursor += li.Glyphs[j].Advance

					if j == len(li.Glyphs)-1 {
						li.Glyphs[j].Flags |= text.FlagLineBreak
					}
				}
			}

			// Advance the global line offset by the run's width
			xOff += runWidth
			runStart = i
		}
	}

	li.RuneOff = runeOff
}

func (li *Line) adjustYOff(yOff int) {
	li.YOff = yOff
	for _, gl := range li.Glyphs {
		gl.Y = int32(yOff)
	}
}

func (li *Line) bounds() image.Rectangle {
	return image.Rectangle{
		Min: image.Pt(li.XOff.Floor(), li.YOff-li.Ascent.Ceil()),
		Max: image.Pt((li.XOff + li.Width).Ceil(), li.YOff+li.Descent.Ceil()),
	}
}

func (li *Line) GetGlyphs(offset, count int) []text.Glyph {
	if count <= 0 {
		return []text.Glyph{}
	}

	out := make([]text.Glyph, count)
	for idx, gl := range li.Glyphs[offset : offset+count] {
		out[idx] = *gl
	}

	return out
}

func (li *Line) All() iter.Seq[text.Glyph] {
	return func(yield func(text.Glyph) bool) {
		for _, gl := range li.Glyphs {
			if !yield(*gl) {
				return
			}
		}
	}
}

// Paragraph contains the pixel coordinates of the start and end position
// of the paragraph.
type Paragraph struct {
	StartX fixed.Int26_6
	StartY int
	EndX   fixed.Int26_6
	EndY   int
	// Runes is the number of runes represented by this paragraph.
	Runes int
	// RuneOff tracks the rune offset of the first rune of the paragraph in the document.
	RuneOff int
}

// Add add a visual line to the paragraph, returning a boolean value indicating
// the end of a paragraph.
func (p *Paragraph) Add(li Line) bool {
	lastGlyph := li.Glyphs[len(li.Glyphs)-1]

	if p.Runes == 0 {
		start := li.Glyphs[0]
		p.StartX = start.X
		p.StartY = int(start.Y)

		p.EndX = lastGlyph.X
		p.EndY = int(lastGlyph.Y)

		p.RuneOff = li.RuneOff
	} else {
		p.EndX = lastGlyph.X
		p.EndY = int(lastGlyph.Y)
	}

	p.Runes += li.Runes
	return lastGlyph.Flags&text.FlagParagraphBreak != 0
}

type glyphIter struct {
	shaper *text.Shaper
}

func (gi glyphIter) All() iter.Seq[text.Glyph] {
	return func(yield func(text.Glyph) bool) {
		for {
			g, ok := gi.shaper.NextGlyph()
			if !ok {
				return
			}

			if !yield(g) {
				return
			}
		}
	}
}
