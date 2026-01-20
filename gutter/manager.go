package gutter

import (
	"image"
	"sort"

	"gioui.org/gesture"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"golang.org/x/image/math/fixed"
)

// Manager coordinates multiple gutter providers, handling layout,
// event processing, and hit testing.
type Manager struct {
	// providers holds all registered providers, sorted by priority.
	providers []GutterProvider

	// gap is the spacing between gutter columns.
	gap unit.Dp

	// totalWidth is the cached total width of all providers.
	totalWidth int

	// providerBounds maps provider IDs to their rendered bounds for hit testing.
	providerBounds map[string]image.Rectangle

	// providerWidths caches the width of each provider for the current layout.
	providerWidths map[string]int

	// clicker handles click events on the gutter area.
	clicker gesture.Click

	// pending holds events that haven't been consumed yet.
	pending []GutterEvent

	// paragraphs caches the visible paragraphs from the last layout for hit testing.
	paragraphs []Paragraph

	// lineHeight caches the line height from the last layout for expanding hit test bounds.
	lineHeight fixed.Int26_6

	// viewport caches the viewport from the last layout.
	viewport image.Rectangle
}

// NewManager creates a new gutter manager with default settings.
func NewManager() *Manager {
	return &Manager{
		providers:      make([]GutterProvider, 0),
		providerBounds: make(map[string]image.Rectangle),
		providerWidths: make(map[string]int),
		gap:            unit.Dp(2),
	}
}

// Register adds a provider to the manager. Providers are automatically
// sorted by priority (lower priority = rendered closer to text).
func (m *Manager) Register(provider GutterProvider) {
	// Check if provider with same ID already exists
	for i, p := range m.providers {
		if p.ID() == provider.ID() {
			m.providers[i] = provider
			m.sortProviders()
			return
		}
	}

	m.providers = append(m.providers, provider)
	m.sortProviders()
}

// Unregister removes a provider by its ID.
func (m *Manager) Unregister(id string) {
	for i, p := range m.providers {
		if p.ID() == id {
			m.providers = append(m.providers[:i], m.providers[i+1:]...)
			delete(m.providerBounds, id)
			delete(m.providerWidths, id)
			return
		}
	}
}

// GetProvider returns a provider by its ID, or nil if not found.
func (m *Manager) GetProvider(id string) GutterProvider {
	for _, p := range m.providers {
		if p.ID() == id {
			return p
		}
	}
	return nil
}

// Providers returns a slice of all registered providers.
func (m *Manager) Providers() []GutterProvider {
	return m.providers
}

// TotalWidth returns the total width of all gutter columns including gaps.
func (m *Manager) TotalWidth() int {
	return m.totalWidth
}

// SetGap sets the spacing between gutter columns.
func (m *Manager) SetGap(gap unit.Dp) {
	m.gap = gap
}

// sortProviders sorts providers by priority (lower = closer to text).
// Since we render left-to-right but want lower priority closer to text,
// we sort in descending order so higher priority providers come first.
func (m *Manager) sortProviders() {
	sort.Slice(m.providers, func(i, j int) bool {
		return m.providers[i].Priority() > m.providers[j].Priority()
	})
}

// Update processes input events and returns any gutter events.
// Call this before Layout to process click/hover events.
func (m *Manager) Update(gtx layout.Context) (GutterEvent, bool) {
	// Return any pending events first
	if len(m.pending) > 0 {
		evt := m.pending[0]
		m.pending = m.pending[1:]
		return evt, true
	}

	// Process click events
	for {
		evt, ok := m.clicker.Update(gtx.Source)
		if !ok {
			break
		}

		if evt.Kind == gesture.KindClick {
			m.handleClick(gtx, evt)
		}
	}

	// Return any newly generated events
	if len(m.pending) > 0 {
		evt := m.pending[0]
		m.pending = m.pending[1:]
		return evt, true
	}

	return nil, false
}

// handleClick processes a click event and generates appropriate gutter events.
func (m *Manager) handleClick(gtx layout.Context, evt gesture.ClickEvent) {
	pos := image.Point{X: int(evt.Position.X), Y: int(evt.Position.Y)}

	// Find which provider was clicked
	for _, p := range m.providers {
		bounds, ok := m.providerBounds[p.ID()]
		if !ok {
			continue
		}

		if pos.In(bounds) {
			// Calculate which line was clicked
			line := m.hitTestLine(pos.Y)
			if line < 0 {
				continue
			}

			// Check if provider is interactive
			if interactive, ok := p.(InteractiveGutter); ok {
				if interactive.HandleClick(line, evt.Source, evt.NumClicks, evt.Modifiers) {
					m.pending = append(m.pending, GutterClickEvent{
						ProviderID: p.ID(),
						Line:       line,
						Source:     evt.Source,
						NumClicks:  evt.NumClicks,
						Modifiers:  evt.Modifiers,
					})
					return
				}
			} else {
				// Non-interactive provider, still emit event
				m.pending = append(m.pending, GutterClickEvent{
					ProviderID: p.ID(),
					Line:       line,
					Source:     evt.Source,
					NumClicks:  evt.NumClicks,
					Modifiers:  evt.Modifiers,
				})
				return
			}
		}
	}
}

