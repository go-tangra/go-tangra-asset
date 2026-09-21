package httpapi

import (
	"strings"

	"github.com/go-freya/freya/services/asset/internal/authz"
)

// subjectsT is the caller subject handlers receive.
type subjectsT = authz.Subjects

func containsFold(s, sub string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(sub))
}

// sanitizeFilename keeps a Content-Disposition filename header-safe.
func sanitizeFilename(name string) string {
	var b strings.Builder
	for _, r := range name {
		switch {
		case r == '"' || r == '\\' || r < 0x20 || r == 0x7f:
			b.WriteRune('_')
		default:
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 {
		return "download"
	}
	return b.String()
}
