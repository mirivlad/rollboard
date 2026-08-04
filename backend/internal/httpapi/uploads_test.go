package httpapi

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"rollboard/internal/identity"
)

// A one-pixel PNG, so the tests exercise real sniffing rather than a stub.
var tinyPNG = []byte{
	0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A,
	0x00, 0x00, 0x00, 0x0D, 'I', 'H', 'D', 'R',
	0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
	0x08, 0x06, 0x00, 0x00, 0x00, 0x1F, 0x15, 0xC4,
	0x89, 0x00, 0x00, 0x00, 0x0A, 'I', 'D', 'A', 'T',
	0x78, 0x9C, 0x63, 0x00, 0x01, 0x00, 0x00, 0x05,
	0x00, 0x01, 0x0D, 0x0A, 0x2D, 0xB4, 0x00, 0x00,
	0x00, 0x00, 'I', 'E', 'N', 'D', 0xAE, 0x42, 0x60, 0x82,
}

func uploadRequest(t *testing.T, filename string, contents []byte) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(contents); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/uploads", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session-token"})
	request.AddCookie(&http.Cookie{Name: csrfCookieName, Value: "csrf-token"})
	request.Header.Set(csrfHeaderName, "csrf-token")
	return request
}

func uploadAPI(t *testing.T) (*API, string) {
	t.Helper()
	dir := t.TempDir()
	user := identity.User{ID: "11111111-1111-1111-1111-111111111111", Email: "author@example.com", DisplayName: "Author"}
	api := New(&spyStore{}).
		WithIdentity(fakeIdentity{actor: &identity.Actor{User: &user}}).
		WithUploads(UploadOptions{Dir: dir})
	return api, dir
}

func TestUploadingAnImageStoresItAndServesItBack(t *testing.T) {
	api, dir := uploadAPI(t)
	mux := newAuthzMux(api)

	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, uploadRequest(t, "art.png", tinyPNG))
	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body=%s", recorder.Code, recorder.Body.String())
	}
	var created struct{ URL, ContentType string }
	if err := json.Unmarshal(recorder.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.ContentType != "image/png" {
		t.Fatalf("contentType = %q, want image/png", created.ContentType)
	}

	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Fatalf("stored %d files, want 1", len(entries))
	}

	fetch := httptest.NewRecorder()
	mux.ServeHTTP(fetch, httptest.NewRequest(http.MethodGet, created.URL, nil))
	if fetch.Code != http.StatusOK {
		t.Fatalf("fetch status = %d, want 200", fetch.Code)
	}
	if !bytes.Equal(fetch.Body.Bytes(), tinyPNG) {
		t.Fatal("the served bytes differ from what was uploaded")
	}
	// Served from the application's own origin, so it must not be able to run.
	if fetch.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("missing nosniff on a user-supplied file")
	}
	if !strings.Contains(fetch.Header().Get("Content-Security-Policy"), "sandbox") {
		t.Fatal("missing sandbox policy on a user-supplied file")
	}
}

// The same picture twice must cost one file, since the name is its content hash.
func TestUploadingTheSameImageTwiceStoresItOnce(t *testing.T) {
	api, dir := uploadAPI(t)
	mux := newAuthzMux(api)

	var urls []string
	for i := 0; i < 2; i++ {
		recorder := httptest.NewRecorder()
		mux.ServeHTTP(recorder, uploadRequest(t, "art.png", tinyPNG))
		var created struct{ URL string }
		_ = json.Unmarshal(recorder.Body.Bytes(), &created)
		urls = append(urls, created.URL)
	}
	if urls[0] != urls[1] {
		t.Fatalf("urls = %v, want the same content-addressed name", urls)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Fatalf("stored %d files, want 1", len(entries))
	}
}

// The type comes from the bytes, never from the name or the declared type,
// both of which the uploader controls.
func TestNonImagesAreRejectedEvenWithAnImageName(t *testing.T) {
	api, dir := uploadAPI(t)
	mux := newAuthzMux(api)

	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, uploadRequest(t, "totally-an-image.png", []byte("<?php echo 'hello'; ?>")))

	if recorder.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("status = %d, want 415; body=%s", recorder.Code, recorder.Body.String())
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		t.Fatalf("stored %d files, want the upload refused", len(entries))
	}
}

func TestSvgIsRejectedBecauseItCanCarryScript(t *testing.T) {
	api, _ := uploadAPI(t)
	mux := newAuthzMux(api)
	svg := []byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`)

	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, uploadRequest(t, "art.svg", svg))

	if recorder.Code == http.StatusCreated {
		t.Fatal("an SVG was accepted")
	}
}

func TestUploadingRequiresAnAccountAndCSRF(t *testing.T) {
	_, dir := uploadAPI(t)
	anonymous := New(&spyStore{}).WithIdentity(fakeIdentity{}).WithUploads(UploadOptions{Dir: dir})
	recorder := httptest.NewRecorder()
	newAuthzMux(anonymous).ServeHTTP(recorder, uploadRequest(t, "art.png", tinyPNG))
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous upload status = %d, want 401", recorder.Code)
	}

	api, _ := uploadAPI(t)
	noCSRF := uploadRequest(t, "art.png", tinyPNG)
	noCSRF.Header.Del(csrfHeaderName)
	forbidden := httptest.NewRecorder()
	newAuthzMux(api).ServeHTTP(forbidden, noCSRF)
	if forbidden.Code != http.StatusForbidden {
		t.Fatalf("upload without CSRF status = %d, want 403", forbidden.Code)
	}
}

// The name becomes a filesystem path, so it is matched against the pattern the
// server itself generates rather than sanitised.
func TestUploadNamesCannotEscapeTheUploadDirectory(t *testing.T) {
	api, dir := uploadAPI(t)
	secret := filepath.Join(filepath.Dir(dir), "secret.png")
	if err := os.WriteFile(secret, append(tinyPNG, []byte("leaked")...), 0o600); err != nil {
		t.Fatal(err)
	}
	mux := newAuthzMux(api)

	for _, name := range []string{"../secret.png", "..%2Fsecret.png", "/etc/passwd", "nope.png", "abc.png"} {
		recorder := httptest.NewRecorder()
		mux.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/uploads/"+name, nil))
		if recorder.Code == http.StatusOK {
			t.Fatalf("name %q was served", name)
		}
		if strings.Contains(recorder.Body.String(), "leaked") {
			t.Fatalf("name %q escaped the upload directory", name)
		}
	}
}

func TestUploadsAreOffUntilADirectoryIsConfigured(t *testing.T) {
	user := identity.User{ID: "11111111-1111-1111-1111-111111111111", Email: "a@example.com", DisplayName: "A"}
	api := New(&spyStore{}).WithIdentity(fakeIdentity{actor: &identity.Actor{User: &user}})

	recorder := httptest.NewRecorder()
	newAuthzMux(api).ServeHTTP(recorder, uploadRequest(t, "art.png", tinyPNG))
	if recorder.Code != http.StatusNotImplemented {
		t.Fatalf("status = %d, want 501 when uploads are not configured", recorder.Code)
	}
}
