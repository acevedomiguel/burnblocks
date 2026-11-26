package kimi

import (
	"io"

	"github.com/acevedomiguel/burnblocks/internal/kimi/projection"
)

// PublicSchemaVersion is the stable public JSON schema version. It is
// independent of the StoredState and snapshot versions.
const PublicSchemaVersion = projection.PublicSchemaVersion

// JSONOptions supplies transport-independent output metadata.
type JSONOptions = projection.ProjectionOptions

// WriteJSON emits one indented schema-1 document and a trailing newline to the
// caller-selected writer.
func WriteJSON(w io.Writer, snapshot Snapshot, opts JSONOptions) error {
	return projection.WriteJSON(w, snapshot, opts)
}
