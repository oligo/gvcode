package layout

import (
	"reflect"
	"strings"
	"testing"

	"gioui.org/text"
	"github.com/oligo/gvcode/internal/buffer"
)

func TestCachedLayoutMatchesFreshLayoutAfterEdit(t *testing.T) {
	buf := buffer.NewTextSource()
	buf.SetText([]byte("alpha\nmiddle\nalpha\n"))
	shaper := text.NewShaper()
	params := text.Parameters{PxPerEm: 14, MaxWidth: 400}
	cached := NewTextLayout(buf)
	cached.Layout(shaper, &params, 4, true)

	check := func() {
		t.Helper()
		gotDims := cached.Layout(shaper, &params, 4, true)
		fresh := NewTextLayout(buffer.NewPieceTable(buffer.NewReader(buf).ReadAll(nil)))
		wantDims := fresh.Layout(shaper, &params, 4, true)

		if gotDims != wantDims {
			t.Fatalf("dimensions differ: got %+v, want %+v", gotDims, wantDims)
		}
		if !reflect.DeepEqual(cached.Lines, fresh.Lines) {
			t.Fatalf("lines differ: got %+v, want %+v", cached.Lines, fresh.Lines)
		}
		if !reflect.DeepEqual(cached.Paragraphs, fresh.Paragraphs) {
			t.Fatalf("paragraphs differ: got %+v, want %+v", cached.Paragraphs, fresh.Paragraphs)
		}
		if !reflect.DeepEqual(cached.Positions, fresh.Positions) {
			t.Fatalf("positions differ: got %+v, want %+v", cached.Positions, fresh.Positions)
		}
		if !reflect.DeepEqual(cached.Graphemes, fresh.Graphemes) {
			t.Fatalf("graphemes differ: got %+v, want %+v", cached.Graphemes, fresh.Graphemes)
		}
	}

	check()
	buf.Replace(8, 9, "X")
	check()
	buf.Replace(5, 6, "")
	check()
	buf.Replace(buf.Len()-1, buf.Len(), "")
	check()
	params.MaxWidth = 40
	check()
}

func TestCachedLayoutClearsDiscardedStorage(t *testing.T) {
	buf := buffer.NewTextSource()
	buf.SetText([]byte(strings.Repeat("line\n", 2000)))
	shaper := text.NewShaper()
	params := text.Parameters{PxPerEm: 14, MaxWidth: 400}
	tl := NewTextLayout(buf)
	tl.Layout(shaper, &params, 4, true)

	buf.SetText([]byte("short"))
	tl.Layout(shaper, &params, 4, true)

	if len(tl.nextParas) != 0 {
		t.Fatalf("inactive paragraph cache length = %d, want 0", len(tl.nextParas))
	}
	if len(tl.nextCache) != 0 {
		t.Fatalf("inactive glyph cache length = %d, want 0", len(tl.nextCache))
	}
	for _, paragraph := range tl.paragraphs[len(tl.paragraphs):cap(tl.paragraphs)] {
		if paragraph.text != "" || len(paragraph.lines) != 0 || len(paragraph.graphemes) != 0 {
			t.Fatal("discarded paragraph cache still holds data")
		}
	}
	for _, line := range tl.Lines[len(tl.Lines):cap(tl.Lines)] {
		if len(line.Glyphs) != 0 {
			t.Fatal("discarded line cache still holds glyphs")
		}
	}
}

func TestLayoutReadsParagraphLargerThanBuffer(t *testing.T) {
	longLine := strings.Repeat("x", 5000)
	buf := buffer.NewPieceTable([]byte(longLine + "\ntail"))
	tl := NewTextLayout(buf)
	params := text.Parameters{PxPerEm: 14, MaxWidth: 400}
	tl.Layout(text.NewShaper(), &params, 4, false)

	if len(tl.paragraphs) != 2 {
		t.Fatalf("paragraph count = %d, want 2", len(tl.paragraphs))
	}
	if got := tl.paragraphs[0].text; got != longLine+"\n" {
		t.Fatalf("first paragraph length = %d, want %d", len(got), len(longLine)+1)
	}
	if got := tl.paragraphs[1].text; got != "tail" {
		t.Fatalf("last paragraph = %q, want %q", got, "tail")
	}
}

func BenchmarkLayout(b *testing.B) {
	buf := buffer.NewTextSource()
	buf.SetText([]byte("a fox jumps over the lazy dog"))
	shaper := text.NewShaper()

	layouter := NewTextLayout(buf)

	for range b.N {
		layouter.Layout(shaper, &text.Parameters{PxPerEm: 14}, 4, true)
	}

}
