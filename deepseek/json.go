package deepseek

import (
	"io"

	"github.com/acevedomiguel/burnblocks/internal/deepseek/projection"
)

// Public JSON contract vocabulary.
type (
	// Document is the versioned public JSON contract.
	Document = projection.Document
	// JSONBalance is one currency's balances in the public contract.
	JSONBalance = projection.JSONBalance
	// JSONRunway is the forecast section.
	JSONRunway = projection.JSONRunway
)

// BuildJSON converts a snapshot into the stable contract.
func BuildJSON(snap Snapshot) Document { return projection.BuildJSON(snap) }

// MarshalJSON renders indented JSON with a trailing newline.
func MarshalJSON(snap Snapshot) ([]byte, error) { return projection.MarshalJSON(snap) }

// WriteJSON writes indented JSON and a trailing newline to w.
func WriteJSON(w io.Writer, snap Snapshot) error { return projection.WriteJSON(w, snap) }
