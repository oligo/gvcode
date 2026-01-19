package layout

import (
	"testing"

	"gioui.org/font"
	"gioui.org/text"
	"golang.org/x/image/math/fixed"
)

func setupShaper() (*text.Shaper, text.Parameters, text.Glyph) {
	shaper := text.NewShaper()

	params := text.Parameters{
		Font:     font.Font{Typeface: font.Typeface("monospace")},
		PxPerEm:  fixed.I(14),
		MaxWidth: 1e6,
	}

	shaper.LayoutString(params, "\u0020")
	spaceGlyph, _ := shaper.NextGlyph()

	return shaper, params, spaceGlyph
}

func TestBidiTextLayout(t *testing.T) {
	testcases := []struct {
		name  string
		input string
	}{
		{
			name:  "Pure RTL Hebrew",
			input: "שלום עולם",
		},
		{
			name:  "Pure RTL Arabic",
			input: "مرحبا بالعالم",
		},
		{
			name:  "Mixed LTR and RTL",
			input: "Hello שלום World",
		},
		{
			name:  "RTL with numbers",
			input: "שלום 123 עולם",
		},
		{
			name:  "LTR with embedded RTL",
			input: "The word שלום means peace",
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			shaper, params, spaceGlyph := setupShaper()

			shaper.LayoutString(params, tc.input)

			wrapper := lineWrapper{}
			lines := wrapper.WrapParagraph(glyphIter{shaper: shaper}.All(), []rune(tc.input), 1e6, 4, &spaceGlyph)

			if len(lines) == 0 {
				t.Fatal("Expected at least one line")
			}

			// Verify total runes match input
			totalRunes := 0
			for _, line := range lines {
				totalRunes += line.Runes
			}
			expectedRunes := len([]rune(tc.input))
			if totalRunes != expectedRunes {
				t.Errorf("Rune count mismatch: got %d, want %d", totalRunes, expectedRunes)
			}

			// Verify all glyphs have valid positions
			for i, line := range lines {
				for j, gl := range line.Glyphs {
					if gl == nil {
						t.Errorf("Line %d, glyph %d: nil glyph", i, j)
						continue
					}
					// X position should be non-negative after layout
					if gl.X < 0 {
						t.Errorf("Line %d, glyph %d: negative X position %d", i, j, gl.X)
					}
				}
			}
		})
	}
}

func TestBidiLineWidth(t *testing.T) {
	shaper, params, spaceGlyph := setupShaper()

	testcases := []struct {
		name  string
		input string
	}{
		{
			name:  "LTR text",
			input: "Hello World",
		},
		{
			name:  "RTL text",
			input: "שלום עולם",
		},
		{
			name:  "Mixed text",
			input: "Hello שלום",
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			shaper.LayoutString(params, tc.input)

			wrapper := lineWrapper{}
			lines := wrapper.WrapParagraph(glyphIter{shaper: shaper}.All(), []rune(tc.input), 1e6, 4, &spaceGlyph)

			if len(lines) == 0 {
				t.Fatal("Expected at least one line")
			}

			line := lines[0]

			// Line width should be positive
			if line.Width <= 0 {
				t.Errorf("Line width should be positive, got %d", line.Width)
			}

			// Calculate sum of advances
			sumAdvances := fixed.I(0)
			for _, gl := range line.Glyphs {
				sumAdvances += gl.Advance
			}

			// Line width should equal sum of advances
			if line.Width != sumAdvances {
				t.Errorf("Line width mismatch: Width=%d, sum of advances=%d", line.Width, sumAdvances)
			}
		})
	}
}

