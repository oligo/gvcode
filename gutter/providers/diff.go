package providers

import (
	"image"

	"gioui.org/f32"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	gvcolor "github.com/oligo/gvcode/color"
	"github.com/oligo/gvcode/gutter"
)

const (
	// DiffProviderID is the unique identifier for the diff provider.
	DiffProviderID = "vcs-diff"
)

// DiffType represents the type of change in a diff hunk.
type DiffType int

const (
	// DiffAdded indicates lines that were added.
	DiffAdded DiffType = iota
	// DiffModified indicates lines that were modified.
	DiffModified
	// DiffDeleted indicates lines that were deleted.
	DiffDeleted
)

// DiffHunk represents a single change in the diff.
type DiffHunk struct {
	// Type is the type of change (added, modified, deleted).
	Type DiffType

	// StartLine is the 0-based line index where this hunk starts in the current document.
	StartLine int

	// EndLine is the 0-based line index where this hunk ends (inclusive).
	// For deleted hunks, StartLine == EndLine and represents where the deletion occurred.
	EndLine int

	// OldLines contains the original content (for modified and deleted hunks).
	OldLines []string

	// NewLines contains the new content (for added and modified hunks).
	NewLines []string
}

// LineCount returns the number of lines affected by this hunk in the current document.
func (h *DiffHunk) LineCount() int {
	if h.Type == DiffDeleted {
		return 0 // Deleted lines don't occupy space in current document
	}
	return h.EndLine - h.StartLine + 1
}

// VCSDiffProvider renders vcs (like git) diff indicators in the gutter.
// It shows colored bars for added/modified lines and triangles for deleted lines.
type VCSDiffProvider struct {
	// hunks stores all diff hunks keyed by their start line.
	hunks map[int]*DiffHunk

	// lineToHunk maps each affected line to its hunk for quick lookup.
	lineToHunk map[int]*DiffHunk

	// deletedAtLine tracks lines after which deletions occurred.
	deletedAtLine map[int]*DiffHunk

	// Colors for different diff types.
	addedColor    gvcolor.Color
	modifiedColor gvcolor.Color
	deletedColor  gvcolor.Color

	// indicatorWidth is the width of the colored bar indicator.
	indicatorWidth unit.Dp

	// highlightLines controls whether to provide full-width line highlighting.
	highlightLines bool

	// highlightAlpha is the alpha value for line highlighting (0-255).
	highlightAlpha uint8
}

// NewGitDiffProvider creates a new git diff provider with default colors.
func NewVCSDiffProvider() *VCSDiffProvider {
	addedColor, _ := gvcolor.Hex2Color("#46ca65")      // Green
	modifiedColor, _ := gvcolor.Hex2Color("#d6a22a97") // Yellow/Orange
	deletedColor, _ := gvcolor.Hex2Color("#e5534b")    // Red

	return &VCSDiffProvider{
		hunks:          make(map[int]*DiffHunk),
		lineToHunk:     make(map[int]*DiffHunk),
		deletedAtLine:  make(map[int]*DiffHunk),
		addedColor:     addedColor,
		modifiedColor:  modifiedColor,
		deletedColor:   deletedColor,
		indicatorWidth: unit.Dp(6),
		highlightLines: true,
		highlightAlpha: 0x20,
	}
}

// SetColors sets custom colors for the diff indicators.
func (p *VCSDiffProvider) SetColors(added, modified, deleted gvcolor.Color) {
	p.addedColor = added
	p.modifiedColor = modified
	p.deletedColor = deleted
}

// SetIndicatorWidth sets the width of the indicator bar.
func (p *VCSDiffProvider) SetIndicatorWidth(width unit.Dp) {
	p.indicatorWidth = width
}

// SetHighlightLines enables or disables full-width line highlighting.
func (p *VCSDiffProvider) SetHighlightLines(enabled bool, alpha uint8) {
	p.highlightLines = enabled
	p.highlightAlpha = alpha
}

