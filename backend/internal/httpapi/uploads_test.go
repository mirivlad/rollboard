package httpapi

import (
	"bytes"
	"context"
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

// fakeUploadRecords is the accounting, in memory: the real one is a table.
type fakeUploadRecords struct {
	claims map[string]map[string]int64 // name -> owner -> size
}

func newFakeUploadRecords() *fakeUploadRecords {
	return &fakeUploadRecords{claims: map[string]map[string]int64{}}
}

func (f *fakeUploadRecords) Usage(_ context.Context, ownerUserID string) (int64, int64, int64, error) {
	var ownerBytes, ownerFiles, totalBytes int64
	for _, owners := range f.claims {
		counted := false
		for owner, size := range owners {
			if owner == ownerUserID {
				ownerBytes += size
				ownerFiles++
			}
			if !counted {
				totalBytes += size
				counted = true
			}
		}
	}
	return ownerBytes, ownerFiles, totalBytes, nil
}

func (f *fakeUploadRecords) Record(_ context.Context, name, ownerUserID string, size int64, _ string) error {
	if f.claims[name] == nil {
		f.claims[name] = map[string]int64{}
	}
	f.claims[name][ownerUserID] = size
	return nil
}

func (f *fakeUploadRecords) Owns(_ context.Context, name, ownerUserID string) (bool, error) {
	_, ok := f.claims[name][ownerUserID]
	return ok, nil
}

func (f *fakeUploadRecords) Release(_ context.Context, name, ownerUserID string) (bool, error) {
	if _, ok := f.claims[name][ownerUserID]; !ok {
		return false, nil
	}
	delete(f.claims[name], ownerUserID)
	if len(f.claims[name]) == 0 {
		delete(f.claims, name)
		return true, nil
	}
	return false, nil
}

func (f *fakeUploadRecords) KnownNames(context.Context) (map[string]bool, error) {
	names := map[string]bool{}
	for name := range f.claims {
		names[name] = true
	}
	return names, nil
}

func uploadAPI(t *testing.T) (*API, string) {
	api, dir, _ := uploadAPIWithLimits(t, UploadOptions{})
	return api, dir
}

func uploadAPIWithLimits(t *testing.T, options UploadOptions) (*API, string, *fakeUploadRecords) {
	t.Helper()
	dir := t.TempDir()
	records := newFakeUploadRecords()
	user := identity.User{ID: "11111111-1111-1111-1111-111111111111", Email: "author@example.com", DisplayName: "Author"}
	options.Dir = dir
	options.Records = records
	api := New(&spyStore{}).
		WithIdentity(fakeIdentity{actor: &identity.Actor{User: &user}}).
		WithUploads(options)
	return api, dir, records
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

func TestAnAccountCannotFillTheDisk(t *testing.T) {
	// Content addressing stops the same picture being stored twice and does
	// nothing about a thousand different ones, so a quota is the only thing
	// between one signed-in account and the whole disk.
	api, dir, _ := uploadAPIWithLimits(t, UploadOptions{PerAccountBytes: int64(len(tinyPNG)) + 1})

	first := httptest.NewRecorder()
	api.handleUploads(first, uploadRequest(t, "a.png", tinyPNG))
	if first.Code != http.StatusCreated {
		t.Fatalf("first upload = %d, want 201", first.Code)
	}

	second := httptest.NewRecorder()
	api.handleUploads(second, uploadRequest(t, "b.png", append(append([]byte{}, tinyPNG...), 'x')))
	if second.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("second upload = %d, want 413", second.Code)
	}
	// The answer has to name the limit; a bare status tells an author nothing
	// they can act on.
	if !strings.Contains(second.Body.String(), "UPLOAD_QUOTA_EXCEEDED") {
		t.Fatalf("body = %s", second.Body.String())
	}

	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Fatalf("stored %d files, want the quota to have stopped at 1", len(entries))
	}
}

func TestTheDeploymentHasACeilingOfItsOwn(t *testing.T) {
	api, _, _ := uploadAPIWithLimits(t, UploadOptions{TotalBytes: 1})
	response := httptest.NewRecorder()
	api.handleUploads(response, uploadRequest(t, "a.png", tinyPNG))
	if response.Code != http.StatusInsufficientStorage {
		t.Fatalf("status = %d, want 507", response.Code)
	}
	if !strings.Contains(response.Body.String(), "UPLOAD_STORAGE_FULL") {
		t.Fatalf("body = %s", response.Body.String())
	}
}

func TestRepeatingAnUploadCostsTheAccountNothing(t *testing.T) {
	api, _, _ := uploadAPIWithLimits(t, UploadOptions{PerAccountBytes: int64(len(tinyPNG))})
	for i := 0; i < 3; i++ {
		response := httptest.NewRecorder()
		api.handleUploads(response, uploadRequest(t, "same.png", tinyPNG))
		if response.Code != http.StatusCreated {
			t.Fatalf("upload %d = %d, want 201", i, response.Code)
		}
	}
}

func TestUploadsAreRateLimitedPerAccount(t *testing.T) {
	api, _, _ := uploadAPIWithLimits(t, UploadOptions{RatePerMinute: 2})
	codes := []int{}
	for i := 0; i < 4; i++ {
		response := httptest.NewRecorder()
		// Distinct bytes each time, so nothing is deduplicated away.
		payload := append(append([]byte{}, tinyPNG...), byte(i))
		api.handleUploads(response, uploadRequest(t, "x.png", payload))
		codes = append(codes, response.Code)
	}
	if codes[0] != http.StatusCreated || codes[1] != http.StatusCreated {
		t.Fatalf("codes = %v, want the first two to succeed", codes)
	}
	if codes[2] != http.StatusTooManyRequests || codes[3] != http.StatusTooManyRequests {
		t.Fatalf("codes = %v, want the rest refused", codes)
	}
}

func TestAnAuthorCanGiveBackTheirImages(t *testing.T) {
	api, dir, _ := uploadAPIWithLimits(t, UploadOptions{})
	created := httptest.NewRecorder()
	api.handleUploads(created, uploadRequest(t, "a.png", tinyPNG))
	var body struct{ URL string }
	if err := json.Unmarshal(created.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}

	request := httptest.NewRequest(http.MethodDelete, body.URL, nil)
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session-token"})
	request.AddCookie(&http.Cookie{Name: csrfCookieName, Value: "csrf-token"})
	request.Header.Set(csrfHeaderName, "csrf-token")
	response := httptest.NewRecorder()
	api.handleUploadByName(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("delete = %d, want 204", response.Code)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		t.Fatalf("%d files left on disk after deleting the only claim", len(entries))
	}
}

func TestFilesNothingClaimsAreSweptAway(t *testing.T) {
	api, dir, _ := uploadAPIWithLimits(t, UploadOptions{})
	// What a crash between writing the file and recording it leaves behind.
	orphan := filepath.Join(dir, "0123456789abcdef0123456789abcdef.png")
	if err := os.WriteFile(orphan, tinyPNG, 0o644); err != nil {
		t.Fatal(err)
	}
	kept := httptest.NewRecorder()
	api.handleUploads(kept, uploadRequest(t, "a.png", tinyPNG))

	removed, err := api.SweepOrphanedUploads(context.Background())
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if removed != 1 {
		t.Fatalf("removed %d, want 1", removed)
	}
	if _, err := os.Stat(orphan); !os.IsNotExist(err) {
		t.Fatal("the orphan survived the sweep")
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Fatalf("the sweep took a claimed file too: %d left", len(entries))
	}
}
