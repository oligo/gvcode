package gvcode

// Mode defines a mode for the editor. The editor can be switched
// back and forth bewteen different modes, depending on the context.
type EditorMode uint8

const (
	// ModeNormal is the default mode for the editor. Users can
	// insert or select text or anything else.
	ModeNormal EditorMode = iota

	// ModeReadOnly controls whether the contents of the editor can be
	// altered by user interaction. If set, the editor will allow selecting
	// text and copying it interactively, but not modifying it. Users can
	// enter or quit this mode via user commands.
	ModeReadOnly

	// ModeSnippet put the editor into the snippet mode required by LSP protocol.
	// The users can navigate bewteen the snippet locations/placeholders using
	// the tab/shift-tab keys. And clicking or pressing the ESC key switched the
	// editor to normal mode.
	//
	// See https://microsoft.github.io/language-server-protocol/specifications/lsp/3.17/specification/#snippet_syntax.
	ModeSnippet

	// ModeColumnEdit enables column (vertical) selection mode, similar to
	// GoLand, VS Code, and other modern editors. Users can select a rectangular
	// block of text across multiple lines and edit them simultaneously.
	ModeColumnEdit
)

func (e *Editor) setMode(mode EditorMode) {
	switch mode {
	case ModeNormal, ModeReadOnly:
		if e.snippetCtx != nil {
			e.snippetCtx.Cancel()
			e.snippetCtx = nil
		}
		// Disable column editing when exiting column edit mode
		if e.mode == ModeColumnEdit && mode != ModeColumnEdit {
			e.clearColumnEdit()
		}
	}

	e.mode = mode
}

// SetColumnEditMode enables or disables column editing mode
func (e *Editor) SetColumnEditMode(enabled bool) {
	println("[ColumnEdit] SetColumnEditMode called with enabled:", enabled, "current mode:", e.mode)
	if enabled {
		e.mode = ModeColumnEdit
		e.columnEdit.enabled = true
		println("[ColumnEdit] Column editing mode enabled")
	} else {
		e.clearColumnEdit()
	}
}

// clearColumnEdit clears all column selections and disables column edit mode
func (e *Editor) clearColumnEdit() {
	println("[ColumnEdit] clearColumnEdit called, clearing", len(e.columnEdit.selections), "selections")
	e.columnEdit.enabled = false
	e.columnEdit.selections = nil
	if e.mode == ModeColumnEdit {
		e.mode = ModeNormal
		println("[ColumnEdit] Column editing mode disabled")
	}
}

// ColumnEditEnabled returns whether column editing mode is active
func (e *Editor) ColumnEditEnabled() bool {
	return e.columnEdit.enabled || e.mode == ModeColumnEdit
}
