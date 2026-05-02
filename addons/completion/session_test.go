package completion

import (
	"strings"
	"testing"

	"github.com/oligo/gvcode"
)

func TestHasTerminateChar(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"", false},
		{".", true},
		{"(", true},
		{")", true},
		{"{", true},
		{"}", true},
		{";", true},
		{",", true},
		{" ", true},
		{"\n", true},
		{"\t", true},
		{"a", false},
		{"1", false},
		{"_", false},
	}
	for _, tt := range tests {
		if got := hasTerminateChar(tt.input); got != tt.expected {
			t.Errorf("hasTerminateChar(%q) = %v, want %v", tt.input, got, tt.expected)
		}
	}
}

func TestIsSymbolChar(t *testing.T) {
	tests := []struct {
		ch       rune
		expected bool
	}{
		{'a', true},
		{'z', true},
		{'A', true},
		{'Z', true},
		{'0', true},
		{'9', true},
		{'_', true},
		{'.', false},
		{'(', false},
		{')', false},
		{' ', false},
		{'\n', false},
		{'!', false},
		{'@', false},
	}
	for _, tt := range tests {
		if got := isSymbolChar(tt.ch); got != tt.expected {
			t.Errorf("isSymbolChar(%q) = %v, want %v", tt.ch, got, tt.expected)
		}
	}
}

func TestCanTrigger(t *testing.T) {
	tr := gvcode.Trigger{
		Characters: []string{".", "::", "->"},
	}

	tests := []struct {
		input    string
		expected bool
	}{
		{".", true},
		{"::", true},
		{"->", true},
		{"a", true},
		{"Z", true},
		{"0", true},
		{"_", true},
		{"(", false},
		{")", false},
		{" ", false},
		{"!", false},
	}
	for _, tt := range tests {
		if got := canTrigger(tr, tt.input); got != tt.expected {
			t.Errorf("canTrigger(%q) = %v, want %v", tt.input, got, tt.expected)
		}
	}
}

// mockCompletor implements gvcode.Completor for testing.
type mockCompletor struct {
	trigger    gvcode.Trigger
	candidates []gvcode.CompletionCandidate
}

func (m *mockCompletor) Trigger() gvcode.Trigger { return m.trigger }

func (m *mockCompletor) Suggest(ctx gvcode.CompletionContext) []gvcode.CompletionCandidate {
	return m.candidates
}

func (m *mockCompletor) FilterAndRank(pattern string, candidates []gvcode.CompletionCandidate) []gvcode.CompletionCandidate {
	var result []gvcode.CompletionCandidate
	for _, c := range candidates {
		if pattern == "" || strings.HasPrefix(c.Label, pattern) {
			result = append(result, c)
		}
	}
	return result
}

// newTestEditor creates an Editor with the given text and caret at pos.
func newTestEditor(text string, caret int) *gvcode.Editor {
	editor := &gvcode.Editor{}
	editor.SetText(text)
	if caret > 0 {
		editor.SetCaret(caret, caret)
	}
	return editor
}

func TestSessionUpdate_IdentTriggerReadsPrefix(t *testing.T) {
	mock := &mockCompletor{
		trigger: gvcode.Trigger{},
		candidates: []gvcode.CompletionCandidate{
			{Label: "foo"},
			{Label: "bar"},
		},
	}

	editor := newTestEditor("f", 1)

	s := newSession(&delegatedCompletor{Completor: mock}, charTrigger)
	ctx := gvcode.CompletionContext{Input: "f", Position: gvcode.Position{Runes: 1}}
	result := s.Update(ctx, editor)

	if s.state.triggered {
		t.Error("expected triggered to be false after first Update")
	}
	// Prefix should be "f" read from buffer via ReadTextBetween(0, 1)
	if string(s.prefix) != "f" {
		t.Errorf("expected prefix 'f', got %q", string(s.prefix))
	}
	if len(result) != 1 || result[0].Label != "foo" {
		t.Errorf("expected [foo], got %v", result)
	}
}

