package completion

import (
	"fmt"
	"image"
	"image/color"
	"strings"

	"gioui.org/font"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/oligo/gvcode"
)

// Kind icon configuration
type kindStyle struct {
	Icon  string
	Color color.NRGBA
}

// Define base colors for reuse
var (
	kindColorBlue   = color.NRGBA{R: 100, G: 149, B: 237, A: 255} // Functions
	kindColorCyan   = color.NRGBA{R: 86, G: 182, B: 194, A: 255}  // Variables, fields
	kindColorYellow = color.NRGBA{R: 229, G: 192, B: 123, A: 255} // Types, classes
	kindColorPurple = color.NRGBA{R: 198, G: 120, B: 221, A: 255} // Keywords, modules
	kindColorGreen  = color.NRGBA{R: 152, G: 195, B: 121, A: 255} // Snippets
	kindColorOrange = color.NRGBA{R: 209, G: 154, B: 102, A: 255} // Constants, enums
	kindColorGray   = color.NRGBA{R: 171, G: 178, B: 191, A: 255} // Text, misc
	kindColorRed    = color.NRGBA{R: 224, G: 108, B: 117, A: 255} // Special
)

var kindStyles = map[string]kindStyle{
	// Functions (full and abbreviated names)
	"function":    {Icon: "fn", Color: kindColorBlue},
	"func":        {Icon: "fn", Color: kindColorBlue},
	"method":      {Icon: "fn", Color: kindColorBlue},
	"constructor": {Icon: "fn", Color: kindColorBlue},

	// Variables and fields
	"variable": {Icon: "ab", Color: kindColorCyan},
	"var":      {Icon: "ab", Color: kindColorCyan},
	"field":    {Icon: "fd", Color: kindColorCyan},
	"property": {Icon: "fd", Color: kindColorCyan},
	"prop":     {Icon: "fd", Color: kindColorCyan},
	"param":    {Icon: "ab", Color: kindColorCyan},
	"parameter":{Icon: "ab", Color: kindColorCyan},

	// Types and classes
	"class":     {Icon: "C", Color: kindColorYellow},
	"interface": {Icon: "I", Color: kindColorYellow},
	"struct":    {Icon: "S", Color: kindColorYellow},
	"type":      {Icon: "T", Color: kindColorYellow},
	"typedef":   {Icon: "T", Color: kindColorYellow},

	// Modules and packages
	"module":  {Icon: "M", Color: kindColorPurple},
	"mod":     {Icon: "M", Color: kindColorPurple},
	"package": {Icon: "P", Color: kindColorPurple},
	"pkg":     {Icon: "P", Color: kindColorPurple},
	"keyword": {Icon: "K", Color: kindColorPurple},
	"kw":      {Icon: "K", Color: kindColorPurple},

	// Snippets and text
	"snippet": {Icon: "sn", Color: kindColorGreen},
	"text":    {Icon: "tx", Color: kindColorGray},

	// Constants and values
	"constant":   {Icon: "ct", Color: kindColorOrange},
	"const":      {Icon: "ct", Color: kindColorOrange},
	"enum":       {Icon: "E", Color: kindColorOrange},
	"enummember": {Icon: "em", Color: kindColorOrange},
	"value":      {Icon: "vl", Color: kindColorOrange},
	"unit":       {Icon: "ut", Color: kindColorOrange},

	// Files and references
	"file":      {Icon: "fi", Color: kindColorGray},
	"folder":    {Icon: "fo", Color: kindColorYellow},
	"dir":       {Icon: "fo", Color: kindColorYellow},
	"reference": {Icon: "rf", Color: kindColorGray},
	"ref":       {Icon: "rf", Color: kindColorGray},

	// Special
	"color":   {Icon: "co", Color: kindColorRed},
	"event":   {Icon: "ev", Color: kindColorOrange},
	"operator":{Icon: "op", Color: kindColorPurple},
	"symbol":  {Icon: "sy", Color: kindColorCyan},
}

var defaultKindStyle = kindStyle{Icon: "??", Color: kindColorGray}

func getKindStyle(kind string) kindStyle {
	if style, ok := kindStyles[strings.ToLower(kind)]; ok {
		return style
	}
	// Log unknown kind for debugging
	if kind != "" {
		logger.Debug("unknown completion kind", "kind", kind)
	}
	return defaultKindStyle
}

// CompletionPopup is the builtin implementation of a completion popup.
type CompletionPopup struct {
	editor     *gvcode.Editor
	cmp        gvcode.Completion
	list       widget.List
	itemsCount int
	focused    int
	labels     []*itemLabel

	// Size configures the max popup dimensions. If no value
	// is provided, a reasonable value is set.
	Size image.Point
	// TextSize configures the size the text displayed in the popup. If no value
	// is provided, a reasonable value is set.
	TextSize unit.Sp
	// Color used to highlight the selected item.
	HighlightColor color.NRGBA
	Theme          *material.Theme
}

