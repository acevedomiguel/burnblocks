package openrouter

import (
	"io"

	"github.com/acevedomiguel/burnblocks/internal/openrouter/projection"
)

// Schema-2 JSON contract vocabulary.
type (
	JSONSnapshot = projection.JSONSnapshot
	JSONAccount  = projection.JSONAccount
	JSONLast30   = projection.JSONLast30
	JSONRunway   = projection.JSONRunway
	JSONKey      = projection.JSONKey
)

// BuildJSON converts a snapshot into the stable schema-2 contract.
func BuildJSON(snap Snapshot, cached bool) JSONSnapshot {
	return projection.BuildJSON(snap, cached)
}

// MarshalJSON renders indented schema-2 JSON with a trailing newline.
func MarshalJSON(snap Snapshot, cached bool) ([]byte, error) {
	return projection.MarshalJSON(snap, cached)
}

// WriteJSON writes indented schema-2 JSON and a trailing newline to w.
func WriteJSON(w io.Writer, snap Snapshot, cached bool) error {
	data, err := MarshalJSON(snap, cached)
	if err != nil {
		return err
	}
	n, err := w.Write(data)
	if err != nil {
		return err
	}
	if n != len(data) {
		return io.ErrShortWrite
	}
	return nil
}