func TestSessionUpdate_NonIdentTriggerPrefixEmpty(t *testing.T) {
	mock := &mockCompletor{
		trigger: gvcode.Trigger{},
		candidates: []gvcode.CompletionCandidate{
			{Label: "foo"},
			{Label: "bar"},
		},
	}

	editor := newTestEditor("#", 1)

	s := newSession(&delegatedCompletor{Completor: mock}, charTrigger)
	ctx := gvcode.CompletionContext{Input: "#", Position: gvcode.Position{Runes: 1}}
	result := s.Update(ctx, editor)

	// Non-identifier trigger: prefix starts at caret, not at trigger char.
	if string(s.prefix) != "" {
		t.Errorf("expected empty prefix for trigger '#', got %q", string(s.prefix))
	}
	if s.prefixRange.Start.Runes != 1 {
		t.Errorf("expected prefixRange.Start.Runes = 1 (at caret), got %d", s.prefixRange.Start.Runes)
	}
	// All candidates should be returned (empty prefix = no filter).
	if len(result) != 2 {
		t.Errorf("expected 2 results (unfiltered), got %d", len(result))
	}
}

func TestSessionUpdate_SubsequentCharExtendsPrefix(t *testing.T) {
	mock := &mockCompletor{
		trigger: gvcode.Trigger{},
		candidates: []gvcode.CompletionCandidate{
			{Label: "foo"},
			{Label: "foobar"},
			{Label: "baz"},
		},
	}

	// Simulate typing "f" then "o" → text is "fo"
	editor := newTestEditor("f", 1)

	s := newSession(&delegatedCompletor{Completor: mock}, charTrigger)
	ctx := gvcode.CompletionContext{Input: "f", Position: gvcode.Position{Runes: 1}}
	s.Update(ctx, editor)

	// Now type "o": text becomes "fo", caret at position 2.
	editor.SetText("fo")
	editor.SetCaret(2, 2)

	ctx2 := gvcode.CompletionContext{Input: "o", Position: gvcode.Position{Runes: 2}}
	result := s.Update(ctx2, editor)

	if string(s.prefix) != "fo" {
		t.Errorf("expected prefix 'fo', got %q", string(s.prefix))
	}
	if s.prefixRange.Start.Runes != 0 {
		t.Errorf("expected prefix start at 0, got %d", s.prefixRange.Start.Runes)
	}
	if len(result) != 2 || result[0].Label != "foo" || result[1].Label != "foobar" {
		t.Errorf("expected [foo foobar], got %v", result)
	}
}

func TestSessionUpdate_BackspaceShrinksPrefix(t *testing.T) {
	mock := &mockCompletor{
		trigger: gvcode.Trigger{},
		candidates: []gvcode.CompletionCandidate{
			{Label: "foo"},
			{Label: "foobar"},
			{Label: "bar"},
		},
	}

	// Simulate typing "f" then "o" then backspace.
	editor := newTestEditor("f", 1)

	s := newSession(&delegatedCompletor{Completor: mock}, charTrigger)
	ctx := gvcode.CompletionContext{Input: "f", Position: gvcode.Position{Runes: 1}}
	s.Update(ctx, editor)

	// Type "o": text becomes "fo".
	editor.SetText("fo")
	editor.SetCaret(2, 2)
	ctx2 := gvcode.CompletionContext{Input: "o", Position: gvcode.Position{Runes: 2}}
	s.Update(ctx2, editor)

	if string(s.prefix) != "fo" {
		t.Fatalf("expected prefix 'fo', got %q", string(s.prefix))
	}

	// Backspace: delete "o", text becomes "f".
	editor.SetText("f")
	editor.SetCaret(1, 1)
	ctx3 := gvcode.CompletionContext{Input: "", Position: gvcode.Position{Runes: 1}}
	result := s.Update(ctx3, editor)

	if !s.IsValid() {
		t.Error("session should remain valid after backspace with non-empty prefix")
	}
	if string(s.prefix) != "f" {
		t.Errorf("expected prefix 'f', got %q", string(s.prefix))
	}
	if len(result) != 2 {
		t.Errorf("expected 2 results, got %v", result)
	}
}

