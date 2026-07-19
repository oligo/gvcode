package gvcode

import (
	"image"
	"strings"
	"testing"

	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"
	"github.com/oligo/gvcode/textview"
)

func BenchmarkDeleteBackwardLargeDocument(b *testing.B) {
	doc := strings.Repeat("func main() { println(\"hello\") }\n", 5000)
	editor := &Editor{}
	editor.WithOptions(WithTextSize(unit.Sp(14)))
	editor.SetText(doc)
	gtx := layout.Context{Constraints: layout.Exact(image.Pt(1200, 800))}
	editor.text.Layout(gtx, text.NewShaper())
	editor.SetCaret(editor.Len(), editor.Len())

	b.ResetTimer()
	for range b.N {
		editor.Delete(-1)
	}
}

func BenchmarkTextInputLargeDocument(b *testing.B) {
	doc := strings.Repeat("func main() { println(\"hello\") }\n", 5000)
	editor := &Editor{}
	editor.WithOptions(WithTextSize(unit.Sp(14)))
	editor.SetText(doc)
	textGtx := layout.Context{Constraints: layout.Exact(image.Pt(1200, 800))}
	shaper := text.NewShaper()
	editor.text.Layout(textGtx, shaper)
	editor.SetCaret(editor.Len(), editor.Len())

	router := new(input.Router)
	gtx := layout.Context{Ops: new(op.Ops), Source: router.Source()}
	editor.Update(gtx)
	router.Frame(new(op.Ops))
	router.Source().Execute(key.FocusCmd{Tag: editor})
	for {
		if _, ok := editor.Update(gtx); !ok {
			break
		}
	}

	b.ResetTimer()
	for range b.N {
		b.StopTimer()
		pos := editor.Len()
		router.Queue(key.EditEvent{Range: key.Range{Start: pos, End: pos}, Text: "a"})
		b.StartTimer()
		evt, ok := editor.Update(gtx)
		b.StopTimer()
		if !ok || !isChangeEvent(evt) {
			b.Fatalf("Update() = (%T, %v), want ChangeEvent", evt, ok)
		}

		for {
			if _, ok := editor.Update(gtx); !ok {
				break
			}
		}
		if _, ok := editor.undo(); !ok {
			b.Fatal("undo failed")
		}
		editor.text.Changed()
		editor.text.Layout(textGtx, shaper)
	}
}

func TestDeleteBackwardEmitsOneChangeEvent(t *testing.T) {
	editor := &Editor{}
	editor.SetText("abc")
	editor.SetCaret(editor.Len(), editor.Len())

	router := new(input.Router)
	gtx := layout.Context{Ops: new(op.Ops), Source: router.Source()}
	editor.Update(gtx)
	router.Frame(new(op.Ops))

	router.Source().Execute(key.FocusCmd{Tag: editor})
	router.Queue(key.Event{Name: key.NameDeleteBackward, State: key.Press})

	gtx.Source = router.Source()
	changes := 0
	for {
		evt, ok := editor.Update(gtx)
		if !ok {
			break
		}
		if _, ok := evt.(ChangeEvent); ok {
			changes++
		}
	}

	if changes != 1 {
		t.Fatalf("DeleteBackward emitted %d ChangeEvents, want 1", changes)
	}
	if got := editor.Len(); got != 2 {
		t.Fatalf("DeleteBackward left %d runes, want 2", got)
	}
}

func TestIsChangeEventAcceptsValueAndPointer(t *testing.T) {
	if !isChangeEvent(ChangeEvent{}) {
		t.Fatal("value ChangeEvent was not recognized")
	}
	if !isChangeEvent(&ChangeEvent{}) {
		t.Fatal("pointer ChangeEvent was not recognized")
	}
	if isChangeEvent(SelectEvent{}) {
		t.Fatal("SelectEvent was recognized as a ChangeEvent")
	}
}

func TestEditEventsEmitSeparateChangeEvents(t *testing.T) {
	editor := &Editor{}
	editor.SetText("")

	router := new(input.Router)
	gtx := layout.Context{Ops: new(op.Ops), Source: router.Source()}
	editor.Update(gtx)
	router.Frame(new(op.Ops))

	router.Source().Execute(key.FocusCmd{Tag: editor})
	router.Queue(
		key.EditEvent{Range: key.Range{Start: 0, End: 0}, Text: "a"},
		key.EditEvent{Range: key.Range{Start: 1, End: 1}, Text: "b"},
	)

	gtx.Source = router.Source()
	changes := 0
	for {
		evt, ok := editor.Update(gtx)
		if !ok {
			break
		}
		if _, ok := evt.(ChangeEvent); ok {
			changes++
		}
	}

	if changes != 2 {
		t.Fatalf("two EditEvents emitted %d ChangeEvents, want 2", changes)
	}
	if got := editor.Text(); got != "ab" {
		t.Fatalf("text = %q, want %q", got, "ab")
	}
}

