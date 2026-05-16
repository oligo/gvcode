package completion

import "github.com/oligo/gvcode"

type triggerState struct {
	triggerKind gvcode.CompletionTriggerKind
	// the activated completor.
	completor    *delegatedCompletor
	policy       gvcode.TriggerPolicy
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

func newSession(completor *delegatedCompletor, kind gvcode.CompletionTriggerKind) *session {
	return &session{
		state: &triggerState{
			triggerKind: kind,
			completor:   completor,
			policy:      policyForTrigger(completor.Trigger()),
			triggered:   true,
		},
	}
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

	if s.state.policy.ShouldCancel(s.state.completor.Trigger(), s.state.triggerKind, s.state.triggerChars, ctx, string(s.prefix)) {
		s.makeInvalid()
		return nil
	}

	s.ctx = ctx
	return s.state.completor.FilterAndRank(string(s.prefix), s.candidates)
}

// setupPrefix initializes the prefix range and prefix for a newly triggered session.
func (s *session) setupPrefix(editor *gvcode.Editor, ctx gvcode.CompletionContext) {
	s.prefixRange = s.state.policy.PrefixRange(s.state.completor.Trigger(), s.state.triggerKind, ctx)
	if editor != nil {
		s.prefix = []rune(editor.ReadTextBetween(s.prefixRange.Start.Runes, ctx.Position.Runes))
		return
	}
	s.prefix = s.prefix[:0]
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