func TestSessionUpdate_CursorMoveWithinPrefixKeepsEnd(t *testing.T) {
	mock := &mockCompletor{
		trigger: gvcode.Trigger{},
		candidates: []gvcode.CompletionCandidate{
			{Label: "foo"},
			{Label: "foobar"},
		},
	}

	// Simulate typing "f" then "o": prefix "fo", End at rune 2.
	editor := newTestEditor("f", 1)

	s := newSession(&delegatedCompletor{Completor: mock}, charTrigger)
	ctx := gvcode.CompletionContext{Input: "f", Position: gvcode.Position{Runes: 1}}
	s.Update(ctx, editor)

	// Type "o".
	editor.SetText("fo")
	editor.SetCaret(2, 2)
	ctx2 := gvcode.CompletionContext{Input: "o", Position: gvcode.Position{Runes: 2}}
	s.Update(ctx2, editor)

	if s.prefixRange.End.Runes != 2 {
		t.Fatalf("expected End.Runes = 2, got %d", s.prefixRange.End.Runes)
	}

	// Move cursor to f|o (caret at 1 within the prefix). End should not shrink.
	ctx3 := gvcode.CompletionContext{Input: "", Position: gvcode.Position{Runes: 1}}
	s.Update(ctx3, editor)

	// Filter prefix should be "f" (only text before caret).
	if string(s.prefix) != "f" {
		t.Errorf("expected filter prefix 'f', got %q", string(s.prefix))
	}
	// End.Runes should stay at 2, not shrink to 1. The replacement range
	// must still cover the full prefix "fo".
	if s.prefixRange.End.Runes != 2 {
		t.Errorf("End.Runes should stay at 2 after cursor move within prefix, got %d",
			s.prefixRange.End.Runes)
	}
}

func TestSessionUpdate_BackspaceToEmptyCancels(t *testing.T) {
	mock := &mockCompletor{
		trigger: gvcode.Trigger{},
		candidates: []gvcode.CompletionCandidate{
			{Label: "foo"},
		},
	}

	editor := newTestEditor("f", 1)

	s := newSession(&delegatedCompletor{Completor: mock}, charTrigger)
	ctx := gvcode.CompletionContext{Input: "f", Position: gvcode.Position{Runes: 1}}
	s.Update(ctx, editor)

	// Backspace everything.
	editor.SetText("")
	editor.SetCaret(0, 0)

	ctx2 := gvcode.CompletionContext{Input: "", Position: gvcode.Position{Runes: 0}}
	result := s.Update(ctx2, editor)

	if s.IsValid() {
		t.Error("session should be canceled after backspace to empty")
	}
	if result != nil {
		t.Error("expected nil result after cancel")
	}
}

func TestSessionUpdate_TerminateChar(t *testing.T) {
	mock := &mockCompletor{
		trigger: gvcode.Trigger{},
		candidates: []gvcode.CompletionCandidate{
			{Label: "foo"},
		},
	}

	editor := newTestEditor("f", 1)

	s := newSession(&delegatedCompletor{Completor: mock}, charTrigger)
	ctx := gvcode.CompletionContext{Input: "f", Position: gvcode.Position{Runes: 1}}
	s.Update(ctx, editor)

	if s.state.triggerChars != "f" {
		t.Errorf("expected triggerChars to be 'f', got %q", s.state.triggerChars)
	}

	// Terminating character invalidates the session.
	ctx2 := gvcode.CompletionContext{Input: "."}
	result := s.Update(ctx2, editor)
	if len(result) != 0 {
		t.Error("expected empty result after terminate char")
	}
	if s.IsValid() {
		t.Error("expected session to be invalidated")
	}
}