// UpdateDiff updates the diff state with new hunks.
// This clears any existing diff data.
func (p *VCSDiffProvider) UpdateDiff(hunks []*DiffHunk) {
	// Clear existing data
	p.hunks = make(map[int]*DiffHunk)
	p.lineToHunk = make(map[int]*DiffHunk)
	p.deletedAtLine = make(map[int]*DiffHunk)

	for _, hunk := range hunks {
		p.hunks[hunk.StartLine] = hunk

		if hunk.Type == DiffDeleted {
			// For deletions, track the line after which the deletion occurred
			p.deletedAtLine[hunk.StartLine] = hunk
		} else {
			// For added/modified, map all affected lines to the hunk
			for line := hunk.StartLine; line <= hunk.EndLine; line++ {
				p.lineToHunk[line] = hunk
			}
		}
	}
}

// ClearDiff removes all diff data.
func (p *VCSDiffProvider) ClearDiff() {
	p.hunks = make(map[int]*DiffHunk)
	p.lineToHunk = make(map[int]*DiffHunk)
	p.deletedAtLine = make(map[int]*DiffHunk)
}

// GetHunk returns the diff hunk for the given line, or nil if none exists.
func (p *VCSDiffProvider) GetHunk(line int) *DiffHunk {
	if hunk, ok := p.lineToHunk[line]; ok {
		return hunk
	}
	if hunk, ok := p.deletedAtLine[line]; ok {
		return hunk
	}
	return nil
}

// GetAllHunks returns all diff hunks.
func (p *VCSDiffProvider) GetAllHunks() []*DiffHunk {
	result := make([]*DiffHunk, 0, len(p.hunks))
	for _, hunk := range p.hunks {
		result = append(result, hunk)
	}
	return result
}

// ID returns the unique identifier for this provider.
func (p *VCSDiffProvider) ID() string {
	return DiffProviderID
}

// Priority returns the rendering priority.
// Git diff indicators are rendered leftmost (highest priority value).
func (p *VCSDiffProvider) Priority() int {
	return 200
}

// Width returns the width needed for the indicator.
func (p *VCSDiffProvider) Width(gtx layout.Context, shaper *text.Shaper, params text.Parameters, lineCount int) unit.Dp {
	if len(p.hunks) == 0 {
		return 0
	}
	return p.indicatorWidth
}

// Layout renders the git diff indicators for visible paragraphs.
func (p *VCSDiffProvider) Layout(gtx layout.Context, ctx gutter.GutterContext) layout.Dimensions {
	if len(p.hunks) == 0 {
		return layout.Dimensions{}
	}

	indicatorWidthPx := gtx.Dp(p.indicatorWidth)
	lineHeight := ctx.LineHeight.Ceil()
	scrollOffY := ctx.Viewport.Min.Y

	for _, para := range ctx.Paragraphs {
		lineIdx := para.Index

		// Check for added/modified lines
		if hunk, ok := p.lineToHunk[lineIdx]; ok {
			var c gvcolor.Color
			switch hunk.Type {
			case DiffAdded:
				c = p.addedColor
			case DiffModified:
				c = p.modifiedColor
			}

			if c.IsSet() {
				p.drawIndicatorBar(gtx, para, indicatorWidthPx, lineHeight, scrollOffY, c)
			}
		}

		// Check for deletions (draw triangle marker)
		if hunk, ok := p.deletedAtLine[lineIdx]; ok {
			p.drawDeletedMarker(gtx, para, indicatorWidthPx, lineHeight, scrollOffY, hunk)
		}
	}

	return layout.Dimensions{
		Size: image.Point{X: indicatorWidthPx, Y: gtx.Constraints.Max.Y},
	}
}

