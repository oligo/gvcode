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
