package layout

import (
	"testing"

	"gioui.org/text"
	"github.com/oligo/gvcode/internal/buffer"
)

func BenchmarkLayout(b *testing.B) {
	buf := buffer.NewTextSource()
	buf.SetText([]byte("a fox jumps over the lazy dog"))
	shaper := text.NewShaper()

	layouter := NewTextLayout(buf)

	for range b.N {
		layouter.Layout(shaper, &text.Parameters{PxPerEm: 14}, 4, true)
	}

}