func TestSessionUpdate_Canceled(t *testing.T) {
	s := &session{canceled: true}
	result := s.Update(gvcode.CompletionContext{Input: "a"}, nil)
	if result != nil {
		t.Error("expected nil result for canceled session")
	}
}

func TestSessionUpdate_KeyTriggerEmptyPrefixStaysAlive(t *testing.T) {
	mock := &mockCompletor{
		trigger: gvcode.Trigger{},
		candidates: []gvcode.CompletionCandidate{
			{Label: "foo"},
			{Label: "bar"},
			{Label: "baz"},
		},
	}

	editor := newTestEditor("", 0)

	s := newSession(&delegatedCompletor{Completor: mock}, keyTrigger)
	result := s.Update(gvcode.CompletionContext{Input: "", Position: gvcode.Position{Runes: 0}}, editor)

	if !s.IsValid() {
		t.Error("key-triggered session should remain valid with empty input and empty prefix")
	}
	if len(result) != 3 {
		t.Errorf("expected 3 results (unfiltered), got %d", len(result))
	}
}

func TestSessionUpdate_NonIdentTriggerThenChar(t *testing.T) {
	mock := &mockCompletor{
		trigger: gvcode.Trigger{},
		candidates: []gvcode.CompletionCandidate{
			{Label: "text"},
			{Label: "table"},
			{Label: "title"},
		},
	}

	// Simulate Typst-style "#" trigger then typing "t".
	editor := newTestEditor("#t", 2)

	// First Update: "#" triggers the session.
	s := newSession(&delegatedCompletor{Completor: mock}, charTrigger)
	ctx := gvcode.CompletionContext{Input: "#", Position: gvcode.Position{Runes: 1}}
	s.Update(ctx, editor)

	// Prefix range starts at caret after "#" (rune 1), prefix is empty.
	if s.prefixRange.Start.Runes != 1 {
		t.Errorf("expected prefix start at rune 1 (after '#'), got %d", s.prefixRange.Start.Runes)
	}

	// Second Update: "t" is typed. Prefix should be "t", NOT "#t".
	ctx2 := gvcode.CompletionContext{Input: "t", Position: gvcode.Position{Runes: 2}}
	result := s.Update(ctx2, editor)

	if string(s.prefix) != "t" {
		t.Errorf("expected prefix 't' (not '#t'), got %q", string(s.prefix))
	}
	if len(result) != 3 {
		t.Errorf("expected 3 results (t*), got %v", result)
	}
}

func TestSessionUpdate_BackspaceAfterNonIdentTriggerCancels(t *testing.T) {
	mock := &mockCompletor{
		trigger: gvcode.Trigger{},
		candidates: []gvcode.CompletionCandidate{
			{Label: "text"},
		},
	}

	// "#text" then backspace all.
	editor := newTestEditor("#text", 4)

	s := newSession(&delegatedCompletor{Completor: mock}, charTrigger)
	ctx := gvcode.CompletionContext{Input: "#", Position: gvcode.Position{Runes: 1}}
	s.Update(ctx, editor)

	// Type "t", "e", "x", "t".
	for i, ch := range []string{"t", "e", "x", "t"} {
		editor.SetCaret(2+i, 2+i)
		ctx := gvcode.CompletionContext{Input: ch, Position: gvcode.Position{Runes: 2 + i}}
		s.Update(ctx, editor)
	}

	if string(s.prefix) != "text" {
		t.Errorf("expected prefix 'text', got %q", string(s.prefix))
	}

	// Backspace all the way through "#".
	editor.SetText("")
	editor.SetCaret(0, 0)

	ctx2 := gvcode.CompletionContext{Input: "", Position: gvcode.Position{Runes: 0}}
	result := s.Update(ctx2, editor)

	if s.IsValid() {
		t.Error("session should be canceled when backspacing past trigger char")
	}
	if result != nil {
		t.Error("expected nil result")
	}
}