// drawIndicatorBar draws a colored vertical bar for added/modified lines.
func (p *VCSDiffProvider) drawIndicatorBar(gtx layout.Context, para gutter.Paragraph, width, lineHeight, scrollOffY int, c gvcolor.Color) {
	ascent := para.Ascent.Ceil()
	descent := para.Descent.Ceil()
	glyphHeight := ascent + descent

	// Calculate leading
	leading := 0
	if lineHeight > glyphHeight {
		leading = lineHeight - glyphHeight
	}
	leadingTop := leading / 2
	leadingBottom := leading - leadingTop

	// Calculate bounds
	top := para.StartY - ascent - leadingTop - scrollOffY
	bottom := para.EndY + descent + leadingBottom - scrollOffY

	rect := image.Rectangle{
		Min: image.Point{X: 0, Y: top},
		Max: image.Point{X: width, Y: bottom},
	}

	stack := clip.Rect(rect).Push(gtx.Ops)
	paint.ColorOp{Color: c.NRGBA()}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	stack.Pop()
}

// drawDeletedMarker draws a triangle marker indicating deleted lines.
func (p *VCSDiffProvider) drawDeletedMarker(gtx layout.Context, para gutter.Paragraph, width, lineHeight, scrollOffY int, hunk *DiffHunk) {
	ascent := para.Ascent.Ceil()
	descent := para.Descent.Ceil()
	glyphHeight := ascent + descent

	// Calculate leading
	leading := 0
	if lineHeight > glyphHeight {
		leading = lineHeight - glyphHeight
	}
	leadingBottom := leading - leading/2

	// Position the triangle at the bottom of the line (where deletion occurred)
	y := float32(para.EndY + descent + leadingBottom - scrollOffY)

	// Draw a small triangle pointing right
	triangleSize := float32(width)
	if triangleSize < 8 {
		triangleSize = 8
	}

	var path clip.Path
	path.Begin(gtx.Ops)
	path.MoveTo(f32.Point{X: 0, Y: y - triangleSize/2})
	path.LineTo(f32.Point{X: triangleSize, Y: y})
	path.LineTo(f32.Point{X: 0, Y: y + triangleSize/2})
	path.Close()

	stack := clip.Outline{Path: path.End()}.Op().Push(gtx.Ops)
	paint.ColorOp{Color: p.deletedColor.NRGBA()}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	stack.Pop()
}

// HandleClick handles click events on the gutter.
// Implements InteractiveGutter interface.
func (p *VCSDiffProvider) HandleClick(line int, source pointer.Source, numClicks int, modifiers key.Modifiers) bool {
	// Check if there's a hunk at this line
	hunk := p.GetHunk(line)
	return hunk != nil
}

// HandleHover handles hover events on the gutter.
// Implements InteractiveGutter interface.
func (p *VCSDiffProvider) HandleHover(line int) *gutter.HoverInfo {
	hunk := p.GetHunk(line)
	if hunk == nil {
		return nil
	}

	var text string
	switch hunk.Type {
	case DiffAdded:
		text = "Added lines - Click to view"
	case DiffModified:
		text = "Modified lines - Click to view"
	case DiffDeleted:
		text = "Deleted lines - Click to view"
	}

	return &gutter.HoverInfo{
		Text: text,
	}
}

// HighlightedLines returns line highlights for changed lines.
// Implements LineHighlighter interface.
func (p *VCSDiffProvider) HighlightedLines() []gutter.LineHighlight {
	if !p.highlightLines || len(p.lineToHunk) == 0 {
		return nil
	}

	highlights := make([]gutter.LineHighlight, 0, len(p.lineToHunk))

	for line, hunk := range p.lineToHunk {
		var c gvcolor.Color
		switch hunk.Type {
		case DiffAdded:
			c = p.addedColor.MulAlpha(p.highlightAlpha)
		case DiffModified:
			c = p.modifiedColor.MulAlpha(p.highlightAlpha)
		}

		if c.IsSet() {
			highlights = append(highlights, gutter.LineHighlight{
				Line:  line,
				Color: c,
			})
		}
	}

	return highlights
}

// Ensure GitDiffProvider implements the required interfaces.
var (
	_ gutter.GutterProvider    = (*VCSDiffProvider)(nil)
	_ gutter.InteractiveGutter = (*VCSDiffProvider)(nil)
	_ gutter.LineHighlighter   = (*VCSDiffProvider)(nil)
)
