package main

import (
	"fmt"
	"image"
	"image/color"
	"log"
	_ "net/http/pprof" // This line registers the pprof handlers
	"os"
	"regexp"
	"strings"

	"gioui.org/app"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/oligo/gvcode"
	"github.com/oligo/gvcode/addons/completion"
	"github.com/oligo/gvcode/addons/diff"
	gvcolor "github.com/oligo/gvcode/color"
	"github.com/oligo/gvcode/gutter"
	"github.com/oligo/gvcode/gutter/providers"

	// "github.com/oligo/gvcode/textstyle/decoration"
	"github.com/oligo/gvcode/textstyle/syntax"
	wg "github.com/oligo/gvcode/widget"
)

type (
	C = layout.Context
	D = layout.Dimensions
)

type EditorApp struct {
	window       *app.Window
	th           *material.Theme
	state        *gvcode.Editor
	xScroll      widget.Scrollbar
	yScroll      widget.Scrollbar
	diffProvider *providers.VCSDiffProvider
	diffPopup    *diff.DiffPopup
	differ       *diff.GitDiff
}

const (
	syntaxPattern = "package|import|type|func|struct|for|var|switch|case|if|else"
)

func (ed *EditorApp) run() error {

	var ops op.Ops
	for {
		e := ed.window.Event()

		switch e := e.(type) {
		case app.DestroyEvent:
			return e.Err
		case app.FrameEvent:
			gtx := app.NewContext(&ops, e)
			ed.layout(gtx, ed.th)
			e.Frame(gtx.Ops)
		}
	}
}

func (ed *EditorApp) layout(gtx C, th *material.Theme) D {
	for {
		evt, ok := ed.state.Update(gtx)
		if !ok {
			break
		}

		switch evt.(type) {
		case gvcode.ChangeEvent:
			tokens := HightlightTextByPattern(ed.state.Text(), syntaxPattern)
			ed.state.SetSyntaxTokens(tokens...)
			// May also need to sync the editor content to the completion engine before
			// calling OnTextEdit.
			ed.state.OnTextEdit()
			// Parse git diff for the current file and update the diff provider
			if hunks := ed.differ.ParseDiff([]byte(ed.state.Text())); len(hunks) > 0 {
				ed.diffProvider.UpdateDiff(hunks)
			}

		case gvcode.GutterEventWrapper:
			wrapper := evt.(gvcode.GutterEventWrapper)
			if click, ok := wrapper.Event.(gutter.GutterClickEvent); ok {
				if click.ProviderID == providers.DiffProviderID {
					hunk := ed.diffProvider.GetHunk(click.Line)
					if hunk != nil {
						ed.diffPopup = diff.NewDiffPopup(hunk, th.TextSize, click.Line)
					}
				}
			}

		}
	}

	xScrollDist := ed.xScroll.ScrollDistance()
	yScrollDist := ed.yScroll.ScrollDistance()
	if xScrollDist != 0.0 || yScrollDist != 0.0 {
		ed.state.Scroll(gtx, xScrollDist, yScrollDist)
	}

	scrollIndicatorColor := gvcolor.MakeColor(th.Fg).MulAlpha(0x30)

	return layout.Flex{
		Axis: layout.Vertical,
	}.Layout(gtx,
		layout.Flexed(1, func(gtx C) D {
			return layout.Inset{
				Top:   unit.Dp(2),
				Left:  unit.Dp(1),
				Right: unit.Dp(1),
			}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{
					Axis: layout.Horizontal,
				}.Layout(gtx,
					layout.Flexed(1.0, func(gtx layout.Context) layout.Dimensions {
						ed.state.WithOptions(
							//gvcode.WithFont(font.Font{Typeface: "monospace", Weight: font.SemiBold}),
							gvcode.WithTextSize(unit.Sp(12)),
							gvcode.WithLineHeight(0, 1.5),
						)

						dims := ed.state.Layout(gtx, th.Shaper)

						if ed.diffPopup != nil {
							if evt, ok := ed.diffPopup.Update(gtx); ok {
								switch evt.Action {
								case diff.ActionRevert:
									//app.revertHunk(evt.Hunk)
								case diff.ActionStage:
									//app.stageHunk(evt.Hunk)
								case diff.ActionClose:
									ed.diffPopup = nil
								}
							}

							if ed.diffPopup != nil {
								// Calculate popup position based on the clicked line
								// Position below the line by adding line height
								_, pos := ed.state.ConvertPos(ed.diffPopup.PopupLine(), 0)
								position := image.Point{X: pos.Round().X, Y: pos.Round().Y}
								log.Println("diff popup position: ", position)
								ed.state.PaintOverlay(gtx, position, func(gtx layout.Context) layout.Dimensions {
									return ed.diffPopup.Layout(gtx, th)
								})
							}
						}

						macro := op.Record(gtx.Ops)
						scrollbarDims := func(gtx C) D {
							return layout.Inset{
								Left: gtx.Metric.PxToDp(ed.state.GutterWidth()),
							}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								minX, maxX, _, _ := ed.state.ScrollRatio()
								bar := makeScrollbar(th, &ed.xScroll, scrollIndicatorColor.NRGBA())
								return bar.Layout(gtx, layout.Horizontal, minX, maxX)
							})
						}(gtx)

						scrollbarOp := macro.Stop()
						defer op.Offset(image.Point{Y: dims.Size.Y - scrollbarDims.Size.Y}).Push(gtx.Ops).Pop()
						scrollbarOp.Add(gtx.Ops)

						return dims
					}),

					layout.Rigid(func(gtx C) D {
						_, _, minY, maxY := ed.state.ScrollRatio()
						bar := makeScrollbar(th, &ed.yScroll, scrollIndicatorColor.NRGBA())
						return bar.Layout(gtx, layout.Vertical, minY, maxY)
					}),
				)

			})
		}),
		layout.Rigid(func(gtx C) D {
			return layout.Inset{
				Right:  unit.Dp(8),
				Top:    unit.Dp(2),
				Bottom: unit.Dp(2),
			}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				line, col := ed.state.CaretPos()
				lb := material.Label(th, th.TextSize*0.7, fmt.Sprintf("Line:%d, Col:%d", line+1, col+1))
				lb.Alignment = text.End
				lb.Color = ed.state.ColorPalette().Foreground.NRGBA()
				return lb.Layout(gtx)
			})
		}),
	)

}

