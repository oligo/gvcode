package gvcode

import (
	"testing"

	"gioui.org/io/key"
	"gioui.org/layout"
)

func TestKeyFilterMatch(t *testing.T) {
	tests := []struct {
		name   string
		filter key.Filter
		event  key.Event
		match  bool
	}{
		{
			name:   "exact name match",
			filter: key.Filter{Name: key.NameEnter},
			event:  key.Event{Name: key.NameEnter},
			match:  true,
		},
		{
			name:   "different name no match",
			filter: key.Filter{Name: key.NameEnter},
			event:  key.Event{Name: key.NameTab},
			match:  false,
		},
		{
			name:   "empty name matches any",
			filter: key.Filter{Name: ""},
			event:  key.Event{Name: "X"},
			match:  true,
		},
		{
			name:   "required modifiers present",
			filter: key.Filter{Name: "C", Required: key.ModShortcut},
			event:  key.Event{Name: "C", Modifiers: key.ModShortcut},
			match:  true,
		},
		{
			name:   "required modifiers missing",
			filter: key.Filter{Name: "C", Required: key.ModShortcut},
			event:  key.Event{Name: "C"},
			match:  false,
		},
		{
			name:   "required modifiers partially present",
			filter: key.Filter{Name: "C", Required: key.ModShortcut | key.ModShift},
			event:  key.Event{Name: "C", Modifiers: key.ModShortcut},
			match:  false,
		},
		{
			name:   "optional modifiers present",
			filter: key.Filter{Name: "Z", Required: key.ModShortcut, Optional: key.ModShift},
			event:  key.Event{Name: "Z", Modifiers: key.ModShortcut | key.ModShift},
			match:  true,
		},
		{
			name:   "optional modifiers absent",
			filter: key.Filter{Name: "Z", Required: key.ModShortcut, Optional: key.ModShift},
			event:  key.Event{Name: "Z", Modifiers: key.ModShortcut},
			match:  true,
		},
		{
			name:   "extra modifier not allowed",
			filter: key.Filter{Name: "C", Required: key.ModShortcut},
			event:  key.Event{Name: "C", Modifiers: key.ModShortcut | key.ModShift},
			match:  false,
		},
		{
			name:   "extra modifier within optional",
			filter: key.Filter{Name: "Z", Required: key.ModShortcut, Optional: key.ModShift | key.ModAlt},
			event:  key.Event{Name: "Z", Modifiers: key.ModShortcut | key.ModShift},
			match:  true,
		},
		{
			name:   "extra modifier outside optional",
			filter: key.Filter{Name: "Z", Required: key.ModShortcut, Optional: key.ModShift},
			event:  key.Event{Name: "Z", Modifiers: key.ModShortcut | key.ModAlt},
			match:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := keyFilterMatch(tt.filter, tt.event)
			if got != tt.match {
				t.Errorf("keyFilterMatch(%+v, %+v) = %v, want %v",
					tt.filter, tt.event, got, tt.match)
			}
		})
	}
}

func TestMatchedCmd(t *testing.T) {
	cmdA := keyCommand{tag: "a", filter: key.Filter{Name: key.NameEnter}}
	cmdB := keyCommand{tag: "b", filter: key.Filter{Name: key.NameTab}}
	cmdC := keyCommand{tag: "c", filter: key.Filter{Name: key.NameEnter, Required: key.ModShift}}

	t.Run("finds first matching command", func(t *testing.T) {
		cmds := []keyCommand{cmdA, cmdB, cmdC}
		evt := key.Event{Name: key.NameTab}
		idx := matchedCmd(evt, cmds)
		if idx != 1 {
			t.Errorf("matchedCmd for Tab event = %d, want 1", idx)
		}
	})

	t.Run("returns -1 when no match", func(t *testing.T) {
		cmds := []keyCommand{cmdA, cmdB}
		evt := key.Event{Name: "Esc"}
		idx := matchedCmd(evt, cmds)
		if idx != -1 {
			t.Errorf("matchedCmd for unmatched event = %d, want -1", idx)
		}
	})

	t.Run("first in slice wins on multiple matches", func(t *testing.T) {
		cmds := []keyCommand{cmdA, cmdC}
		evt := key.Event{Name: key.NameEnter}
		idx := matchedCmd(evt, cmds)
		if idx != 0 {
			t.Errorf("matchedCmd matched cmd at index %d, want 0 (first match)", idx)
		}
	})

	t.Run("empty slice returns -1", func(t *testing.T) {
		idx := matchedCmd(key.Event{Name: key.NameEnter}, nil)
		if idx != -1 {
			t.Errorf("matchedCmd with empty cmds = %d, want -1", idx)
		}
	})
}

