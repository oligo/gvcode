package completion

import (
	"slices"

	"github.com/oligo/gvcode"
)

type triggerKind uint8

const (
	autoTrigger triggerKind = iota
	charTrigger
	keyTrigger
)

type triggerState struct {
	triggerKind triggerKind
	// the activated completor.
	completor    *delegatedCompletor
	triggered    bool
	triggerChars string
}

// A session is started when some trigger is activated, and is destroyed when
// the completion is canceled or confirmed.
type session struct {
	ctx      gvcode.CompletionContext
	state    *triggerState
	canceled bool
	// prefix is the current filter text: the text between prefixRange.Start
	// and the current caret. When the caret moves within the prefix, prefix
	// changes but prefixRange does not shrink — prefixRange tracks the full
	// replacement range for OnConfirm.
	prefix []rune
	// prefixRange spans from the session start position to the furthest the
	// caret has reached. Its Start and End are used by OnConfirm (via
	// ConvertPos) to determine the replacement range. May be wider than prefix
	// when the caret moves back within the prefix.
	prefixRange gvcode.EditRange
	// Full candidates from the completor.
	candidates []gvcode.CompletionCandidate
}

func newSession(completor *delegatedCompletor, kind triggerKind) *session {
	return &session{
		state: &triggerState{
			triggerKind: kind,
			completor:   completor,
			triggered:   true,
		},
	}
}

var terminatingChars = []rune{
	'{', '}', '(', ')', ',', ';', ' ', '\n', '\t', '.',
}

func hasTerminateChar(input string) bool {
	if input == "" {
		return false
	}

	return slices.Contains(terminatingChars, []rune(input)[0])
}

func (s *session) Update(ctx gvcode.CompletionContext, editor *gvcode.Editor) []gvcode.CompletionCandidate {
	if s.canceled {
		return nil
	}

	if s.state.triggered {
		s.candidates = s.state.completor.Suggest(ctx)
		s.state.triggerChars = ctx.Input
		s.state.triggered = false
		s.setupPrefix(editor, ctx)
	} else {
		s.syncPrefix(editor, ctx)
	}

	if hasTerminateChar(ctx.Input) && ctx.Input != s.state.triggerChars {
		s.makeInvalid()
		return nil
	}

	// If nothing was typed and the prefix is empty, there is nothing to complete
	// anymore — cancel the session. Key-triggered sessions are exempt: pressing
	// Ctrl+Space with nothing at the caret should still show all completions.
	if ctx.Input == "" && len(s.prefix) == 0 && s.state.triggerKind != keyTrigger {
		s.makeInvalid()
		return nil
	}

	s.ctx = ctx
	return s.state.completor.FilterAndRank(string(s.prefix), s.candidates)
}

// setupPrefix initializes the prefix range and prefix for a newly triggered session.
func (s *session) setupPrefix(editor *gvcode.Editor, ctx gvcode.CompletionContext) {
	s.prefixRange.End = ctx.Position

	switch s.state.triggerKind {
	case keyTrigger:
		// Key-triggered — prefix starts at current caret.
		s.prefixRange.Start = ctx.Position
		s.prefixRange.Start.Runes = ctx.Position.Runes
		s.prefix = s.prefix[:0]
	default:
		if isSymbolChar([]rune(ctx.Input)[0]) {
			start := ctx.Position
			start.Column = max(0, start.Column-len([]rune(ctx.Input)))
			start.Runes = ctx.Position.Runes - len([]rune(ctx.Input))
			s.prefixRange.Start = start
			if editor != nil {
				s.prefix = []rune(editor.ReadTextBetween(start.Runes, ctx.Position.Runes))
			}
		} else {
			// Non-identifier trigger char — prefix starts at current caret, not
			// at the trigger character.
			s.prefixRange.Start = ctx.Position
			s.prefixRange.Start.Runes = ctx.Position.Runes
			s.prefix = s.prefix[:0]
		}
	}
}

// syncPrefix reads the current prefix from the editor buffer for an already active session.
func (s *session) syncPrefix(editor *gvcode.Editor, ctx gvcode.CompletionContext) {
	if editor == nil {
		return
	}
	s.prefix = []rune(editor.ReadTextBetween(s.prefixRange.Start.Runes, ctx.Position.Runes))
	// Only extend the end of the prefix range; never shrink it. Cursor
	// movement within the prefix should not reduce the replacement range
	// that OnConfirm uses — only the filter prefix should change.
	if ctx.Position.Runes > s.prefixRange.End.Runes {
		s.prefixRange.End = ctx.Position
	}
}

func (s *session) makeInvalid() {
	s.canceled = true
	s.prefix = s.prefix[:0]
	s.prefixRange = gvcode.EditRange{}
	s.candidates = s.candidates[:0]
}

func (s *session) IsValid() bool {
	return s != nil && s.state != nil && !s.canceled
}

// Prefix returns text buffered since the session is triggered.
func (s *session) Prefix() string {
	return string(s.prefix)
}

func (s *session) PrefixRange() gvcode.EditRange {
	return s.prefixRange
}

func (s *session) Completor() *delegatedCompletor {
	return s.state.completor
}