func makeScrollbar(th *material.Theme, scroll *widget.Scrollbar, color color.NRGBA) material.ScrollbarStyle {
	bar := material.Scrollbar(th, scroll)
	bar.Indicator.Color = color
	bar.Indicator.CornerRadius = unit.Dp(0)
	bar.Indicator.MinorWidth = unit.Dp(12)
	bar.Track.MajorPadding = unit.Dp(0)
	bar.Track.MinorPadding = unit.Dp(1)
	return bar
}

func main() {
	log.SetFlags(log.Flags() | log.Lshortfile)
	th := material.NewTheme()

	editorApp := EditorApp{
		window: &app.Window{},
		th:     th,
	}
	editorApp.window.Option(app.Title("gvcode demo"))

	gvcode.SetDebug(false)
	editorApp.state = wg.NewEditor(th)

	thisFile, _ := os.ReadFile("./main.go")
	editorApp.state.SetText(string(thisFile))
	editorApp.differ = diff.NewGitDiff("./main.go")

	// Setting up auto-completion.
	cm := &completion.DefaultCompletion{Editor: editorApp.state}

	// set popup widget to let user navigate the candidates.
	popup := completion.NewCompletionPopup(editorApp.state, cm)
	popup.Theme = th
	popup.TextSize = unit.Sp(12)

	cm.AddCompletor(&goCompletor{editor: editorApp.state}, popup)

	// color scheme
	colorScheme := syntax.ColorScheme{}
	colorScheme.Foreground = gvcolor.MakeColor(th.Fg)
	colorScheme.SelectColor = gvcolor.MakeColor(th.ContrastBg).MulAlpha(0x60)
	colorScheme.LineColor = gvcolor.MakeColor(th.ContrastBg).MulAlpha(0x30)
	colorScheme.LineNumberColor = gvcolor.MakeColor(th.ContrastBg).MulAlpha(0xb6)
	keywordColor, _ := gvcolor.Hex2Color("#AF00DB")
	colorScheme.AddStyle("keyword", syntax.Underline, keywordColor, gvcolor.Color{})

	editorApp.state.WithOptions(
		gvcode.WrapLine(true),
		gvcode.WithAutoCompletion(cm),
		gvcode.WithColorScheme(colorScheme),
		gvcode.WithCornerRadius(unit.Dp(4)),
	)
	editorApp.state.WithOptions(gvcode.WithDefaultGutters(), gvcode.WithGutterGap(unit.Dp(12)))
	editorApp.diffProvider = providers.NewVCSDiffProvider()
	editorApp.state.WithOptions(gvcode.WithGutter(editorApp.diffProvider))

	// Parse git diff for the current file and update the diff provider
	if hunks := editorApp.differ.ParseDiff(thisFile); len(hunks) > 0 {
		editorApp.diffProvider.UpdateDiff(hunks)
	}

	tokens := HightlightTextByPattern(editorApp.state.Text(), syntaxPattern)
	editorApp.state.SetSyntaxTokens(tokens...)

	go func() {
		err := editorApp.run()
		if err != nil {
			os.Exit(1)
		}

		os.Exit(0)
	}()

	app.Main()

}

