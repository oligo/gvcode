package diff

import (
	"image"
	"image/color"

	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	gvcolor "github.com/oligo/gvcode/color"
	"github.com/oligo/gvcode/gutter/providers"
)

// PopupAction represents an action that can be performed on a diff hunk.
type PopupAction int

const (
	// ActionNone indicates no action was taken.
	ActionNone PopupAction = iota
	// DiffActionRevert indicates the user wants to revert the changes.
	ActionRevert
	// ActionStage indicates the user wants to stage the changes.
	ActionStage
	// ActionClose indicates the user closed the popup.
	ActionClose
)

// PopupEvent is emitted when the user interacts with the diff popup.
type PopupEvent struct {
	Action PopupAction
	Hunk   *providers.DiffHunk
}

// PopupColors defines the color scheme for the diff popup.
type PopupColors struct {
	// Background is the popup background color.
	Background gvcolor.Color
	// Border is the popup border color.
	Border gvcolor.Color
	// AddedBackground is the background for added lines.
	AddedBackground gvcolor.Color
	// AddedText is the text color for added lines.
	AddedText gvcolor.Color
	// DeletedBackground is the background for deleted lines.
	DeletedBackground gvcolor.Color
	// DeletedText is the text color for deleted lines.
	DeletedText gvcolor.Color
	// ButtonBackground is the button background color.
	ButtonBackground gvcolor.Color
	// ButtonText is the button text color.
	ButtonText gvcolor.Color
	// HeaderText is the header text color.
	HeaderText gvcolor.Color
}

// DefaultPopupColors returns the default color scheme for the diff popup.
func DefaultPopupColors() PopupColors {
	background, _ := gvcolor.Hex2Color("#1e1e1e00")
	border, _ := gvcolor.Hex2Color("#3C3C3C")
	addedBg, _ := gvcolor.Hex2Color("#234D2C")
	addedText, _ := gvcolor.Hex2Color("#6AD57F")
	deletedBg, _ := gvcolor.Hex2Color("#4D2326")
	deletedText, _ := gvcolor.Hex2Color("#E56B6B")
	buttonBg, _ := gvcolor.Hex2Color("#2D2D2D")
	buttonText, _ := gvcolor.Hex2Color("#CCCCCC")
	headerText, _ := gvcolor.Hex2Color("#AAAAAA")

	return PopupColors{
		Background:        background,
		Border:            border,
		AddedBackground:   addedBg,
		AddedText:         addedText,
		DeletedBackground: deletedBg,
		DeletedText:       deletedText,
		ButtonBackground:  buttonBg,
		ButtonText:        buttonText,
		HeaderText:        headerText,
	}
}

// DiffPopup displays a diff hunk with actions to revert or stage changes.
type DiffPopup struct {
	// Hunk is the diff hunk to display.
	Hunk *providers.DiffHunk

	// Colors defines the color scheme.
	Colors PopupColors

	// TextSize is the size of the text.
	TextSize unit.Sp

	// ShowStageButton controls whether to show the stage button.
	ShowStageButton bool

	// MaxHeight is the maximum height of the popup.
	MaxHeight unit.Dp

	// popupLine is the line where the diff popup should appear
	popupLine int

	// buttons
	revertBtn widget.Clickable
	stageBtn  widget.Clickable
	closeBtn  widget.Clickable

	// scroll state for diff content
	diffList widget.List
}

// NewDiffPopup creates a new diff popup for the given hunk.
func NewDiffPopup(hunk *providers.DiffHunk, textSize unit.Sp, popupLine int) *DiffPopup {
	return &DiffPopup{
		Hunk:            hunk,
		Colors:          DefaultPopupColors(),
		TextSize:        textSize,
		ShowStageButton: true,
		popupLine:       popupLine,
		MaxHeight:       unit.Dp(300),
		diffList: widget.List{
			List: layout.List{
				Axis: layout.Vertical,
			},
		},
	}
}

