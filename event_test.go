package gvcode

import (
	"testing"

	"gioui.org/layout"
	"gioui.org/text"
	"gioui.org/unit"
	"github.com/oligo/gvcode/textview"
)

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
