package gvcode

import (
	"image"
	"image/color"
	"sort"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	gvcolor "github.com/oligo/gvcode/color"
	"github.com/oligo/gvcode/gutter"
	"github.com/oligo/gvcode/internal/painter"
)

// GutterManager returns the editor's gutter manager, if one is configured.
func (e *Editor) GutterManager() *gutter.Manager {
	return e.gutterManager
}

// buildGutterContext creates a GutterContext from the current editor state.
func (e *Editor) buildGutterContext(gtx layout.Context, shaper *text.Shaper) gutter.GutterContext {
	viewport := e.text.Viewport()
	textLayout := e.text.TextLayout()

	// Convert internal Paragraphs to gutter.Paragraph slice
	paragraphs := make([]gutter.Paragraph, 0, len(textLayout.Paragraphs))
	for i, p := range textLayout.Paragraphs {
		// Skip paragraphs outside the viewport
		if p.EndY < viewport.Min.Y {
			continue
		}
		if p.StartY > viewport.Max.Y {
			break
		}
		paragraphs = append(paragraphs, gutter.Paragraph{
			StartY:  p.StartY,
			EndY:    p.EndY,
			Ascent:  p.Ascent,
			Descent: p.Descent,
			Runes:   p.Runes,
			RuneOff: p.RuneOff,
			Index:   i,
		})
	}

	// Determine current line (-1 if selection spans multiple lines)
	currentLine := -1
	if start, end := e.text.Selection(); start == end {
		currentLine, _ = e.text.CaretPos()
	}

	return gutter.GutterContext{
		Shaper:      shaper,
		TextParams:  e.text.Params(),
		Viewport:    viewport,
		Paragraphs:  paragraphs,
		CurrentLine: currentLine,
		LineHeight:  e.text.GetLineHeight(),
		Colors:      e.gutterColors(),
	}
}

// gutterColors returns the GutterColors based on the color palette.
func (e *Editor) gutterColors() *gutter.GutterColors {
	if e.colorPalette == nil {
		return &gutter.GutterColors{}
	}

	var text, highlight gvcolor.Color

	if e.colorPalette.LineNumberColor.IsSet() {
		highlight = e.colorPalette.LineNumberColor
		// Use a slightly dimmed version for non-highlighted lines
		text = e.colorPalette.LineNumberColor.MulAlpha(0x90)
	} else {
		// Default to foreground color with reduced alpha
		text = gvcolor.MakeColor(color.NRGBA{A: 0x90})
		highlight = gvcolor.MakeColor(color.NRGBA{A: 0xFF})
	}

	var lineHighlight gvcolor.Color
	if e.colorPalette.LineColor.IsSet() {
		lineHighlight = e.colorPalette.LineColor
	} else if e.colorPalette.Foreground.IsSet() {
		lineHighlight = e.colorPalette.Foreground.MulAlpha(0x30)
	}

	return &gutter.GutterColors{
		Text:          text,
		TextHighlight: highlight,
		Background:    gvcolor.Color{}, // Transparent by default
		LineHighlight: lineHighlight,
		Custom:        nil,
	}
}

// paintProviderHighlights paints full-width line highlights from gutter providers.
// The highlights span the entire editor width (gutter + text area).
// Consecutive lines with the same color are merged into a single polygon.
func (e *Editor) paintProviderHighlights(gtx layout.Context, ctx gutter.GutterContext, highlights []gutter.LineHighlight) {
	if len(highlights) == 0 {
		return
	}

	// Build a map of line index to paragraph for quick lookup
	paraByLine := make(map[int]gutter.Paragraph, len(ctx.Paragraphs))
	for _, p := range ctx.Paragraphs {
		paraByLine[p.Index] = p
	}

	// Sort highlights by line number
	sort.Slice(highlights, func(i, j int) bool {
		return highlights[i].Line < highlights[j].Line
	})

	// Group consecutive lines with the same color
	type highlightGroup struct {
		color gvcolor.Color
		rects []image.Rectangle
	}

	var groups []highlightGroup
	lineHeight := ctx.LineHeight.Ceil()
	scrollOffY := ctx.Viewport.Min.Y

	for _, hl := range highlights {
		para, ok := paraByLine[hl.Line]
		if !ok {
			// Line not visible, skip
			continue
		}

		// Calculate bounds from baseline using ascent/descent
		ascent := para.Ascent.Ceil()
		descent := para.Descent.Ceil()
		glyphHeight := ascent + descent

		// Calculate leading (extra space beyond glyph bounds)
		leading := 0
		if lineHeight > glyphHeight {
			leading = lineHeight - glyphHeight
		}
		leadingTop := leading / 2
		leadingBottom := leading - leadingTop

		// Build the full-width bounds
		bounds := image.Rectangle{
			Min: image.Point{X: 0, Y: para.StartY - ascent - leadingTop - scrollOffY},
			Max: image.Point{X: gtx.Constraints.Max.X, Y: para.EndY + descent + leadingBottom - scrollOffY},
		}

		// Check if this highlight can be added to the last group
		// (same color and consecutive line)
		if len(groups) > 0 {
			lastGroup := &groups[len(groups)-1]
			lastRect := lastGroup.rects[len(lastGroup.rects)-1]
			// Same color and vertically adjacent (or overlapping)
			if lastGroup.color == hl.Color && bounds.Min.Y <= lastRect.Max.Y {
				lastGroup.rects = append(lastGroup.rects, bounds)
				continue
			}
		}

		// Start a new group
		groups = append(groups, highlightGroup{
			color: hl.Color,
			rects: []image.Rectangle{bounds},
		})
	}

	// Paint each group using polygon builder (with radius=0 for sharp corners)
	polygonBuilder := painter.NewPolygonBuilder(false, 0, 0)

	for _, group := range groups {
		polygonBuilder.Group(group.rects)
		paths := polygonBuilder.Paths(gtx)

		for _, path := range paths {
			outline := clip.Outline{Path: path}.Op().Push(gtx.Ops)
			paint.ColorOp{Color: group.color.NRGBA()}.Add(gtx.Ops)
			paint.PaintOp{}.Add(gtx.Ops)
			outline.Pop()
		}
	}
}