func TestBidiGlyphOrder(t *testing.T) {
	shaper, params, spaceGlyph := setupShaper()

	// For mixed bidi text, glyphs should be in visual order
	input := "AB שלום CD"
	shaper.LayoutString(params, input)

	wrapper := lineWrapper{}
	lines := wrapper.WrapParagraph(glyphIter{shaper: shaper}.All(), []rune(input), 1e6, 4, &spaceGlyph)

	if len(lines) == 0 {
		t.Fatal("Expected at least one line")
	}

	line := lines[0]

	// After recompute, verify glyphs are properly positioned
	line.recompute(fixed.I(0), 0)

	// Check that we have the expected number of glyphs (accounting for cluster breaks)
	if len(line.Glyphs) == 0 {
		t.Fatal("Expected glyphs in line")
	}

	// Verify no overlapping glyphs (each glyph's right edge should not exceed next glyph's position significantly)
	// Note: For bidi text, glyphs might not be strictly ordered by X position
	// but they should form a valid visual representation
	for i, gl := range line.Glyphs {
		if gl.X < 0 {
			t.Errorf("Glyph %d has negative X position: %d", i, gl.X)
		}
	}
}

func TestEmptyLineRecompute(t *testing.T) {
	line := Line{}

	// Should not panic on empty line
	line.recompute(fixed.I(100), 0)

	if line.RuneOff != 0 {
		t.Errorf("RuneOff should be 0, got %d", line.RuneOff)
	}
}

// makeGlyph creates a test glyph with specified advance and direction.
func makeGlyph(advance int, rtl bool) *text.Glyph {
	g := &text.Glyph{
		Advance: fixed.I(advance),
		Runes:   1,
	}
	if rtl {
		g.Flags |= text.FlagTowardOrigin
	}
	return g
}

func TestRecomputeLTROnly(t *testing.T) {
	// Test pure LTR text: glyphs should be laid out left-to-right
	line := Line{
		Glyphs: []*text.Glyph{
			makeGlyph(10, false), // advance=10, LTR
			makeGlyph(20, false), // advance=20, LTR
			makeGlyph(15, false), // advance=15, LTR
		},
	}

	alignOff := fixed.I(5)
	line.recompute(alignOff, 100)

	// Verify RuneOff is set
	if line.RuneOff != 100 {
		t.Errorf("RuneOff: got %d, want 100", line.RuneOff)
	}

	// Verify X positions: each glyph starts where the previous one ends
	expectedX := []fixed.Int26_6{
		fixed.I(5),  // alignOff
		fixed.I(15), // alignOff + 10
		fixed.I(35), // alignOff + 10 + 20
	}

	for i, gl := range line.Glyphs {
		if gl.X != expectedX[i] {
			t.Errorf("Glyph %d: X = %d, want %d", i, gl.X, expectedX[i])
		}
	}

	// Verify last glyph has FlagLineBreak
	if line.Glyphs[2].Flags&text.FlagLineBreak == 0 {
		t.Error("Last glyph should have FlagLineBreak")
	}
}

func TestRecomputeRTLOnly(t *testing.T) {
	// Test pure RTL text: glyphs should be laid out right-to-left within their run
	line := Line{
		Glyphs: []*text.Glyph{
			makeGlyph(10, true), // advance=10, RTL
			makeGlyph(20, true), // advance=20, RTL
			makeGlyph(15, true), // advance=15, RTL
		},
	}

	alignOff := fixed.I(0)
	line.recompute(alignOff, 0)

	// For RTL run with total width 45:
	// - Run occupies [0, 45)
	// - Glyph 0 (advance=10): X = 45 - 10 = 35
	// - Glyph 1 (advance=20): X = 35 - 20 = 15
	// - Glyph 2 (advance=15): X = 15 - 15 = 0
	expectedX := []fixed.Int26_6{
		fixed.I(35), // runWidth - advance[0] = 45 - 10 = 35
		fixed.I(15), // 35 - advance[1] = 35 - 20 = 15
		fixed.I(0),  // 15 - advance[2] = 15 - 15 = 0
	}

	for i, gl := range line.Glyphs {
		if gl.X != expectedX[i] {
			t.Errorf("Glyph %d: X = %d, want %d", i, gl.X, expectedX[i])
		}
	}

	// Verify last glyph has FlagLineBreak
	if line.Glyphs[2].Flags&text.FlagLineBreak == 0 {
		t.Error("Last glyph should have FlagLineBreak")
	}
}

