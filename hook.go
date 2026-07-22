package gvcode

import "gioui.org/io/key"

// BeforePasteHook defines a hook to be called before pasting text to transform the text.
type BeforePasteHook func(text string) string

// EnterHook is called when Enter/Return is pressed.
// Return true to skip the default behavior.
// The hook receives the Editor and should mutate it directly
type EnterHook func(ed *Editor) bool

// TextInputHook is called before text input processing.
// ke is the input event containing the character and replacement range.
// Return true to skip the default processing.
//
// The hook receives the Editor so it can read context and insert text.
type TextInputHook func(ed *Editor, ke key.EditEvent) bool