func NewCompletionPopup(editor *gvcode.Editor, cmp gvcode.Completion) *CompletionPopup {
	return &CompletionPopup{
		editor: editor,
		cmp:    cmp,
	}
}

func (pop *CompletionPopup) Layout(gtx layout.Context, items []gvcode.CompletionCandidate) layout.Dimensions {
	pop.itemsCount = len(items)
	pop.update(gtx)

	if !pop.cmp.IsActive() || pop.itemsCount == 0 {
		pop.reset()
		return layout.Dimensions{}
	}

	// Set constraints for popup content
	gtx.Constraints.Max = pop.Size
	gtx.Constraints.Min = image.Point{
		X: gtx.Constraints.Max.X,
		Y: 0,
	}

	// Record the content to measure its size first
	macro := op.Record(gtx.Ops)
	contentDims := layout.UniformInset(unit.Dp(4)).Layout(gtx,
		func(gtx layout.Context) layout.Dimensions {
			// Measure footer first to know how much space it needs
			footerMacro := op.Record(gtx.Ops)
			footerDims := pop.layoutFooter(gtx, pop.Theme)
			footerCall := footerMacro.Stop()

			// Constrain list height to leave room for footer
			listGtx := gtx
			listGtx.Constraints.Max.Y = max(0, gtx.Constraints.Max.Y-footerDims.Size.Y)

			// Layout list with constrained height
			listDims := pop.layoutList(listGtx, pop.Theme, items)

			// Draw footer below the list
			defer op.Offset(image.Point{Y: listDims.Size.Y}).Push(gtx.Ops).Pop()
			footerCall.Add(gtx.Ops)

			return layout.Dimensions{
				Size: image.Point{
					X: max(listDims.Size.X, footerDims.Size.X),
					Y: listDims.Size.Y + footerDims.Size.Y,
				},
			}
		})
	contentCall := macro.Stop()

	// Calculate shadow and border dimensions
	cornerRadius := gtx.Dp(unit.Dp(6))
	shadowOffset := gtx.Dp(unit.Dp(2))
	shadowBlur := gtx.Dp(unit.Dp(8))

	// Draw shadow layers (from outer to inner for depth effect)
	shadowColors := []color.NRGBA{
		{A: 0x08},
		{A: 0x10},
		{A: 0x18},
	}
	for i, shadowColor := range shadowColors {
		offset := shadowOffset + shadowBlur - i*(shadowBlur/len(shadowColors))
		shadowRect := image.Rectangle{
			Min: image.Point{X: offset / 2, Y: offset / 2},
			Max: image.Point{X: contentDims.Size.X + offset, Y: contentDims.Size.Y + offset},
		}
		paint.FillShape(gtx.Ops, shadowColor,
			clip.UniformRRect(shadowRect, cornerRadius+offset/4).Op(gtx.Ops))
	}

	// Draw background with rounded corners
	bgRect := image.Rectangle{Max: contentDims.Size}
	defer clip.UniformRRect(bgRect, cornerRadius).Push(gtx.Ops).Pop()
	paint.Fill(gtx.Ops, pop.Theme.Bg)

	// Draw subtle border
	borderColor := adjustAlpha(pop.Theme.Fg, 0x30)
	paint.FillShape(gtx.Ops, borderColor,
		clip.Stroke{
			Path:  clip.UniformRRect(bgRect, cornerRadius).Path(gtx.Ops),
			Width: float32(gtx.Dp(unit.Dp(1))),
		}.Op())

	// Draw the content
	contentCall.Add(gtx.Ops)

	return contentDims
}

func (pop *CompletionPopup) updateSelection(direction int) {
	pop.labels[pop.focused].selected = false
	if direction < 0 {
		pop.focused = max(pop.focused+direction, 0)
	} else {
		pop.focused = min(pop.focused+direction, pop.itemsCount-1)
	}

	pop.labels[pop.focused].selected = true

	// Keep some items visible around the focused item for context
	const contextItems = 2

	// Calculate the range of fully visible items
	firstVisible := pop.list.Position.First
	if pop.list.Position.Offset > 0 {
		firstVisible++ // First item is partially hidden
	}
	lastVisible := pop.list.Position.First + pop.list.Position.Count - 1
	if pop.list.Position.OffsetLast < 0 {
		lastVisible-- // Last item is partially hidden
	}

	if direction > 0 {
		// Moving down: ensure contextItems are visible below focused item
		if pop.focused+contextItems > lastVisible && lastVisible < pop.itemsCount-1 {
			pop.list.ScrollBy(float32(direction))
		}
	} else {
		// Moving up: ensure contextItems are visible above focused item
		if pop.focused-contextItems < firstVisible && firstVisible > 0 {
			pop.list.ScrollBy(float32(direction))
		}
	}
}