func TestResetIMEClosesUndoGroup(t *testing.T) {
	editor := &Editor{}
	editor.SetText("")
	editor.ime.isComposing = true
	editor.buffer.GroupOp()
	editor.replace(0, 0, "中")

	editor.resetIME()
	editor.replace(1, 1, " ")

	if _, ok := editor.undo(); !ok {
		t.Fatal("undo failed")
	}
	if got := editor.Text(); got != "中" {
		t.Fatalf("undo after IME reset left %q, want %q", got, "中")
	}
}

func TestAutoInsertionTracksEditsBeforeClosingRune(t *testing.T) {
	editor := &Editor{}
	editor.WithOptions(WithTextSize(unit.Sp(14)))
	editor.SetText("")
	editor.text.Layout(layout.Context{Constraints: layout.Exact(image.Pt(800, 600))}, text.NewShaper())

	editor.onTextInput(key.EditEvent{Range: key.Range{Start: 0, End: 0}, Text: "("})
	editor.onTextInput(key.EditEvent{Range: key.Range{Start: 1, End: 1}, Text: "x"})
	editor.onTextInput(key.EditEvent{Range: key.Range{Start: 2, End: 2}, Text: ")"})

	if got := editor.Text(); got != "(x)" {
		t.Fatalf("text = %q, want %q", got, "(x)")
	}
	start, end := editor.Selection()
	if start != 3 || end != 3 {
		t.Fatalf("selection = (%d, %d), want (3, 3)", start, end)
	}
}

func TestOnDeleteBackward_Indentation(t *testing.T) {
	setup := func(input string, cursorPos, tabWidth int) *Editor {
		vw := textview.NewTextView()
		vw.TabWidth = tabWidth
		vw.TextSize = unit.Sp(14)
		vw.SetText(input)

		gtx := layout.Context{}
		shaper := text.NewShaper()
		vw.Layout(gtx, shaper)

		vw.SetCaret(cursorPos, cursorPos)

		return &Editor{
			text:           vw,
			buffer:         vw.Source(),
			scratch:        make([]byte, 0, 256),
			autoInsertions: make(map[int]rune),
		}
	}

	cases := []struct {
		name      string
		input     string
		cursorPos int
		tabWidth  int
		// wantStart and wantEnd are expected Selection() return values.
		// If wantStart == wantEnd, no selection expansion occurred.
		wantStart int
		wantEnd   int
	}{
		{
			name:      "cursor at position 0 returns early",
			input:     "test",
			cursorPos: 0,
			tabWidth:  4,
			wantStart: 0,
			wantEnd:   0,
		},
		{
			name:      "prev char is not space (letters), no-op",
			input:     "hello",
			cursorPos: 4,
			tabWidth:  4,
			wantStart: 4,
			wantEnd:   4,
		},
		{
			name:      "prev char is space but leading has text",
			input:     "text ",
			cursorPos: 5,
			tabWidth:  4,
			wantStart: 5,
			wantEnd:   5,
		},
		{
			name:      "4 spaces only, cursor at end",
			input:     "    ",
			cursorPos: 4,
			tabWidth:  4,
			wantStart: 4,
			wantEnd:   0,
		},
		{
			name:      "8 spaces, capped at tabWidth=4",
			input:     "        ",
			cursorPos: 8,
			tabWidth:  4,
			wantStart: 8,
			wantEnd:   4,
		},
		{
			name:      "3 spaces, less than tabWidth",
			input:     "   ",
			cursorPos: 3,
			tabWidth:  4,
			wantStart: 3,
			wantEnd:   0,
		},
		{
			name:      "tab then 2 spaces, only trailing spaces counted",
			input:     "\t  ",
			cursorPos: 3,
			tabWidth:  4,
			wantStart: 3,
			wantEnd:   1,
		},
		{
			name:      "spaces tab spaces, stops at tab",
			input:     "    \t  ",
			cursorPos: 7,
			tabWidth:  4,
			wantStart: 7,
			wantEnd:   5,
		},
		{
			name:      "tab only, no trailing spaces",
			input:     "\t",
			cursorPos: 1,
			tabWidth:  4,
			wantStart: 1,
			wantEnd:   1,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := setup(tc.input, tc.cursorPos, tc.tabWidth)
			e.onDeleteBackward()
			start, end := e.Selection()

			if start != tc.wantStart || end != tc.wantEnd {
				t.Errorf("Selection() = (%d, %d), want (%d, %d)",
					start, end, tc.wantStart, tc.wantEnd)
			}
		})
	}
}
