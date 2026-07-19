package db

import "strings"

// buildFTSQuery transforms a user-typed search string into an FTS5
// prefix-match query. Each whitespace-separated token gets a trailing
// '*', so "mana" becomes "mana*" — matching manager, management, etc.
// Existing '*' suffixes are stripped first to avoid double-appending.
//
// Mirrors learn-tool's internal/db/fts_query.go.
func buildFTSQuery(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	var b strings.Builder
	for _, tok := range strings.Fields(s) {
		tok = strings.TrimRight(tok, "*")
		if tok == "" {
			continue
		}
		if b.Len() > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(tok)
		b.WriteByte('*')
	}
	return b.String()
}
