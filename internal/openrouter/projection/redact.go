package projection

import (
	"github.com/acevedomiguel/burnblocks/internal/openrouter/credentials"
)

// credRedact scrubs anything that looks like a credential from a message.
func credRedact(s string) string {
	return credentials.Redact(s)
}