func (p *DiffPopup) PopupLine() int {
	return p.popupLine
}

// Update processes events and returns any action taken.
func (p *DiffPopup) Update(gtx layout.Context) (PopupEvent, bool) {
	// Handle keyboard events
	for {
		event, ok := gtx.Event(key.Filter{Name: key.NameEscape})
		if !ok {
			break
		}
		if ev, ok := event.(key.Event); ok && ev.State == key.Press {
			return PopupEvent{Action: ActionClose, Hunk: p.Hunk}, true
		}
	}

	// Handle button clicks
	if p.revertBtn.Clicked(gtx) {
		return PopupEvent{Action: ActionRevert, Hunk: p.Hunk}, true
	}
	if p.stageBtn.Clicked(gtx) {
		return PopupEvent{Action: ActionStage, Hunk: p.Hunk}, true
	}
	if p.closeBtn.Clicked(gtx) {
		return PopupEvent{Action: ActionClose, Hunk: p.Hunk}, true
	}

	return PopupEvent{}, false
}

// Layout renders the diff popup.
func (p *DiffPopup) Layout(gtx layout.Context, th *material.Theme) layout.Dimensions {
	if p.Hunk == nil {
		return layout.Dimensions{}
	}

	gtx.Constraints.Max.Y = gtx.Dp(p.MaxHeight)
	gtx.Constraints.Min = image.Point{}

	macro := op.Record(gtx.Ops)
	dims := layout.Flex{
		Axis: layout.Vertical,
	}.Layout(gtx,
		// Header with buttons
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return p.layoutHeader(gtx, th)
		}),
		// Separator
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return p.layoutSeparator(gtx)
		}),
		// Diff content
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return p.layoutDiffContent(gtx, th)
		}),
	)
	callOp := macro.Stop()

	defer clip.Rect(image.Rectangle{Max: dims.Size}).Push(gtx.Ops).Pop()
	paint.ColorOp{Color: th.ContrastBg}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	callOp.Add(gtx.Ops)

	return dims
}

func (p *DiffPopup) layoutHeader(gtx layout.Context, th *material.Theme) layout.Dimensions {
	inset := layout.Inset{
		Top:    unit.Dp(8),
		Bottom: unit.Dp(8),
		Left:   unit.Dp(12),
		Right:  unit.Dp(12),
	}

	return inset.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{
			Axis:      layout.Horizontal,
			Alignment: layout.Middle,
			Spacing:   layout.SpaceBetween,
		}.Layout(gtx,
			// Title
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return p.layoutTitle(gtx, th)
			}),
			// Buttons
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return p.layoutButtons(gtx, th)
			}),
		)
	})
}

func (p *DiffPopup) layoutTitle(gtx layout.Context, th *material.Theme) layout.Dimensions {
	var title string
	switch p.Hunk.Type {
	case providers.DiffAdded:
		title = "Added Lines"
	case providers.DiffModified:
		title = "Modified Lines"
	case providers.DiffDeleted:
		title = "Deleted Lines"
	}

	label := material.Label(th, p.TextSize, title)
	label.Color = p.Colors.HeaderText.NRGBA()
	return label.Layout(gtx)
}

func (p *DiffPopup) layoutButtons(gtx layout.Context, th *material.Theme) layout.Dimensions {
	return layout.Flex{
		Axis:    layout.Horizontal,
		Spacing: layout.SpaceStart,
	}.Layout(gtx,
		// Revert button
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return p.layoutButton(gtx, th, &p.revertBtn, "Revert")
		}),
		// Stage button (optional)
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if !p.ShowStageButton {
				return layout.Dimensions{}
			}
			return layout.Inset{Left: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return p.layoutButton(gtx, th, &p.stageBtn, "Stage")
			})
		}),
		// Close button
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Left: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return p.layoutButton(gtx, th, &p.closeBtn, "✕")
			})
		}),
	)
}

