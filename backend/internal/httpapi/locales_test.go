package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newLocaleAPI(t *testing.T, files map[string]string) (*API, string) {
	t.Helper()
	dir := t.TempDir()
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatalf("write locale %s: %v", name, err)
		}
	}
	return New(&spyStore{}).WithLocales(LocaleOptions{Dir: dir}), dir
}

func TestLocaleListingReportsAvailableCatalogs(t *testing.T) {
	api, _ := newLocaleAPI(t, map[string]string{
		"en.json":       `{"app.title":"Rollboard"}`,
		"ru.json":       `{"app.title":"Rollboard"}`,
		"pt-BR.json":    `{"app.title":"Rollboard"}`,
		"notes.txt":     "ignored",
		"README.md":     "ignored",
		"bad name.json": "ignored",
	})
	recorder := httptest.NewRecorder()
	newAuthzMux(api).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/locales", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
	}
	var body struct {
		Locales []string `json:"locales"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	want := []string{"en", "pt-BR", "ru"}
	if len(body.Locales) != len(want) {
		t.Fatalf("locales = %v, want %v", body.Locales, want)
	}
	for i, tag := range want {
		if body.Locales[i] != tag {
			t.Fatalf("locales = %v, want %v", body.Locales, want)
		}
	}
}

func TestLocaleCatalogIsServed(t *testing.T) {
	api, _ := newLocaleAPI(t, map[string]string{"ru.json": `{"app.title":"Роллборд"}`})
	recorder := httptest.NewRecorder()
	newAuthzMux(api).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/locales/ru", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
	}
	var catalog map[string]string
	if err := json.Unmarshal(recorder.Body.Bytes(), &catalog); err != nil {
		t.Fatal(err)
	}
	if catalog["app.title"] != "Роллборд" {
		t.Fatalf("catalog = %v, want the Russian title", catalog)
	}
	if recorder.Header().Get("Cache-Control") != "no-cache" {
		t.Fatalf("Cache-Control = %q; mounted catalogs must not be cached hard", recorder.Header().Get("Cache-Control"))
	}
}

// TestLocaleTagCannotEscapeTheLocaleDirectory matters because the tag is used
// to build a filesystem path.
func TestLocaleTagCannotEscapeTheLocaleDirectory(t *testing.T) {
	api, dir := newLocaleAPI(t, map[string]string{"en.json": `{"app.title":"Rollboard"}`})
	secret := filepath.Join(filepath.Dir(dir), "secret.json")
	if err := os.WriteFile(secret, []byte(`{"leaked":"yes"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	for _, tag := range []string{
		"../secret",
		"..%2Fsecret",
		"....//secret",
		"/etc/passwd",
		"en/../../secret",
		"en.json.bak",
		"..",
	} {
		t.Run(tag, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, "/api/locales/"+tag, nil)
			newAuthzMux(api).ServeHTTP(recorder, request)

			if recorder.Code == http.StatusOK {
				t.Fatalf("tag %q was served: %s", tag, recorder.Body.String())
			}
			if body := recorder.Body.String(); strings.Contains(body, "leaked") {
				t.Fatalf("tag %q leaked a file outside the locale directory", tag)
			}
		})
	}
}

func TestUnknownLocaleIsNotFound(t *testing.T) {
	api, _ := newLocaleAPI(t, map[string]string{"en.json": `{"app.title":"Rollboard"}`})
	recorder := httptest.NewRecorder()
	newAuthzMux(api).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/locales/de", nil))

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestMalformedCatalogIsRejectedByTheServer(t *testing.T) {
	api, _ := newLocaleAPI(t, map[string]string{"en.json": `{"app.title": broken`})
	recorder := httptest.NewRecorder()
	newAuthzMux(api).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/locales/en", nil))

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500; body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestLocalesWithoutConfiguredDirectoryAreEmpty(t *testing.T) {
	api := New(&spyStore{})
	recorder := httptest.NewRecorder()
	newAuthzMux(api).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/locales", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
	}
	if body := recorder.Body.String(); !strings.Contains(body, `"locales":[]`) {
		t.Fatalf("body = %s, want an empty locale list", body)
	}
}
