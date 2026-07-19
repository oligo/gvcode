//go:build ignore
// +build ignore

package main

import "testing"

func TestHighlightTextByPatternUsesRuneOffsets(t *testing.T) {
	tokens := HightlightTextByPattern("你好 func", syntaxPattern)
	if len(tokens) != 1 || tokens[0].Start != 3 || tokens[0].End != 7 {
		t.Fatalf("unexpected tokens: %+v", tokens)
	}
}
