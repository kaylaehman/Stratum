// Package templates renders saved Docker Compose stack templates (Feature 14):
// substitutes {{VAR}} tokens from supplied values (falling back to per-variable
// defaults) and reports any tokens left unresolved. Pure + unit-tested; the
// deploy orchestration lives in the API layer.
package templates

import (
	"regexp"
	"sort"

	"github.com/kaylaehman/stratum/backend/db"
)

// tokenRe matches {{ NAME }} with optional surrounding whitespace.
var tokenRe = regexp.MustCompile(`\{\{\s*([A-Za-z0-9_]+)\s*\}\}`)

// Render substitutes {{VAR}} tokens in compose using values (which override the
// variables' defaults). It returns the rendered text and the sorted, de-duped
// list of tokens that had no value and no default (left untouched in the
// output). A non-empty unresolved list means the template isn't ready to deploy.
func Render(compose string, vars []db.TemplateVar, values map[string]string) (string, []string) {
	eff := map[string]string{}
	for _, v := range vars {
		if v.Default != "" {
			eff[v.Name] = v.Default
		}
	}
	for k, val := range values {
		if val != "" {
			eff[k] = val
		}
	}

	unresolved := map[string]bool{}
	out := tokenRe.ReplaceAllStringFunc(compose, func(m string) string {
		name := tokenRe.FindStringSubmatch(m)[1]
		if val, ok := eff[name]; ok {
			return val
		}
		unresolved[name] = true
		return m
	})

	names := make([]string, 0, len(unresolved))
	for n := range unresolved {
		names = append(names, n)
	}
	sort.Strings(names)
	return out, names
}
