package httpapi

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// maxUploadBytes caps a single image. Board art does not need to be larger,
// and an unbounded upload endpoint is a way to fill somebody's disk.
const maxUploadBytes = 2 << 20 // 2 MiB

// allowedImageTypes maps a sniffed content type to the extension it is stored
// under. SVG is deliberately absent: it can carry script, and these files are
// served from the application's own origin.
var allowedImageTypes = map[string]string{
	"image/png":  ".png",
	"image/jpeg": ".jpg",
	"image/gif":  ".gif",
	"image/webp": ".webp",
}

// uploadNamePattern matches the names this server generates, and nothing else.
// The name arrives from a URL and becomes a file path, so it is matched rather
// than sanitised.
var uploadNamePattern = regexp.MustCompile(`^[0-9a-f]{32}\.(png|jpg|gif|webp)$`)

type UploadOptions struct {
	// Dir stores uploaded images. Empty disables uploading entirely, which is
	// the default: a deployment opts in by configuring somewhere to write.
	Dir string
}

func (a *API) WithUploads(options UploadOptions) *API {
	a.uploads = options
	return a
}

func (a *API) handleUploads(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed", "use POST")
		return
	}
	if a.uploads.Dir == "" {
		writeError(w, http.StatusNotImplemented, "NOT_IMPLEMENTED", "uploads are not enabled", "set ROLLBOARD_UPLOADS_DIR")
		return
	}
	// Authors only: an open upload endpoint is a free file host.
	if _, ok := a.requireAccount(w, r); !ok {
		return
	}
	if !requireCSRF(w, r) {
		return
	}

	// Bound the read itself, not just the declared length, so a lying
	// Content-Length cannot make the server read forever.
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes+1024)
	if err := r.ParseMultipartForm(maxUploadBytes); err != nil {
		writeError(w, http.StatusRequestEntityTooLarge, "UPLOAD_TOO_LARGE", "image is too large", "images must be at most 2 MB")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_UPLOAD", "no file was sent", "attach a file field named 'file'")
		return
	}
	defer file.Close()
	if header.Size > maxUploadBytes {
		writeError(w, http.StatusRequestEntityTooLarge, "UPLOAD_TOO_LARGE", "image is too large", "images must be at most 2 MB")
		return
	}

	data, err := io.ReadAll(io.LimitReader(file, maxUploadBytes+1))
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_UPLOAD", "could not read the file", "try again")
		return
	}
	if len(data) > maxUploadBytes {
		writeError(w, http.StatusRequestEntityTooLarge, "UPLOAD_TOO_LARGE", "image is too large", "images must be at most 2 MB")
		return
	}

	// The type is sniffed from the bytes, never taken from the filename or the
	// client's Content-Type, either of which the uploader controls.
	contentType := http.DetectContentType(data)
	extension, allowed := allowedImageTypes[contentType]
	if !allowed {
		writeError(w, http.StatusUnsupportedMediaType, "UNSUPPORTED_IMAGE", "that file is not a supported image",
			"use PNG, JPEG, GIF or WebP")
		return
	}

	// Content-addressed, so uploading the same picture twice costs one file.
	digest := sha256.Sum256(data)
	name := hex.EncodeToString(digest[:16]) + extension
	if err := os.MkdirAll(a.uploads.Dir, 0o755); err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "could not store the image", "try again later")
		return
	}
	path := filepath.Join(a.uploads.Dir, name)
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		if err := os.WriteFile(path, data, 0o644); err != nil {
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "could not store the image", "try again later")
			return
		}
	}

	writeJSON(w, http.StatusCreated, map[string]string{
		"url":         "/api/uploads/" + name,
		"contentType": contentType,
	})
}

func (a *API) handleUploadByName(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed", "use GET")
		return
	}
	if a.uploads.Dir == "" {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "not found", "uploads are not enabled")
		return
	}
	name := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/uploads/"), "/")
	if name == "" {
		a.handleUploads(w, r)
		return
	}
	if !uploadNamePattern.MatchString(name) {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "image not found", "check the image URL")
		return
	}

	data, err := os.ReadFile(filepath.Join(a.uploads.Dir, name))
	if err != nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "image not found", "check the image URL")
		return
	}

	contentType := http.DetectContentType(data)
	if _, allowed := allowedImageTypes[contentType]; !allowed {
		// Something on disk is no longer what it claimed to be; refuse rather
		// than serve an unexpected type from our own origin.
		writeError(w, http.StatusNotFound, "NOT_FOUND", "image not found", "check the image URL")
		return
	}
	w.Header().Set("Content-Type", contentType)
	// The name is the content hash, so a stored image never changes.
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; sandbox")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if _, err := w.Write(data); err != nil {
		return
	}
}
