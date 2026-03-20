package decoration

import (
	"testing"

	"github.com/oligo/gvcode/internal/buffer"
)

func TestInsertAndQueryDecoration(t *testing.T) {
	d := NewDecorationTree(buffer.NewTextSource())

	bg := Decoration{Start: 0, End: 5, Background: &Background{}}
	italic := Decoration{Start: 6, End: 9, Italic: true}
	bold := Decoration{Start: 6, End: 9, Bold: true}
	underline := Decoration{Start: 0, End: 6, Underline: &Underline{}}
	strikethrough := Decoration{Start: 11, End: 15, Strikethrough: &Strikethrough{}}
	box := Decoration{Start: 16, End: 20, Border: &Border{}}

	d.Insert(bg)
	d.Insert(italic)
	d.Insert(bold)
	d.Insert(underline)
	d.Insert(strikethrough)
	d.Insert(box)

	all := d.QueryRange(0, 20)
	if len(all) != 6 {
		t.Fail()
	}

	all = d.QueryRange(0, 6)
	if len(all) != 2 {
		t.Logf("Expected: %d, got: %d", 2, len(all))
		t.Fail()
	}
}

func TestRemoveDecorationBySource(t *testing.T) {
	d := NewDecorationTree(buffer.NewTextSource())

	bg := Decoration{Start: 0, End: 5, Background: &Background{}}
	italic := Decoration{Start: 6, End: 9, Italic: true}
	bold := Decoration{Start: 6, End: 9, Bold: true}
	underline := Decoration{Start: 0, End: 6, Underline: &Underline{}}
	strikethrough := Decoration{Start: 11, End: 15, Strikethrough: &Strikethrough{}}
	box := Decoration{Start: 16, End: 20, Border: &Border{}}
	bg.Source = "selection"
	box.Source = "selection"

	d.Insert(bg)
	d.Insert(italic)
	d.Insert(bold)
	d.Insert(underline)
	d.Insert(strikethrough)
	d.Insert(box)

	d.RemoveBySource("selection")
	if v := d.QueryRange(0, 5); len(v) != 1 {
		t.Fail()
	}

	if v := d.QueryRange(16, 20); len(v) > 0 {
		t.Fail()
	}
}

func TestQueryRangeDecorationExtendsBeyondQuery(t *testing.T) {
	d := NewDecorationTree(buffer.NewTextSource())

	// Decoration that extends beyond the query range on both sides
	bigDeco := Decoration{Start: 0, End: 15, Background: &Background{}}
	// Decoration within the query range
	smallDeco := Decoration{Start: 5, End: 10, Italic: true}

	d.Insert(bigDeco)
	d.Insert(smallDeco)

	// Query range [5, 10)
	all := d.QueryRange(5, 10)

	// Should include both:
	// - bigDeco [0, 15) overlaps with [5, 10)
	// - smallDeco [5, 10) overlaps with [5, 10)
	if len(all) != 2 {
		t.Logf("Expected 2, got %d", len(all))
		for _, deco := range all {
			t.Logf("Decoration: %d - %d", deco.Start, deco.End)
		}
		t.Fail()
	}
}

func TestQueryRangeDecorationStartsBeforeQuery(t *testing.T) {
	d := NewDecorationTree(buffer.NewTextSource())

	// Decoration that starts before but ends within query
	decoration := Decoration{Start: 3, End: 7, Background: &Background{}}

	d.Insert(decoration)

	// Query range [5, 10)
	all := d.QueryRange(5, 10)

	// Should include decoration because [3, 7) overlaps with [5, 10)
	if len(all) != 1 {
		t.Logf("Expected 1, got %d", len(all))
		t.Fail()
	}
}

func TestQueryRangeDecorationEndsAtQueryStart(t *testing.T) {
	d := NewDecorationTree(buffer.NewTextSource())

	// Decoration that ends exactly at query start
	decoration := Decoration{Start: 0, End: 5, Background: &Background{}}

	d.Insert(decoration)

	// Query range [5, 10)
	all := d.QueryRange(5, 10)

	// Should NOT include decoration because [0, 5) doesn't overlap with [5, 10)
	// (end is exclusive)
	if len(all) != 0 {
		t.Logf("Expected 0, got %d", len(all))
		t.Fail()
	}
}

func TestQueryRangeEmptyRange(t *testing.T) {
	d := NewDecorationTree(buffer.NewTextSource())

	decoration := Decoration{Start: 5, End: 10, Background: &Background{}}

	d.Insert(decoration)

	// Query empty range [5, 5)
	all := d.QueryRange(5, 5)

	// Should return empty - an empty range has no overlap with [5, 10)
	if len(all) != 0 {
		t.Logf("Expected 0, got %d", len(all))
		t.Fail()
	}
}

func TestQueryRangeStartGreaterThanEnd(t *testing.T) {
	d := NewDecorationTree(buffer.NewTextSource())

	decoration := Decoration{Start: 0, End: 10, Background: &Background{}}

	d.Insert(decoration)

	// Invalid query range
	all := d.QueryRange(10, 5)

	// Should return nil for invalid range
	if all != nil {
		t.Logf("Expected nil, got %v", all)
		t.Fail()
	}
}