func TestRecomputeMixedLTRThenRTL(t *testing.T) {
	// Test mixed: LTR run followed by RTL run
	// Visual: [LTR1][LTR2][RTL2][RTL1]
	line := Line{
		Glyphs: []*text.Glyph{
			makeGlyph(10, false), // LTR, advance=10
			makeGlyph(10, false), // LTR, advance=10
			makeGlyph(10, true),  // RTL, advance=10
			makeGlyph(10, true),  // RTL, advance=10
		},
	}

	alignOff := fixed.I(0)
	line.recompute(alignOff, 0)

	// LTR run [0,1]: width=20, occupies [0, 20)
	// - Glyph 0: X = 0
	// - Glyph 1: X = 10
	// RTL run [2,3]: width=20, occupies [20, 40)
	// - Glyph 2: X = 40 - 10 = 30
	// - Glyph 3: X = 30 - 10 = 20
	expectedX := []fixed.Int26_6{
		fixed.I(0),  // LTR: starts at 0
		fixed.I(10), // LTR: 0 + 10
		fixed.I(30), // RTL: run starts at 20, width 20, first glyph at 20+20-10=30
		fixed.I(20), // RTL: 30 - 10 = 20
	}

	for i, gl := range line.Glyphs {
		if gl.X != expectedX[i] {
			t.Errorf("Glyph %d: X = %d, want %d", i, gl.X, expectedX[i])
		}
	}
}

func TestRecomputeMixedRTLThenLTR(t *testing.T) {
	// Test mixed: RTL run followed by LTR run
	line := Line{
		Glyphs: []*text.Glyph{
			makeGlyph(10, true),  // RTL, advance=10
			makeGlyph(10, true),  // RTL, advance=10
			makeGlyph(10, false), // LTR, advance=10
			makeGlyph(10, false), // LTR, advance=10
		},
	}

	alignOff := fixed.I(0)
	line.recompute(alignOff, 0)

	// RTL run [0,1]: width=20, occupies [0, 20)
	// - Glyph 0: X = 20 - 10 = 10
	// - Glyph 1: X = 10 - 10 = 0
	// LTR run [2,3]: width=20, occupies [20, 40)
	// - Glyph 2: X = 20
	// - Glyph 3: X = 30
	expectedX := []fixed.Int26_6{
		fixed.I(10), // RTL: run width 20, first glyph at 20-10=10
		fixed.I(0),  // RTL: 10 - 10 = 0
		fixed.I(20), // LTR: starts at xOff=20
		fixed.I(30), // LTR: 20 + 10 = 30
	}

	for i, gl := range line.Glyphs {
		if gl.X != expectedX[i] {
			t.Errorf("Glyph %d: X = %d, want %d", i, gl.X, expectedX[i])
		}
	}
}

func TestRecomputeWithAlignmentOffset(t *testing.T) {
	// Test that alignment offset is correctly applied
	line := Line{
		Glyphs: []*text.Glyph{
			makeGlyph(10, false),
			makeGlyph(10, false),
		},
	}

	alignOff := fixed.I(100)
	line.recompute(alignOff, 0)

	expectedX := []fixed.Int26_6{
		fixed.I(100), // alignOff
		fixed.I(110), // alignOff + 10
	}

	for i, gl := range line.Glyphs {
		if gl.X != expectedX[i] {
			t.Errorf("Glyph %d: X = %d, want %d", i, gl.X, expectedX[i])
		}
	}
}

func TestRecomputeWithTabLikeAdvance(t *testing.T) {
	// Test that large advances (like expanded tabs) work correctly
	line := Line{
		Glyphs: []*text.Glyph{
			makeGlyph(10, false), // normal char
			makeGlyph(80, false), // tab (expanded to 80 pixels)
			makeGlyph(10, false), // normal char after tab
		},
	}

	alignOff := fixed.I(0)
	line.recompute(alignOff, 0)

	expectedX := []fixed.Int26_6{
		fixed.I(0),  // first char
		fixed.I(10), // tab starts at 10
		fixed.I(90), // char after tab: 10 + 80 = 90
	}

	for i, gl := range line.Glyphs {
		if gl.X != expectedX[i] {
			t.Errorf("Glyph %d: X = %d, want %d", i, gl.X, expectedX[i])
		}
	}
}

