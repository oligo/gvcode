package completion

import (
	"errors"
	"image"
	"slices"
	"strings"

	"gioui.org/io/key"
	"gioui.org/layout"
	"github.com/oligo/gvcode"
)

var _ gvcode.Completion = (*DefaultCompletion)(nil)

// DefaultCompletion is a built-in implementation of the gvcode.Completion API.
type DefaultCompletion struct {
	Editor     *gvcode.Editor
	completors []*delegatedCompletor
	candidates []gvcode.CompletionCandidate
	session    *session
}

type delegatedCompletor struct {
	popup gvcode.CompletionPopup
	gvcode.Completor
}

func (dc *DefaultCompletion) AddCompletor(completor gvcode.Completor, popup gvcode.CompletionPopup) error {
	c := &delegatedCompletor{
		Completor: completor,
		popup:     popup,
	}

	trigger := completor.Trigger()

	duplicatedKey := slices.ContainsFunc(dc.completors, func(cm *delegatedCompletor) bool {
		tr := cm.Completor.Trigger()
		return tr.KeyBinding.Name == trigger.KeyBinding.Name && tr.KeyBinding.Modifiers == trigger.KeyBinding.Modifiers
	})
	if duplicatedKey {
		return errors.New("duplicated key binding")
	}

	if trigger.KeyBinding.Name != "" && trigger.KeyBinding.Modifiers != 0 {
		dc.Editor.RegisterCommand(dc,
			key.Filter{Name: trigger.KeyBinding.Name, Required: trigger.KeyBinding.Modifiers},
			func(gtx layout.Context, evt key.Event) gvcode.EditorEvent {
				dc.onKey(evt)
				return nil
			})
	}

	dc.completors = append(dc.completors, c)
	return nil
}

// onKey activates the completor when the registered key binding are pressed.
// The execution of the activated completor is repeated as the user type ahead,
// which is run by the OnText method.
func (dc *DefaultCompletion) onKey(evt key.Event) {
	// cancel existing completion.
	dc.Cancel()

	var cmp *delegatedCompletor
	for _, c := range dc.completors {
		if c.Trigger().ActivateOnKey(evt) {
			cmp = c
			break
		}
	}

	if cmp == nil {
		return
	}

	ctx := dc.Editor.GetCompletionContext()
	dc.session = newSession(cmp, keyTrigger)
	dc.updateCandidates(dc.session.Update(ctx))
}

func (dc *DefaultCompletion) OnText(ctx gvcode.CompletionContext) {
	if ctx.Input == "" {
		dc.Cancel()
		return
	}

	if dc.session != nil && dc.session.IsValid() {
		dc.updateCandidates(dc.session.Update(ctx))
		if dc.session.IsValid() {
			return
		}
		// Session became invalid (e.g., terminated by a trigger char like ".").
		// Fall through to check if input can start a new completion session.
	}

	var completor *delegatedCompletor
	var kind triggerKind

	for _, cmp := range dc.completors {
		if canTrigger(cmp.Trigger(), ctx.Input) {
			completor = cmp
			kind = charTrigger
			break
		}
	}

	if completor != nil {
		if dc.session == nil || !dc.session.IsValid() {
			dc.session = newSession(completor, kind)
		}

		dc.updateCandidates(dc.session.Update(ctx))
	}

}

func (dc *DefaultCompletion) updateCandidates(candidates []gvcode.CompletionCandidate) {
	dc.candidates = dc.candidates[:0]
	dc.candidates = append(dc.candidates, candidates...)
}

func (dc *DefaultCompletion) IsActive() bool {
	return dc.session != nil && dc.session.IsValid()
}

func (dc *DefaultCompletion) Offset() image.Point {
	if dc.session == nil {
		return image.Point{}
	}

	return dc.session.ctx.Coords
}

func (dc *DefaultCompletion) Layout(gtx layout.Context) layout.Dimensions {
	if dc.session == nil {
		return layout.Dimensions{}
	}

	completor := dc.session.Completor()
	// when a session is marked as invalid, we'll have to still layout once to
	// reset the popup to unregister the event handler.
	return completor.popup.Layout(gtx, dc.candidates)
}

func (dc *DefaultCompletion) Cancel() {
	if dc.session != nil && dc.session.IsValid() {
		dc.session.makeInvalid()
	}
	dc.candidates = dc.candidates[:0]
}

func (dc *DefaultCompletion) OnConfirm(idx int) {
	if dc.Editor == nil {
		return
	}
	if idx < 0 || idx >= len(dc.candidates) {
		return
	}

	candidate := dc.candidates[idx]
	editRange := candidate.TextEdit.EditRange

	// Merge the LSP's edit range with our tracked prefix range to ensure we
	// replace both the LSP's detected token and everything the user typed.
	editRange = mergeRange(editRange, dc.session.PrefixRange())

	caretStart, caretEnd := editRange.Start.Runes, editRange.End.Runes
	// Assume line/column is set, convert the line/column position to rune offsets.
	if caretStart <= 0 && caretEnd <= 0 {
		caretStart, _ = dc.Editor.ConvertPos(editRange.Start.Line, editRange.Start.Column)
		caretEnd, _ = dc.Editor.ConvertPos(editRange.End.Line, editRange.End.Column)
	}
	// set the selection using range provided by the completor.
	dc.Editor.SetCaret(caretStart, caretEnd)

	if strings.ToLower(candidate.TextFormat) == "snippet" {
		_, err := dc.Editor.InsertSnippet(candidate.TextEdit.NewText)
		if err != nil {
			logger.Error("insert snippet failed", "error", err)
		}
	} else {
		dc.Editor.Insert(candidate.TextEdit.NewText)
	}
	dc.Cancel()
}

// mergeRange merges two edit ranges on the same line, returning a range that
// covers both. This ensures we replace everything: the LSP's detected token
// start and everything the user typed.
func mergeRange(r1, r2 gvcode.EditRange) gvcode.EditRange {
	// If either range is empty, return the other
	if r1 == (gvcode.EditRange{}) {
		return r2
	}
	if r2 == (gvcode.EditRange{}) {
		return r1
	}

	// Only merge if on the same line
	if r1.Start.Line != r2.Start.Line || r1.End.Line != r2.End.Line {
		return r1
	}

	// Take the earlier start and later end
	result := r1
	if r2.Start.Column < result.Start.Column {
		result.Start = r2.Start
	}
	if r2.End.Column > result.End.Column {
		result.End = r2.End
	}
	return result
}

func isSymbolChar(ch rune) bool {
	if (ch >= 'a' && ch <= 'z') ||
		(ch >= 'A' && ch <= 'Z') ||
		(ch >= '0' && ch <= '9') ||
		ch == '_' {
		return true
	}

	return false
}

func canTrigger(tr gvcode.Trigger, input string) bool {
	// Check explicit trigger characters first.
	if slices.Contains(tr.Characters, input) {
		return true
	}

	// Always allow symbol characters to trigger completion.
	return isSymbolChar([]rune(input)[0])
}
