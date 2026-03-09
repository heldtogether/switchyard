package api

import "strings"

var reservedUISlugs = map[string]struct{}{
	"runs":          {},
	"jobs":          {},
	"artefacts":     {},
	"billing":       {},
	"executors":     {},
	"settings":      {},
	"login":         {},
	"accept-invite": {},
	"api":           {},
}

func isReservedSlug(slug string) bool {
	normalized := strings.ToLower(strings.TrimSpace(slug))
	_, ok := reservedUISlugs[normalized]
	return ok
}