func (p *DiffPopup) layoutButton(gtx layout.Context, th *material.Theme, btn *widget.Clickable, labelText string) layout.Dimensions {
	return btn.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Background{}.Layout(gtx,
			func(gtx layout.Context) layout.Dimensions {
				rr := gtx.Dp(unit.Dp(4))
				rect := clip.UniformRRect(image.Rectangle{Max: gtx.Constraints.Min}, rr)
				paint.FillShape(gtx.Ops, p.Colors.ButtonBackground.NRGBA(), rect.Op(gtx.Ops))
				return layout.Dimensions{Size: gtx.Constraints.Min}
			},
			func(gtx layout.Context) layout.Dimensions {
				return layout.UniformInset(unit.Dp(6)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					label := material.Label(th, p.TextSize-1, labelText)
					label.Color = p.Colors.ButtonText.NRGBA()
					return label.Layout(gtx)
				})
			},
		)
	})
}

func (p *DiffPopup) layoutSeparator(gtx layout.Context) layout.Dimensions {
	height := gtx.Dp(unit.Dp(1))
	rect := image.Rectangle{
		Max: image.Point{X: gtx.Constraints.Max.X, Y: height},
	}

	stack := clip.Rect(rect).Push(gtx.Ops)
	paint.ColorOp{Color: p.Colors.Border.NRGBA()}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	stack.Pop()

	return layout.Dimensions{Size: rect.Max}
}

type diffLine struct {
	text  string
	isOld bool
	isNew bool
}

func (p *DiffPopup) layoutDiffContent(gtx layout.Context, th *material.Theme) layout.Dimensions {
	var lines []diffLine

	switch p.Hunk.Type {
	case providers.DiffAdded:
		for _, line := range p.Hunk.NewLines {
			lines = append(lines, diffLine{text: "+ " + line, isNew: true})
		}

	case providers.DiffDeleted:
		for _, line := range p.Hunk.OldLines {
			lines = append(lines, diffLine{text: "- " + line, isOld: true})
		}

	case providers.DiffModified:
		for _, line := range p.Hunk.OldLines {
			lines = append(lines, diffLine{text: "- " + line, isOld: true})
		}
		for _, line := range p.Hunk.NewLines {
			lines = append(lines, diffLine{text: "+ " + line, isNew: true})
		}
	}

	if len(lines) == 0 {
		return layout.Dimensions{}
	}

	return p.diffList.Layout(gtx, len(lines), func(gtx layout.Context, index int) layout.Dimensions {
		line := lines[index]
		return p.layoutDiffLine(gtx, th, line.text, line.isOld, line.isNew)
	})
}

func (p *DiffPopup) layoutDiffLine(gtx layout.Context, th *material.Theme, lineText string, isOld, isNew bool) layout.Dimensions {
	// Determine colors
	var bgColor gvcolor.Color
	var textColor color.NRGBA

	if isOld {
		bgColor = p.Colors.DeletedBackground
		textColor = p.Colors.DeletedText.NRGBA()
	} else if isNew {
		bgColor = p.Colors.AddedBackground
		textColor = p.Colors.AddedText.NRGBA()
	} else {
		textColor = p.Colors.HeaderText.NRGBA()
	}

	return layout.Background{}.Layout(gtx,
		func(gtx layout.Context) layout.Dimensions {
			if bgColor.IsSet() {
				rect := image.Rectangle{Max: gtx.Constraints.Min}
				paint.FillShape(gtx.Ops, bgColor.NRGBA(), clip.Rect(rect).Op())
			}
			return layout.Dimensions{Size: gtx.Constraints.Min}
		},
		func(gtx layout.Context) layout.Dimensions {
			inset := layout.Inset{
				Top:    unit.Dp(2),
				Bottom: unit.Dp(2),
				Left:   unit.Dp(12),
				Right:  unit.Dp(12),
			}
			return inset.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				label := material.Label(th, p.TextSize, lineText)
				label.Color = textColor
				return label.Layout(gtx)
			})
		},
	)
}