func (pop *CompletionPopup) reset() {
	pop.focused = 0
	pop.labels = pop.labels[:0]
	pop.list.ScrollTo(0)
	pop.editor.RemoveCommands(pop)
}

func (pop *CompletionPopup) update(gtx layout.Context) {
	if pop.TextSize <= 0 {
		pop.TextSize = unit.Sp(12)
	}
	if pop.Size == (image.Point{}) {
		pop.Size = image.Point{
			X: gtx.Dp(unit.Dp(400)),
			Y: gtx.Dp(unit.Dp(200)),
		}
	}

	pop.editor.RegisterCommand(pop, key.Filter{Name: key.NameUpArrow, Optional: key.ModShift},
		func(gtx layout.Context, evt key.Event) gvcode.EditorEvent {
			pop.updateSelection(-1)
			return nil
		},
	)
	pop.editor.RegisterCommand(pop, key.Filter{Name: key.NameDownArrow, Optional: key.ModShift},
		func(gtx layout.Context, evt key.Event) gvcode.EditorEvent {
			pop.updateSelection(1)
			return nil
		},
	)
	pop.editor.RegisterCommand(pop, key.Filter{Name: key.NameEnter, Optional: key.ModShift},
		func(gtx layout.Context, evt key.Event) gvcode.EditorEvent {
			if pop.focused >= 0 && len(pop.labels) > 0 {
				// simulate a click
				pop.labels[pop.focused].Click()
			}
			return nil
		},
	)

	pop.editor.RegisterCommand(pop, key.Filter{Name: key.NameReturn, Optional: key.ModShift},
		func(gtx layout.Context, evt key.Event) gvcode.EditorEvent {
			if pop.focused >= 0 && len(pop.labels) > 0 {
				// simulate a click
				pop.labels[pop.focused].Click()
			}
			return nil
		},
	)

	// press Tab to confirm
	pop.editor.RegisterCommand(pop, key.Filter{Name: key.NameTab, Optional: key.ModShift},
		func(gtx layout.Context, evt key.Event) gvcode.EditorEvent {
			if pop.focused >= 0 && len(pop.labels) > 0 {
				// simulate a click
				pop.labels[pop.focused].Click()
			}
			return nil
		},
	)

	// press ESC to cancel and close the popup
	pop.editor.RegisterCommand(pop, key.Filter{Name: key.NameEscape},
		func(gtx layout.Context, evt key.Event) gvcode.EditorEvent {
			pop.cmp.Cancel()
			return nil
		},
	)

	if len(pop.labels) < pop.itemsCount {
		for i := len(pop.labels); i < pop.itemsCount; i++ {
			pop.labels = append(pop.labels, &itemLabel{onClicked: func() {
				pop.cmp.OnConfirm(i)
				gtx.Execute(key.FocusCmd{Tag: pop.editor})
				gtx.Execute(op.InvalidateCmd{})
			}})
		}
	} else {
		pop.labels = pop.labels[:pop.itemsCount]
	}

	if len(pop.labels) > 0 {
		pop.focused = min(pop.focused, len(pop.labels)-1)
		// Clear all selected states before setting the focused one
		// to avoid multiple items appearing selected when labels are reused
		for _, lbl := range pop.labels {
			lbl.selected = false
		}
		pop.labels[pop.focused].selected = true
	}

}

func (pop *CompletionPopup) layoutFooter(gtx layout.Context, th *material.Theme) layout.Dimensions {
	// Show position / total in a subtle footer
	text := fmt.Sprintf("%d / %d", pop.focused+1, pop.itemsCount)

	return layout.Inset{
		Top:    unit.Dp(4),
		Bottom: unit.Dp(2),
		Left:   unit.Dp(8),
		Right:  unit.Dp(8),
	}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return layout.Dimensions{Size: image.Point{X: gtx.Constraints.Min.X, Y: 0}}
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				lb := material.Label(th, pop.TextSize-1, text)
				lb.Color = adjustAlpha(th.Fg, 0x66)
				return lb.Layout(gtx)
			}),
		)
	})
}

