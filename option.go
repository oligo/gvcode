package gvcode

import (
	"gioui.org/font"
	"gioui.org/text"
	"gioui.org/unit"
	"github.com/oligo/gvcode/gutter"
	"github.com/oligo/gvcode/gutter/providers"
	"github.com/oligo/gvcode/textstyle/syntax"
)

// EditorOption defines a function to configure the editor.
type EditorOption func(*Editor)

// WithOptions applies various options to configure the editor.
func (e *Editor) WithOptions(opts ...EditorOption) {
	for _, opt := range opts {
		opt(e)
	}
}

// Set font to use for the editor.
func WithFont(font font.Font) EditorOption {
	return func(e *Editor) {
		e.initBuffer()
		e.text.Font = font
	}
}

// Set size of the text.
func WithTextSize(textSize unit.Sp) EditorOption {
	return func(e *Editor) {
		e.initBuffer()
		e.text.TextSize = textSize
	}
}

func WithTextAlignment(align text.Alignment) EditorOption {
	return func(e *Editor) {
		e.initBuffer()
		e.text.Alignment = align
	}
}

func WithLineHeight(lineHeight unit.Sp, lineHeightScale float32) EditorOption {
	return func(e *Editor) {
		e.initBuffer()
		e.text.LineHeight = lineHeight
		e.text.LineHeightScale = lineHeightScale
	}
}

// Set a radis value for the corners of selection polygons or borders of other shapes.
func WithCornerRadius(radius unit.Dp) EditorOption {
	return func(e *Editor) {
		e.initBuffer()
		e.text.CornerRadius = radius
	}
}

// WithTabWidth set how many spaces to represent a tab character. In the case of
// soft tab, this determines the number of space characters to insert into the editor.
// While for hard tab, this controls the maximum width of the 'tab' glyph to expand to.
func WithTabWidth(tabWidth int) EditorOption {
	return func(e *Editor) {
		e.initBuffer()
		e.text.TabWidth = tabWidth
	}
}

// WithSoftTab controls the behaviour when user try to insert a Tab character.
// If set to true, the editor will insert the amount of space characters specified by
// TabWidth, else the editor insert a \t character.
func WithSoftTab(enabled bool) EditorOption {
	return func(e *Editor) {
		e.initBuffer()
		e.text.SoftTab = enabled
	}
}

// WithWordSeperators configures a set of characters that will be used as word separators
// when doing word related operations, like navigating or deleting by word.
func WithWordSeperators(seperators string) EditorOption {
	return func(e *Editor) {
		e.initBuffer()
		e.text.WordSeperators = seperators
	}
}

// WithQuotePairs configures a set of quote pairs that can be auto-completed when the left
// half is entered.
func WithQuotePairs(quotePairs map[rune]rune) EditorOption {
	return func(e *Editor) {
		e.initBuffer()
		e.text.BracketsQuotes.SetQuotes(quotePairs)
	}
}

// WithBracketPairs configures a set of bracket pairs that can be auto-completed when the left
// half is entered.
func WithBracketPairs(bracketPairs map[rune]rune) EditorOption {
	return func(e *Editor) {
		e.initBuffer()
		e.text.BracketsQuotes.SetBrackets(bracketPairs)
	}
}

// ReadOnlyMode controls whether the contents of the editor can be altered by
// user interaction. If set to true, the editor will allow selecting text
// and copying it interactively, but not modifying it.
func ReadOnlyMode(enabled bool) EditorOption {
	return func(e *Editor) {
		e.initBuffer()
		if enabled {
			e.setMode(ModeReadOnly)
		} else {
			e.setMode(ModeNormal)
		}
	}
}

// WrapLine configures whether the displayed text will be broken into lines or not.
func WrapLine(enabled bool) EditorOption {
	return func(e *Editor) {
		e.initBuffer()
		e.text.SetWrapLine(enabled)
	}
}

// Deprecated: Please use [WithGutter] or [WithDefaultGutters]
// WithLineNumber configures whether to show line number or not.
func WithLineNumber(enabled bool) EditorOption {
	return func(e *Editor) {
		e.initBuffer()
		// nothing to do.
	}
}

func WithGutterGap(gap unit.Dp) EditorOption {
	return func(e *Editor) {
		e.initBuffer()
		e.gutterGap = gap
	}
}

func WithAutoCompletion(completor Completion) EditorOption {
	return func(e *Editor) {
		e.initBuffer()
		e.completor = completor
	}
}

// WithColorScheme configures the color scheme used for styling syntax tokens.
// Syntax highlight implementations should align with the token types used in the
// ColorScheme.
func WithColorScheme(scheme syntax.ColorScheme) EditorOption {
	return func(e *Editor) {
		e.initBuffer()
		e.colorPalette = &scheme.ColorPalette
		e.text.SetColorScheme(&scheme)
	}
}

// BeforePasteHook defines a hook to be called before pasting text to transform the text.
type BeforePasteHook func(text string) string

func AddBeforePasteHook(hook BeforePasteHook) EditorOption {
	return func(ed *Editor) {
		ed.onPaste = hook
	}
}

// WithGutter adds a gutter provider to the editor. Creates a gutter manager if needed.
// Multiple providers can be added by calling this function multiple times.
func WithGutter(provider gutter.GutterProvider) EditorOption {
	return func(e *Editor) {
		e.initBuffer()
		if e.gutterManager == nil {
			e.gutterManager = gutter.NewManager()
		}
		e.gutterManager.Register(provider)
	}
}

// WithDefaultGutters enables line numbers via the new gutter system.
// This is the recommended way to enable line numbers for new code.
func WithDefaultGutters() EditorOption {
	return func(e *Editor) {
		e.initBuffer()
		if e.gutterManager == nil {
			e.gutterManager = gutter.NewManager()
		}
		e.gutterManager.Register(providers.NewLineNumberProvider())
	}
}