func newTestEditor() *Editor {
	return &Editor{}
}

func TestRegisterCommand(t *testing.T) {
	handler := func(gtx layout.Context, evt key.Event) EditorEvent { return nil }

	t.Run("registers first command", func(t *testing.T) {
		e := newTestEditor()
		// Pre-populate commands to avoid buildBuiltinCommands.
		e.commands = make(map[key.Name][]keyCommand)
		e.commands[key.NameEnter] = []keyCommand{{tag: "dummy"}}

		e.RegisterCommand("test", key.Filter{Name: "F1"}, handler)
		cmds := e.commands["F1"]
		if len(cmds) != 1 {
			t.Fatalf("expected 1 command for F1, got %d", len(cmds))
		}
		if cmds[0].tag != "test" {
			t.Errorf("tag = %v, want 'test'", cmds[0].tag)
		}
	})

	t.Run("appends to existing key group", func(t *testing.T) {
		e := newTestEditor()
		e.commands = make(map[key.Name][]keyCommand)
		// Add a dummy entry so buildBuiltinCommands is not triggered.
		e.commands[key.NameEnter] = []keyCommand{{tag: "dummy"}}

		e.RegisterCommand("first", key.Filter{Name: "F2"}, handler)
		e.RegisterCommand("second", key.Filter{Name: "F2", Required: key.ModShift}, handler)

		cmds := e.commands["F2"]
		if len(cmds) != 2 {
			t.Fatalf("expected 2 commands for F2, got %d", len(cmds))
		}
	})

	t.Run("overwrites command with same filter and tag", func(t *testing.T) {
		e := newTestEditor()
		e.commands = make(map[key.Name][]keyCommand)
		e.commands[key.NameEnter] = []keyCommand{{tag: "dummy"}}

		e.RegisterCommand("dup", key.Filter{Name: "F3"}, handler)
		count := 0
		countingHandler := func(gtx layout.Context, evt key.Event) EditorEvent {
			count++
			return nil
		}
		e.RegisterCommand("dup", key.Filter{Name: "F3"}, countingHandler)

		cmds := e.commands["F3"]
		if len(cmds) != 1 {
			t.Fatalf("expected 1 command after overwrite, got %d", len(cmds))
		}
	})

	t.Run("ignores nil tag", func(t *testing.T) {
		e := newTestEditor()
		e.commands = make(map[key.Name][]keyCommand)
		e.commands[key.NameEnter] = []keyCommand{{tag: "dummy"}}

		e.RegisterCommand(nil, key.Filter{Name: "F4"}, handler)
		if _, exists := e.commands["F4"]; exists {
			t.Error("command with nil tag should not be registered")
		}
	})

	t.Run("ignores nil handler", func(t *testing.T) {
		e := newTestEditor()
		e.commands = make(map[key.Name][]keyCommand)
		e.commands[key.NameEnter] = []keyCommand{{tag: "dummy"}}

		e.RegisterCommand("test", key.Filter{Name: "F5"}, nil)
		if _, exists := e.commands["F5"]; exists {
			t.Error("command with nil handler should not be registered")
		}
	})

	t.Run("sets filter focus to editor", func(t *testing.T) {
		e := newTestEditor()
		e.commands = make(map[key.Name][]keyCommand)
		e.commands[key.NameEnter] = []keyCommand{{tag: "dummy"}}

		e.RegisterCommand("test", key.Filter{Name: "F6"}, handler)
		cmds := e.commands["F6"]
		if len(cmds) != 1 {
			t.Fatalf("expected 1 command, got %d", len(cmds))
		}
		if cmds[0].filter.Focus != e {
			t.Error("filter.Focus should be set to the editor")
		}
	})
}

