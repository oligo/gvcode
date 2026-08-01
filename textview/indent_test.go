package textview

import (
	"fmt"
	"testing"

	"gioui.org/layout"
	"gioui.org/text"
	"gioui.org/unit"
	"github.com/oligo/gvcode/internal/buffer"
)

func TestIndentLines(t *testing.T) {
	setup := func(input string, selection []int) *TextView {
		vw := NewTextView()
		vw.TabWidth = 4
		vw.SoftTab = false
		vw.TextSize = unit.Sp(14)
		vw.SetText(input)

		gtx := layout.Context{}
		shaper := text.NewShaper()
		vw.Layout(gtx, shaper)

		vw.SetCaret(selection[0], selection[1])
		return vw
	}

	cases := []struct {
		input     string
		selection []int
		backward  bool
		want      string
		wantMoves int
	}{
		{
			input:     "abc",
			selection: []int{1, 1},
			backward:  false,
			want:      "a\tbc",
			wantMoves: 1,
		},
		{
			input:     "abc",
			selection: []int{1, 1},
			backward:  true,
			want:      "abc",
			wantMoves: 0,
		},
		{
			input:     "abc",
			selection: []int{1, 2},
			backward:  false,
			want:      "a\tc",
			wantMoves: 1,
		},
		// multiple lines
		{
			input:     "\tabc\n\tdef",
			selection: []int{0, 9},
			backward:  false,
			want:      "\t\tabc\n\t\tdef",
			wantMoves: 11,
		},
		{
			input:     "\tabc\n\tdef",
			selection: []int{0, 9},
			backward:  true,
			want:      "abc\ndef",
			wantMoves: 7,
		},
		{
			input:     "abc\n\tdef",
			selection: []int{0, 8},
			backward:  true,
			want:      "abc\ndef",
			wantMoves: 7,
		},
		{
			input:     "  abc\n\tdef",
			selection: []int{0, 10},
			backward:  true,
			want:      "abc\ndef",
			wantMoves: 7,
		},
		{
			input:     "  abc\n\tdef",
			selection: []int{0, 10},
			backward:  false,
			want:      "\t  abc\n\t\tdef",
			wantMoves: 12,
		},
	}

	for i, tc := range cases {
		t.Run(fmt.Sprintf("%d: %s", i, tc.input), func(t *testing.T) {
			text := setup(tc.input, tc.selection)
			actual := text.IndentLines(tc.backward)
			reader := buffer.NewReader(text.src)
			finalContent := reader.ReadAll(nil)
			if actual != tc.wantMoves || string(finalContent) != tc.want {
				t.Logf("want content: %q, actual content: %q, want moves: %d, actual moves: %d", tc.want, string(finalContent), tc.wantMoves, actual)
				t.Fail()
			}
		})
	}

}

func TestDedentLine(t *testing.T) {
	text := NewTextView()
	text.TabWidth = 4

	cases := []struct {
		input string
		want  string
	}{
		{
			input: "abc",
			want:  "abc",
		},
		{
			input: "\t\tabc",
			want:  "\tabc",
		},

		{
			input: "\t    abc",
			want:  "\tabc",
		},

		{
			input: "\t      abc",
			want:  "\t    abc",
		},
		{
			input: "    abc",
			want:  "abc",
		},
		{
			input: "   abc",
			want:  "abc",
		},
	}

	for i, tc := range cases {
		t.Run(fmt.Sprintf("%d: %s", i, tc.input), func(t *testing.T) {
			actual := text.dedentLine(tc.input)
			if actual != tc.want {
				t.Fail()
			}
		})
	}

}

func TestIndentOnBreak(t *testing.T) {
	setup := func(input string, selection int) *TextView {
		vw := NewTextView()
		vw.TabWidth = 4
		vw.SoftTab = false
		vw.TextSize = unit.Sp(14)
		vw.BracketsQuotes.SetBrackets(map[rune]rune{
			'(': ')',
			'{': '}',
			'[': ']',
			'$': '$',
		})
		vw.SetText(input)

		gtx := layout.Context{}
		shaper := text.NewShaper()
		vw.Layout(gtx, shaper)

		vw.SetCaret(selection, selection)
		return vw
	}

	cases := []struct {
		input     string
		selection int
		want      string
		wantMoves int
	}{
		{
			input:     "abc",
			selection: 3,
			want:      "abc\n",
			wantMoves: 1,
		},
		{
			input:     "\tabcde",
			selection: 4,
			want:      "\tabc\n\tde",
			wantMoves: 2,
		},
		{
			input:     "abc{\n}",
			selection: 4,
			want:      "abc{\n\t\n}",
			wantMoves: 2,
		},

		{
			input:     "abc{de\n}",
			selection: 6,
			want:      "abc{de\n\t\n}",
			wantMoves: 2,
		},
		{
			input:     "abc{}",
			selection: 4,
			want:      "abc{\n\t\n}",
			wantMoves: 3,
		},
		// check self-paired brackets
		{
			input:     "abc$$",
			selection: 4,
			want:      "abc$\n\t\n$",
			wantMoves: 3,
		},
		{
			input:     "\tabc{\n\n}",
			selection: 6,
			want:      "\tabc{\n\n\n}",
			wantMoves: 1,
		},
	}

	for i, tc := range cases {
		t.Run(fmt.Sprintf("%d: %s", i, tc.input), func(t *testing.T) {
			text := setup(tc.input, tc.selection)
			actual := text.IndentOnBreak("\n")
			reader := buffer.NewReader(text.src)
			finalContent := reader.ReadAll(nil)
			if actual != tc.wantMoves || string(finalContent) != tc.want {
				t.Logf("want content: %q, actual content: %q, want moves: %d, actual moves: %d", tc.want, string(finalContent), tc.wantMoves, actual)
				t.Fail()
			}
		})
	}

}
