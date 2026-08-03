package httpapi

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// localeTagPattern accepts an IETF-ish tag such as "en", "ru" or "pt-BR".
//
// The parameter is used to build a file path, so it is matched against this
// pattern rather than sanitised: anything containing a separator, a dot or a
// parent reference simply does not match and never reaches the filesystem.
var localeTagPattern = regexp.MustCompile(`^[a-z]{2,3}(-[A-Za-z0-9]{2,8})?$`)

// LocaleOptions configures where translation catalogs are read from.
type LocaleOptions struct {
	// Dir holds one <tag>.json file per available language. Operators mount a
	// volume here to add or override translations without rebuilding the image.
	Dir string
}

func (a *API) WithLocales(options LocaleOptions) *API {
	a.locales = options
	return a
}

// handleLocales lists the languages the deployment can serve.
func (a *API) handleLocales(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed", "use GET")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"locales": a.availableLocales()})
}

// handleLocaleByTag serves one catalog.
func (a *API) handleLocaleByTag(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed", "use GET")
		return
	}
	tag := strings.TrimSuffix(strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/locales/"), "/"), ".json")
	if tag == "" {
		a.handleLocales(w, r)
		return
	}
	if !localeTagPattern.MatchString(tag) {
		writeError(w, http.StatusBadRequest, "INVALID_LOCALE", "invalid language tag", "use a tag such as en or pt-BR")
		return
	}
	if a.locales.Dir == "" {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "translation not found", "no locale directory is configured")
		return
	}

	raw, err := os.ReadFile(filepath.Join(a.locales.Dir, tag+".json"))
	if err != nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "translation not found", "check the language tag")
		return
	}
	// Serve only well-formed catalogs, so a broken file on the volume fails
	// loudly here instead of somewhere inside the browser.
	if !json.Valid(raw) {
		writeError(w, http.StatusInternalServerError, "INVALID_LOCALE", "translation file is not valid JSON", "fix the catalog on the locale volume")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	// Catalogs are mounted from disk and may change without a redeploy, so they
	// must not be cached hard by the browser.
	w.Header().Set("Cache-Control", "no-cache")
	if _, err := w.Write(raw); err != nil {
		return
	}
}

func (a *API) availableLocales() []string {
	if a.locales.Dir == "" {
		return []string{}
	}
	entries, err := os.ReadDir(a.locales.Dir)
	if err != nil {
		return []string{}
	}
	tags := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".json") {
			continue
		}
		tag := strings.TrimSuffix(name, ".json")
		if localeTagPattern.MatchString(tag) {
			tags = append(tags, tag)
		}
	}
	sort.Strings(tags)
	return tags
}
