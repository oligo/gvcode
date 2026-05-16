package completion

import (
	"slices"

	"github.com/oligo/gvcode"
)

// DefaultTriggerPolicy preserves the built-in programming-language completion
// behavior: explicit trigger characters and symbol characters can start a
// session, symbol-triggered prefixes include the typed symbol, and common
// separators cancel the session.
type DefaultTriggerPolicy struct{}

// ExplicitTriggerPolicy starts completion only for Trigger.Characters. It is a
// useful base policy for slash commands, mentions, and resource completions.
type ExplicitTriggerPolicy struct {
	DefaultTriggerPolicy
}

var terminatingChars = []rune{
	'{', '}', '(', ')', ',', ';', ' ', '\n', '\t', '.',
}

func policyForTrigger(tr gvcode.Trigger) gvcode.TriggerPolicy {
	if tr.Policy != nil {
		return tr.Policy
	}
	return DefaultTriggerPolicy{}
}

func (DefaultTriggerPolicy) CanStart(tr gvcode.Trigger, ctx gvcode.CompletionContext) bool {
	if ctx.Input == "" {
		return false
	}

	// Check explicit trigger characters first.
	if slices.Contains(tr.Characters, ctx.Input) {
		return true
	}

	// Always allow symbol characters to trigger completion.
	return isSymbolChar([]rune(ctx.Input)[0])
}

func (DefaultTriggerPolicy) PrefixRange(_ gvcode.Trigger, kind gvcode.CompletionTriggerKind, ctx gvcode.CompletionContext) gvcode.EditRange {
	prefixRange := gvcode.EditRange{
		Start: ctx.Position,
		End:   ctx.Position,
	}

	if kind == gvcode.KeyTrigger {
		return prefixRange
	}

	if ctx.Input != "" && isSymbolChar([]rune(ctx.Input)[0]) {
		inputLen := len([]rune(ctx.Input))
		prefixRange.Start.Column = max(0, ctx.Position.Column-inputLen)
		prefixRange.Start.Runes = ctx.Position.Runes - inputLen
	}

	return prefixRange
}

func (DefaultTriggerPolicy) ShouldCancel(_ gvcode.Trigger, kind gvcode.CompletionTriggerKind, triggerInput string, ctx gvcode.CompletionContext, prefix string) bool {
	if hasTerminateChar(ctx.Input) && ctx.Input != triggerInput {
		return true
	}

	// If nothing was typed and the prefix is empty, there is nothing to complete
	// anymore. Key-triggered sessions are exempt: pressing Ctrl+Space with
	// nothing at the caret should still show all completions.
	return ctx.Input == "" && prefix == "" && kind != gvcode.KeyTrigger
}

func (ExplicitTriggerPolicy) CanStart(tr gvcode.Trigger, ctx gvcode.CompletionContext) bool {
	return ctx.Input != "" && slices.Contains(tr.Characters, ctx.Input)
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

func hasTerminateChar(input string) bool {
	if input == "" {
		return false
	}

	return slices.Contains(terminatingChars, []rune(input)[0])
}
