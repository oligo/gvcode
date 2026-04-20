package syntax

import (
	"reflect"
	"testing"
)

func TestTextTokens_QueryRange(t *testing.T) {
	initial := []TokenStyle{
		{Start: 0, End: 5, Style: 1},
		{Start: 5, End: 10, Style: 2},
		{Start: 10, End: 15, Style: 3},
		{Start: 15, End: 20, Style: 4},
	}

	tests := []struct {
		name     string
		start    int
		end      int
		expected []TokenStyle
	}{
		{
			name:     "query entirely within one token",
			start:    2,
			end:      4,
			expected: []TokenStyle{{Start: 0, End: 5, Style: 1}},
		},
		{
			name:  "query overlapping two tokens",
			start: 4,
			end:   6,
			expected: []TokenStyle{
				{Start: 0, End: 5, Style: 1},
				{Start: 5, End: 10, Style: 2},
			},
		},
		{
			name:  "query exact match for one token",
			start: 5,
			end:   10,
			expected: []TokenStyle{
				{Start: 5, End: 10, Style: 2},
			},
		},
		{
			name:     "query out of bounds (before)",
			start:    -5,
			end:      0,
			expected: nil,
		},
		{
			name:     "query out of bounds (after)",
			start:    20,
			end:      25,
			expected: nil,
		},
		{
			name:     "invalid inverted range",
			start:    10,
			end:      5,
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens := &TextTokens{
				tokens: append([]TokenStyle{}, initial...),
			}

			got := tokens.QueryRange(tt.start, tt.end)

			// Both nil and empty slice are functionally equivalent here
			if len(got) == 0 && len(tt.expected) == 0 {
				return
			}

			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("QueryRange(%d, %d) got = %v, want %v", tt.start, tt.end, got, tt.expected)
			}
		})
	}
}

func TestTextTokens_AdjustOffsets(t *testing.T) {
	tests := []struct {
		name     string
		initial  []TokenStyle
		start    int
		end      int
		newEnd   int
		expected []TokenStyle
	}{
		{
			name: "pure insertion inside a token",
			// Insert 1 rune at index 5. The token spans 0-10.
			initial:  []TokenStyle{{Start: 0, End: 10, Style: 1}},
			start:    5,
			end:      5,
			newEnd:   6,
			expected: []TokenStyle{{Start: 0, End: 11, Style: 1}},
		},
		{
			name: "pure insertion at the boundary of two tokens",
			// Insert 1 rune at index 10. The left token should absorb the new character.
			initial: []TokenStyle{
				{Start: 0, End: 10, Style: 1},
				{Start: 10, End: 20, Style: 2},
			},
			start:  10,
			end:    10,
			newEnd: 11,
			expected: []TokenStyle{
				{Start: 0, End: 11, Style: 1},
				{Start: 11, End: 21, Style: 2},
			},
		},
		{
			name: "deletion entirely within a single token",
			// Delete 2 runes from index 5 to 7.
			initial:  []TokenStyle{{Start: 0, End: 10, Style: 1}},
			start:    5,
			end:      7,
			newEnd:   5,
			expected: []TokenStyle{{Start: 0, End: 8, Style: 1}},
		},
		{
			name: "deletion overlapping multiple tokens",
			// Delete from index 8 to 12.
			initial: []TokenStyle{
				{Start: 0, End: 10, Style: 1},
				{Start: 10, End: 20, Style: 2},
			},
			start:  8,
			end:    12,
			newEnd: 8,
			expected: []TokenStyle{
				{Start: 0, End: 8, Style: 1},
				{Start: 8, End: 16, Style: 2},
			},
		},
		{
			name: "deletion entirely wiping out a middle token",
			// Delete from index 2 to 12.
			initial: []TokenStyle{
				{Start: 0, End: 5, Style: 1},
				{Start: 5, End: 10, Style: 2}, // Should be completely removed
				{Start: 10, End: 15, Style: 3},
			},
			start:  2,
			end:    12,
			newEnd: 2,
			expected: []TokenStyle{
				{Start: 0, End: 2, Style: 1},
				{Start: 2, End: 5, Style: 3},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Initialize directly to bypass ColorScheme
			tokens := &TextTokens{
				tokens: append([]TokenStyle{}, tt.initial...),
			}

			tokens.AdjustOffsets(tt.start, tt.end, tt.newEnd)

			if !reflect.DeepEqual(tokens.tokens, tt.expected) {
				t.Errorf("AdjustOffsets() got = %v, want %v", tokens.tokens, tt.expected)
			}
		})
	}
}