// hitTestLine determines which logical line (paragraph) index corresponds to a Y coordinate.
// The Y coordinate is in local gutter coordinates (0 = top of visible area).
// The function expands paragraph bounds by the leading (line height - glyph height) to cover
// the gaps between lines, similar to how adjustPadding works in textview.
func (m *Manager) hitTestLine(y int) int {
	if len(m.paragraphs) == 0 {
		return -1
	}

	// Convert local Y to document Y coordinate
	docY := y + m.viewport.Min.Y
	idx := sort.Search(len(m.paragraphs), func(i int) bool {
		_, expandedEndY := m.expandBounds(m.paragraphs[i])
		return expandedEndY >= docY
	})

	if idx >= len(m.paragraphs) {
		return -1
	}

	para := m.paragraphs[idx]
	expandedStartY, expandedEndY := m.expandBounds(para)

	// Check if docY is actually within this paragraph's expanded bounds
	if docY < expandedStartY || docY > expandedEndY {
		return -1
	}

	return para.Index
}

// expandBounds expands a paragraph's vertical bounds to cover the full clickable area.
// StartY and EndY are baselines for the first and last screen lines of the paragraph.
// We use Ascent and Descent to calculate glyph bounds, then add leading if line height is larger.
func (m *Manager) expandBounds(para Paragraph) (startY, endY int) {
	ascent := para.Ascent.Ceil()
	descent := para.Descent.Ceil()
	glyphHeight := ascent + descent
	lineHeightPx := m.lineHeight.Ceil()

	// Calculate leading (extra space beyond glyph bounds)
	leading := 0
	if lineHeightPx > glyphHeight {
		leading = lineHeightPx - glyphHeight
	}

	// Split leading evenly above and below
	leadingTop := leading / 2
	leadingBottom := leading - leadingTop

	// Top: baseline - ascent - leading above
	// Bottom: baseline + descent + leading below
	return para.StartY - ascent - leadingTop, para.EndY + descent + leadingBottom
}

// Layout renders all gutter providers and returns the total dimensions.
func (m *Manager) Layout(gtx layout.Context, ctx GutterContext) layout.Dimensions {
	if len(m.providers) == 0 {
		m.totalWidth = 0
		return layout.Dimensions{}
	}

	// Cache layout parameters for hit testing
	m.paragraphs = append(m.paragraphs[:0], ctx.Paragraphs...)
	m.lineHeight = ctx.LineHeight
	m.viewport = ctx.Viewport

	// Calculate total line count for width calculation
	lineCount := 0
	if len(ctx.Paragraphs) > 0 {
		// Use the index of the last paragraph + 1 as the actual line count
		lineCount = ctx.Paragraphs[len(ctx.Paragraphs)-1].Index + 1
	}

	// Calculate total width
	gapPx := gtx.Dp(m.gap)
	totalWidth := 0

	for i, p := range m.providers {
		width := gtx.Dp(p.Width(gtx, ctx.Shaper, ctx.TextParams, lineCount))
		m.providerWidths[p.ID()] = width
		totalWidth += width
		if i < len(m.providers)-1 {
			totalWidth += gapPx
		}
	}

	m.totalWidth = totalWidth

	// Set up the clip area and register for events
	area := clip.Rect(image.Rectangle{Max: image.Point{X: totalWidth, Y: gtx.Constraints.Max.Y}})
	stack := area.Push(gtx.Ops)

	// Paint background if specified
	if ctx.Colors != nil && ctx.Colors.Background.A > 0 {
		paint.ColorOp{Color: ctx.Colors.Background}.Add(gtx.Ops)
		paint.PaintOp{}.Add(gtx.Ops)
	}

	// Register click handler
	pointer.CursorDefault.Add(gtx.Ops)
	m.clicker.Add(gtx.Ops)

	// Render each provider
	xOffset := 0
	for i, p := range m.providers {
		width := m.providerWidths[p.ID()]

		// Record the bounds for this provider
		m.providerBounds[p.ID()] = image.Rectangle{
			Min: image.Point{X: xOffset, Y: 0},
			Max: image.Point{X: xOffset + width, Y: gtx.Constraints.Max.Y},
		}

		// Set up the transform and constraints for this provider
		trans := op.Offset(image.Point{X: xOffset, Y: 0}).Push(gtx.Ops)

		providerGtx := gtx
		providerGtx.Constraints = layout.Exact(image.Point{X: width, Y: gtx.Constraints.Max.Y})

		p.Layout(providerGtx, ctx)

		trans.Pop()

		xOffset += width
		if i < len(m.providers)-1 {
			xOffset += gapPx
		}
	}

	stack.Pop()

	return layout.Dimensions{
		Size: image.Point{X: totalWidth, Y: gtx.Constraints.Max.Y},
	}
}

// CalculateWidth calculates the total width without rendering.
// Useful for layout calculations before actual rendering.
func (m *Manager) CalculateWidth(gtx layout.Context, shaper *text.Shaper, params text.Parameters, lineCount int) int {
	if len(m.providers) == 0 {
		return 0
	}

	gapPx := gtx.Dp(m.gap)
	totalWidth := 0

	for i, p := range m.providers {
		width := gtx.Dp(p.Width(gtx, shaper, params, lineCount))
		totalWidth += width
		if i < len(m.providers)-1 {
			totalWidth += gapPx
		}
	}

	return totalWidth
}

// HasProviders returns true if there are any registered providers.
func (m *Manager) HasProviders() bool {
	return len(m.providers) > 0
}