func TestRemoveCommands(t *testing.T) {
	handler := func(gtx layout.Context, evt key.Event) EditorEvent { return nil }

	t.Run("removes commands with matching tag", func(t *testing.T) {
		e := newTestEditor()
		e.commands = map[key.Name][]keyCommand{
			"F1": {{tag: "keep"}, {tag: "remove"}},
			"F2": {{tag: "remove"}},
		}

		e.RemoveCommands("remove")
		if len(e.commands["F1"]) != 1 {
			t.Errorf("F1 group: expected 1 remaining, got %d", len(e.commands["F1"]))
		}
		if e.commands["F1"][0].tag != "keep" {
			t.Errorf("F1 remaining command tag = %v, want 'keep'", e.commands["F1"][0].tag)
		}
		if _, exists := e.commands["F2"]; exists {
			t.Error("F2 group should be deleted when empty")
		}
	})

	t.Run("no-op when tag has no commands", func(t *testing.T) {
		e := newTestEditor()
		e.commands = map[key.Name][]keyCommand{
			"F1": {{tag: "keep"}},
		}
		e.RemoveCommands("nonexistent")
		if len(e.commands["F1"]) != 1 {
			t.Error("unrelated commands should not be affected")
		}
	})

	t.Run("ignores nil handler", func(t *testing.T) {
		e := newTestEditor()
		e.commands = make(map[key.Name][]keyCommand)
		e.commands[key.NameEnter] = []keyCommand{{tag: "dummy"}}

		e.RegisterCommand(nil, key.Filter{Name: "F4"}, handler)
		if _, exists := e.commands["F4"]; exists {
			t.Error("command with nil tag should not be registered")
		}
	})
}

func TestBuildBuiltinCommands(t *testing.T) {
	e := newTestEditor()
	e.buildBuiltinCommands()

	// Check that expected key groups exist.
	expectedNames := []key.Name{
		key.NameEnter,
		key.NameReturn,
		"C",
		"V",
		"X",
		"Z",
		"A",
		key.NameHome,
		key.NameEnd,
		key.NameTab,
		key.NameDeleteBackward,
		key.NameDeleteForward,
		key.NamePageUp,
		key.NamePageDown,
		key.NameLeftArrow,
		key.NameRightArrow,
		key.NameUpArrow,
		key.NameDownArrow,
	}

	for _, name := range expectedNames {
		cmds, exists := e.commands[name]
		if !exists {
			t.Errorf("missing builtin command for %q", name)
			continue
		}
		if len(cmds) < 1 {
			t.Errorf("empty command list for %q", name)
		}
		// Verify filter has correct Name and Focus.
		for _, cmd := range cmds {
			if cmd.filter.Name != name {
				t.Errorf("filter name mismatch: %q != %q", cmd.filter.Name, name)
			}
			if cmd.filter.Focus != e {
				t.Errorf("filter focus should be editor for %q", name)
			}
		}
	}

	// Verify Enter has Shift as optional modifier.
	enterCmd := e.commands[key.NameEnter][0]
	if enterCmd.filter.Optional&key.ModShift == 0 {
		t.Error("Enter command should have Shift as optional modifier")
	}

	// Verify Ctrl+Z has Shift as optional (for redo).
	undoCmd := e.commands["Z"][0]
	if undoCmd.filter.Optional&key.ModShift == 0 {
		t.Error("Ctrl+Z command should have Shift as optional modifier")
	}

	t.Run("clears before building", func(t *testing.T) {
		e := newTestEditor()
		e.commands = map[key.Name][]keyCommand{
			"CustomKey": {{tag: "custom"}},
		}
		e.buildBuiltinCommands()
		if _, exists := e.commands["CustomKey"]; exists {
			t.Error("buildBuiltinCommands should clear previous commands")
		}
	})
}

func TestRegisterCommandWithBuiltins(t *testing.T) {
	handler := func(gtx layout.Context, evt key.Event) EditorEvent { return nil }

	t.Run("last registered wins for matching filter", func(t *testing.T) {
		e := newTestEditor()

		// First call: triggers buildBuiltinCommands, then registers our command.
		e.RegisterCommand("custom", key.Filter{Name: key.NameEnter}, handler)

		cmds := e.commands[key.NameEnter]
		if len(cmds) < 2 {
			t.Fatalf("expected at least 2 commands (builtin + custom), got %d", len(cmds))
		}
		// The custom command should be last (appended after builtin).
		last := cmds[len(cmds)-1]
		if last.tag != "custom" {
			t.Errorf("custom command should be last in slice, got tag %v", last.tag)
		}
	})

	t.Run("overwriting custom command replaces not appends", func(t *testing.T) {
		e := newTestEditor()
		e.commands = map[key.Name][]keyCommand{
			key.NameEnter: {{tag: "dummy"}},
		}

		handler2 := func(gtx layout.Context, evt key.Event) EditorEvent { return nil }
		e.RegisterCommand("custom", key.Filter{Name: key.NameEnter}, handler)
		count := len(e.commands[key.NameEnter])
		e.RegisterCommand("custom", key.Filter{Name: key.NameEnter}, handler2)

		if len(e.commands[key.NameEnter]) != count {
			t.Errorf("overwriting with same tag+filter should not increase count: %d -> %d",
				count, len(e.commands[key.NameEnter]))
		}
	})
}