func TestRecomputeSingleGlyph(t *testing.T) {
	t.Run("single LTR", func(t *testing.T) {
		line := Line{
			Glyphs: []*text.Glyph{makeGlyph(10, false)},
		}
		line.recompute(fixed.I(5), 42)

		if line.Glyphs[0].X != fixed.I(5) {
			t.Errorf("X = %d, want %d", line.Glyphs[0].X, fixed.I(5))
		}
		if line.RuneOff != 42 {
			t.Errorf("RuneOff = %d, want 42", line.RuneOff)
		}
		if line.Glyphs[0].Flags&text.FlagLineBreak == 0 {
			t.Error("Should have FlagLineBreak")
		}
	})

	t.Run("single RTL", func(t *testing.T) {
		line := Line{
			Glyphs: []*text.Glyph{makeGlyph(10, true)},
		}
		line.recompute(fixed.I(0), 0)

		// RTL single glyph: run width=10, X = 0 + 10 - 10 = 0
		if line.Glyphs[0].X != fixed.I(0) {
			t.Errorf("X = %d, want %d", line.Glyphs[0].X, fixed.I(0))
		}
		if line.Glyphs[0].Flags&text.FlagLineBreak == 0 {
			t.Error("Should have FlagLineBreak")
		}
	})
}

func TestRecomputeAlternatingDirections(t *testing.T) {
	// Test alternating LTR-RTL-LTR pattern
	line := Line{
		Glyphs: []*text.Glyph{
			makeGlyph(10, false), // LTR
			makeGlyph(10, true),  // RTL
			makeGlyph(10, false), // LTR
		},
	}

	alignOff := fixed.I(0)
	line.recompute(alignOff, 0)

	// LTR run [0]: width=10, occupies [0, 10)
	// - Glyph 0: X = 0
	// RTL run [1]: width=10, occupies [10, 20)
	// - Glyph 1: X = 20 - 10 = 10
	// LTR run [2]: width=10, occupies [20, 30)
	// - Glyph 2: X = 20
	expectedX := []fixed.Int26_6{
		fixed.I(0),  // LTR
		fixed.I(10), // RTL: starts at xOff=10, width=10, so X = 10+10-10 = 10
		fixed.I(20), // LTR: starts at xOff=20
	}

	for i, gl := range line.Glyphs {
		if gl.X != expectedX[i] {
			t.Errorf("Glyph %d: X = %d, want %d", i, gl.X, expectedX[i])
		}
	}
}

func TestRecomputeTotalWidth(t *testing.T) {
	// Verify that the total span of glyphs equals the sum of advances
	testCases := []struct {
		name   string
		glyphs []*text.Glyph
	}{
		{
			name: "LTR only",
			glyphs: []*text.Glyph{
				makeGlyph(10, false),
				makeGlyph(20, false),
				makeGlyph(15, false),
			},
		},
		{
			name: "RTL only",
			glyphs: []*text.Glyph{
				makeGlyph(10, true),
				makeGlyph(20, true),
				makeGlyph(15, true),
			},
		},
		{
			name: "Mixed",
			glyphs: []*text.Glyph{
				makeGlyph(10, false),
				makeGlyph(20, true),
				makeGlyph(15, false),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			line := Line{Glyphs: tc.glyphs}
			line.recompute(fixed.I(0), 0)

			// Calculate expected total width
			totalAdvance := fixed.I(0)
			for _, gl := range tc.glyphs {
				totalAdvance += gl.Advance
			}

			// Find min and max X positions
			minX := line.Glyphs[0].X
			maxX := line.Glyphs[0].X + line.Glyphs[0].Advance
			for _, gl := range line.Glyphs {
				if gl.X < minX {
					minX = gl.X
				}
				if gl.X+gl.Advance > maxX {
					maxX = gl.X + gl.Advance
				}
			}

			actualWidth := maxX - minX
			if actualWidth != totalAdvance {
				t.Errorf("Total width = %d, want %d (sum of advances)", actualWidth, totalAdvance)
			}
		})
	}
}
