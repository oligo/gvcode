package textview

import (
	"fmt"
	"testing"

	"gioui.org/layout"
	"gioui.org/text"
)

func TestReadWord(t *testing.T) {
	view := NewTextView()

	doc := "hello,world!!!"

	testcases := []struct {
		position int
		want     struct {
			word   string
			offset int
		}
	}{
		{
			position: 0,
			want: struct {
				word   string
				offset int
			}{word: "hello", offset: 0},
		},
		{
			position: 2,
			want: struct {
				word   string
				offset int
			}{word: "hello", offset: 2},
		},

		{
			position: 5,
			want: struct {
				word   string
				offset int
			}{word: "hello", offset: 5},
		},

		{
			position: 6,
			want: struct {
				word   string
				offset int
			}{word: "world", offset: 0},
		},

		{
			position: 11,
			want: struct {
				word   string
				offset int
			}{word: "world", offset: 5},
		},
		{
			position: 12,
			want: struct {
				word   string
				offset int
			}{word: "", offset: 0},
		},
	}

	for i, tc := range testcases {
		t.Run(fmt.Sprintf("case %d", i), func(t *testing.T) {
			view.SetText(doc)
			gtx := layout.Context{}
			shaper := text.NewShaper()
			view.Layout(gtx, shaper)
			view.SetCaret(tc.position, tc.position)
			w, o := view.ReadWord(false)
			if w != tc.want.word || o != tc.want.offset {
				t.Logf("want: [word: %s, offset: %d], actual: [word: %s, offset: %d]", tc.want.word, tc.want.offset, w, o)
				t.Fail()
			}
		})
	}

}

func TestFindAllTextOccurrences(t *testing.T) {
	view := NewTextView()
	gtx := layout.Context{}
	shaper := text.NewShaper()

	testcases := []struct {
		doc      string
		start    int
		end      int
		expected [][2]int
	}{
		{
			doc:      "hello world hello hello world",
			start:    0,
			end:      5, // "hello"
			expected: [][2]int{{0, 5}, {12, 17}, {18, 23}},
		},
		{
			doc:      "hello world hello hello world",
			start:    6,
			end:      11, // "world"
			expected: [][2]int{{6, 11}, {24, 29}},
		},
		{
			doc:      "hello world hello hello world",
			start:    0,
			end:      0, // empty selection
			expected: nil,
		},
		{
			doc:      "hello world hello hello world",
			start:    0,
			end:      3, // "hel"
			expected: [][2]int{{0, 3}, {12, 15}, {18, 21}},
		},
		{
			doc:      "aaaaaa",
			start:    0,
			end:      3, // "aaa"
			expected: [][2]int{{0, 3}, {3, 6}},
		},
	}

	for i, tc := range testcases {
		t.Run(fmt.Sprintf("case %d", i), func(t *testing.T) {
			view.SetText(tc.doc)
			view.Layout(gtx, shaper)

			occurrences := view.FindAllTextOccurrences(tc.start, tc.end)

			if tc.expected == nil {
				if occurrences != nil {
					t.Errorf("Expected nil, got %v", occurrences)
				}
				return
			}

			if len(occurrences) != len(tc.expected) {
				t.Errorf("Expected %d occurrences, got %d", len(tc.expected), len(occurrences))
				return
			}

			for j, occ := range occurrences {
				if occ[0] != tc.expected[j][0] || occ[1] != tc.expected[j][1] {
					t.Errorf("Occurrence %d mismatch: got [%d,%d], expected [%d,%d]", j, occ[0], occ[1], tc.expected[j][0], tc.expected[j][1])
				}
			}
		})
	}
}

func TestFindAllWordOccurrences(t *testing.T) {
	view := NewTextView()
	gtx := layout.Context{}
	shaper := text.NewShaper()

	testcases := []struct {
		doc      string
		start    int
		end      int
		bySpace  bool
		expected [][2]int
	}{
		{
			doc:      "hello world hello hello world",
			start:    0,
			end:      5, // "hello"
			bySpace:  false,
			expected: [][2]int{{0, 5}, {12, 17}, {18, 23}},
		},
		{
			doc:      "hello world hello hello world",
			start:    6,
			end:      11, // "world"
			bySpace:  false,
			expected: [][2]int{{6, 11}, {24, 29}},
		},
		{
			doc:      "hello.world.hello.hello.world",
			start:    0,
			end:      5, // "hello"
			bySpace:  false,
			expected: [][2]int{{0, 5}, {12, 17}, {18, 23}},
		},
		{
			doc:      "hello world",
			start:    0,
			end:      0, // empty selection
			bySpace:  false,
			expected: nil,
		},
		{
			doc:      "  hello  world  hello  ",
			start:    2,
			end:      7, // "hello"
			bySpace:  true,
			expected: [][2]int{{2, 7}, {16, 21}},
		},
	}

	for i, tc := range testcases {
		t.Run(fmt.Sprintf("case %d", i), func(t *testing.T) {
			view.SetText(tc.doc)
			view.Layout(gtx, shaper)

			occurrences := view.FindAllWordOccurrences(tc.start, tc.end, tc.bySpace)

			if tc.expected == nil {
				if occurrences != nil {
					t.Errorf("Expected nil, got %v", occurrences)
				}
				return
			}

			if len(occurrences) != len(tc.expected) {
				t.Errorf("Expected %d occurrences, got %d", len(tc.expected), len(occurrences))
				return
			}

			for j, occ := range occurrences {
				if occ[0] != tc.expected[j][0] || occ[1] != tc.expected[j][1] {
					t.Errorf("Occurrence %d mismatch: got [%d,%d], expected [%d,%d]", j, occ[0], occ[1], tc.expected[j][0], tc.expected[j][1])
				}
			}
		})
	}
}
