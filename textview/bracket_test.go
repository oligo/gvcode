package textview

import (
	"fmt"
	"testing"

	"gioui.org/layout"
	"gioui.org/text"
)

func TestNearestMatchingBrackets(t *testing.T) {
	view := NewTextView()
	gtx := layout.Context{}
	shaper := text.NewShaper()

	setup := func(input string, caret int) func() {
		return func() {
			view.SetText(input)
			view.Layout(gtx, shaper)
			view.SetCaret(caret, caret)
		}
	}

	cases := []struct {
		setup func()
		want  []int
	}{
		{
			setup: setup("{abc}", 0),
			want:  []int{0, 4},
		},

		{
			setup: setup("{abc}", 4),
			want:  []int{0, 4},
		},

		{
			setup: setup("{abc}", 1),
			want:  []int{0, 4},
		},

		{
			setup: setup("{a[b]c}", 0),
			want:  []int{0, 6},
		},

		{
			setup: setup("{a[b]c}", 2),
			want:  []int{2, 4},
		},
		{
			setup: setup("{a[b]c}", 3),
			want:  []int{2, 4},
		},
		{
			setup: setup("{a[b]cde}", 6),
			want:  []int{0, 8},
		},
		{
			setup: setup("{ab)c}", 3),
			want:  []int{-1, 3},
		},
		{
			setup: setup("{ab(c}", 3),
			want:  []int{3, -1},
		},
	}

	for i, tc := range cases {
		t.Run(fmt.Sprintf("case %d", i), func(t *testing.T) {
			tc.setup()
			left, right := view.NearestMatchingBrackets()
			if left != tc.want[0] || right != tc.want[1] {
				t.Logf("expected [%d, %d], got [%d, %d]", tc.want[0], tc.want[1], left, right)
				t.Fail()
			}
		})
	}

	// Test self-paired brackets (e.g. $$).
	selfPairedCases := []struct {
		input string
		caret int
		want  []int
	}{
		// $$ with caret between them — the right $ is the closing bracket.
		{"$$", 1, []int{0, 1}},
		// $$ with caret before both — finds the pair to the right.
		{"$$", 0, []int{0, 1}},
		// $$ with caret after both — finds the pair to the left.
		{"$$", 2, []int{0, 1}},
		// $abc$ with caret inside content — finds enclosing pair.
		{"$abc$", 0, []int{0, 4}},
		{"$abc$", 2, []int{0, 4}},
	}

	for i, tc := range selfPairedCases {
		t.Run(fmt.Sprintf("selfPaired_%d", i), func(t *testing.T) {
			view.BracketsQuotes.SetBrackets(map[rune]rune{'$': '$'})
			view.SetText(tc.input)
			view.Layout(gtx, shaper)
			view.SetCaret(tc.caret, tc.caret)
			left, right := view.NearestMatchingBrackets()
			if left != tc.want[0] || right != tc.want[1] {
				t.Logf("input=%q caret=%d: expected [%d, %d], got [%d, %d]",
					tc.input, tc.caret, tc.want[0], tc.want[1], left, right)
				t.Fail()
			}
		})
	}
}
