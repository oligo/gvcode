package gvcode

import "github.com/oligo/gvcode/internal/buffer"

// Re-export internal marker types for public use.
// These are transparent type aliases for buffer.Marker.
type (
	Marker     = buffer.Marker
	MarkerBias = buffer.MarkerBias
)

const (
	BiasForward  = buffer.BiasForward
	BiasBackward = buffer.BiasBackward
)

// CreateMarker creates a position-tracking marker at the given rune offset.
// The marker survives text edits — call marker.Offset() later for the
// updated position. Returns an error if the rune offset is out of range.
//
// Use BiasForward to have the marker move with inserted text,
// BiasBackward to keep it anchored.
func (e *Editor) CreateMarker(runeOff int, bias MarkerBias) (*Marker, error) {
	e.initBuffer()

	return e.buffer.CreateMarker(runeOff, bias)
}

// RemoveMarker removes a previously created marker, releasing its resources.
func (e *Editor) RemoveMarker(marker *Marker) {
	e.initBuffer()
	e.buffer.RemoveMarker(marker)
}