func (pop *CompletionPopup) layoutList(gtx layout.Context, th *material.Theme, items []gvcode.CompletionCandidate) layout.Dimensions {
	pop.list.Axis = layout.Vertical

	li := material.List(th, &pop.list)
	li.AnchorStrategy = material.Overlay
	li.ScrollbarStyle.Indicator.HoverColor = adjustAlpha(th.ContrastBg, 0xb0)
	li.ScrollbarStyle.Indicator.Color = adjustAlpha(th.ContrastBg, 0x20)
	li.ScrollbarStyle.Indicator.MinorWidth = unit.Dp(6)

	return li.Layout(gtx, len(items), func(gtx layout.Context, index int) layout.Dimensions {
		c := items[index]
		highlightColor := pop.HighlightColor
		if highlightColor == (color.NRGBA{}) {
			highlightColor = adjustAlpha(th.ContrastBg, 0x40)
		}
		return pop.labels[index].Layout(gtx, highlightColor, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{
				Top:    unit.Dp(3),
				Bottom: unit.Dp(3),
				Left:   unit.Dp(8),
				Right:  unit.Dp(8),
			}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{
					Axis:      layout.Horizontal,
					Alignment: layout.Middle,
				}.Layout(gtx,
					// Kind icon with fixed width badge
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						ks := getKindStyle(c.Kind)
						badgeWidth := gtx.Dp(unit.Dp(22))
						badgeHeight := gtx.Dp(unit.Dp(16))
						cornerRadius := gtx.Dp(unit.Dp(3))

						return layout.Stack{Alignment: layout.Center}.Layout(gtx,
							// Background badge
							layout.Stacked(func(gtx layout.Context) layout.Dimensions {
								size := image.Point{X: badgeWidth, Y: badgeHeight}
								bgColor := adjustAlpha(ks.Color, 0x25)
								rect := clip.UniformRRect(image.Rectangle{Max: size}, cornerRadius)
								paint.FillShape(gtx.Ops, bgColor, rect.Op(gtx.Ops))
								return layout.Dimensions{Size: size}
							}),
							// Icon text centered
							layout.Stacked(func(gtx layout.Context) layout.Dimensions {
								lb := material.Label(th, pop.TextSize-1, ks.Icon)
								lb.Color = ks.Color
								lb.Font.Weight = font.SemiBold
								lb.Alignment = 1
								return lb.Layout(gtx)
							}),
						)
					}),
					layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
					// Label (main text)
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						lb := material.Label(th, pop.TextSize, c.Label)
						lb.Font.Weight = font.Medium
						return lb.Layout(gtx)
					}),
					// Flexible spacer
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return layout.Dimensions{Size: image.Point{X: gtx.Constraints.Min.X, Y: 0}}
					}),
					// Description (dimmed, right-aligned)
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						if c.Description == "" {
							return layout.Dimensions{}
						}
						lb := material.Label(th, pop.TextSize-1, c.Description)
						lb.Color = adjustAlpha(th.Fg, 0x99)
						lb.MaxLines = 1
						return lb.Layout(gtx)
					}),
				)
			})
		})
	})
}

type itemLabel struct {
	state     widget.Clickable
	hovering  bool
	selected  bool
	onClicked func()
}

func (l *itemLabel) update(gtx layout.Context) bool {
	for {
		event, ok := gtx.Event(
			pointer.Filter{Target: l, Kinds: pointer.Enter | pointer.Leave},
		)
		if !ok {
			break
		}

		switch event := event.(type) {
		case pointer.Event:
			switch event.Kind {
			case pointer.Enter:
				l.hovering = true
			case pointer.Leave:
				l.hovering = false
			case pointer.Cancel:
				l.hovering = false
			}
		}
	}

	if l.state.Clicked(gtx) && l.onClicked != nil {
		l.onClicked()
		return true
	}

	return false
}

func (l *itemLabel) Click() {
	l.state.Click()
}

func (l *itemLabel) Layout(gtx layout.Context, highlightColor color.NRGBA, w layout.Widget) layout.Dimensions {
	l.update(gtx)

	macro := op.Record(gtx.Ops)
	dims := layout.Background{}.Layout(gtx,
		func(gtx layout.Context) layout.Dimensions {
			if !l.selected && !l.hovering {
				return layout.Dimensions{Size: gtx.Constraints.Min}
			}

			var fill color.NRGBA
			if l.selected {
				fill = highlightColor
			} else if l.hovering {
				fill = adjustAlpha(highlightColor, 0x30)
			}

			rect := clip.Rect{
				Max: image.Point{X: gtx.Constraints.Max.X, Y: gtx.Constraints.Min.Y},
			}
			paint.FillShape(gtx.Ops, fill, rect.Op())
			return layout.Dimensions{Size: gtx.Constraints.Min}
		},
		func(gtx layout.Context) layout.Dimensions {
			return l.state.Layout(gtx, w)
		},
	)
	call := macro.Stop()

	defer pointer.PassOp{}.Push(gtx.Ops).Pop()
	defer clip.Rect(image.Rectangle{Max: dims.Size}).Push(gtx.Ops).Pop()
	event.Op(gtx.Ops, l)
	call.Add(gtx.Ops)

	return dims
}

func adjustAlpha(c color.NRGBA, alpha uint8) color.NRGBA {
	return color.NRGBA{
		R: c.R,
		G: c.G,
		B: c.B,
		A: alpha,
	}
}