func HightlightTextByPattern(text string, pattern string) []syntax.Token {
	var tokens []syntax.Token

	re := regexp.MustCompile(pattern)
	matches := re.FindAllIndex([]byte(text), -1)
	for _, match := range matches {
		tokens = append(tokens, syntax.Token{
			Start: match[0],
			End:   match[1],
			Scope: "keyword",
		})
	}

	return tokens
}

var golangKeywords = []string{
	"break",
	"default",
	"func",
	"interface",
	"select",
	"case",
	"defer", "go", "map", "struct",
	"chan", "else", "goto", "package", "switch",
	"const", "fallthrough", "if", "range", "type",
	"continue", "for", "import", "return", "var",
}

type goCompletor struct {
	editor *gvcode.Editor
}

func isSymbolSeperator(ch rune) bool {
	if (ch >= 'a' && ch <= 'z') ||
		(ch >= 'A' && ch <= 'Z') ||
		(ch >= '0' && ch <= '9') ||
		ch == '_' {
		return false
	}

	return true
}

func (c *goCompletor) Trigger() gvcode.Trigger {
	return gvcode.Trigger{
		Characters: []string{"."},
		KeyBinding: struct {
			Name      key.Name
			Modifiers key.Modifiers
		}{
			Name: "P", Modifiers: key.ModShortcut,
		},
	}
}

func (c *goCompletor) Suggest(ctx gvcode.CompletionContext) []gvcode.CompletionCandidate {
	prefix := c.editor.ReadUntil(-1, isSymbolSeperator)
	candicates := make([]gvcode.CompletionCandidate, 0)
	for _, kw := range golangKeywords {
		if strings.Contains(kw, prefix) {
			candicates = append(candicates, gvcode.CompletionCandidate{
				Label: kw,
				TextEdit: gvcode.TextEdit{
					NewText: kw,
					// EditRange can be omitted to let the completion engine determine it.
					// EditRange: gvcode.EditRange{
					// 	Start: gvcode.Position{Runes: ctx.Position.Runes - utf8.RuneCountInString(prefix)},
					// 	End:   gvcode.Position{Runes: ctx.Position.Runes},
					// },
				},
				Description: kw,
				Kind:        "text",
				TextFormat:  "Snippet",
			})
		}
	}

	return candicates
}

func (c *goCompletor) FilterAndRank(pattern string, candidates []gvcode.CompletionCandidate) []gvcode.CompletionCandidate {
	return candidates
}
