package httpapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
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

// UploadRecords is the accounting an upload quota needs: who holds what.
//
// Content addressing stops the same picture being stored twice and does
// nothing about a thousand different ones, so without this any signed-in
// account could fill the disk one unique image at a time.
type UploadRecords interface {
	Usage(ctx context.Context, ownerUserID string) (ownerBytes, ownerFiles, totalBytes int64, err error)
	Record(ctx context.Context, name, ownerUserID string, size int64, contentType string) error
	Owns(ctx context.Context, name, ownerUserID string) (bool, error)
	Release(ctx context.Context, name, ownerUserID string) (bool, error)
	KnownNames(ctx context.Context) (map[string]bool, error)
}

type UploadOptions struct {
	// Dir stores uploaded images. Empty disables uploading entirely, which is
	// the default: a deployment opts in by configuring somewhere to write.
	Dir string
	// Records accounts for what is stored. Without it uploads are refused
	// rather than accepted unaccounted: an unbounded endpoint is worse than a
	// disabled one.
	Records UploadRecords
	// PerAccountBytes and TotalBytes bound one author and the deployment.
	PerAccountBytes int64
	TotalBytes      int64
	// RatePerMinute bounds how fast one account can upload, so a quota cannot
	// be filled faster than a person could notice.
	RatePerMinute int
}

func (a *API) WithUploads(options UploadOptions) *API {
	a.uploads = options
	if options.RatePerMinute > 0 {
		a.uploadLimiter = newRateLimiter(options.RatePerMinute, rateLimitWindow)
	}
	return a
}

// SweepOrphanedUploads deletes files on disk that no account claims.
//
// A crash between writing the file and recording it leaves exactly that, and
// nothing else would ever remove it. Images an author no longer uses in any
// game are not touched: only they know whether a picture is finished with, and
// the delete endpoint is how they say so.
func (a *API) SweepOrphanedUploads(ctx context.Context) (int, error) {
	if a.uploads.Dir == "" || a.uploads.Records == nil {
		return 0, nil
	}
	known, err := a.uploads.Records.KnownNames(ctx)
	if err != nil {
		return 0, err
	}
	entries, err := os.ReadDir(a.uploads.Dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return 0, nil
		}
		return 0, err
	}
	removed := 0
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || known[name] || !uploadNamePattern.MatchString(name) {
			continue
		}
		if err := os.Remove(filepath.Join(a.uploads.Dir, name)); err == nil {
			removed++
		}
	}
	return removed, nil
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
	user, ok := a.requireAccount(w, r)
	if !ok {
		return
	}
	if !requireCSRF(w, r) {
		return
	}
	if a.uploads.Records == nil {
		// Accepting uploads nothing accounts for is how a disk fills up with
		// no way to find out what filled it.
		writeError(w, http.StatusNotImplemented, "NOT_IMPLEMENTED", "uploads are not enabled",
			"upload accounting is not configured")
		return
	}
	if a.uploadLimiter != nil {
		if allowed, retryAfter := a.uploadLimiter.allow(user.ID); !allowed {
			seconds := int(retryAfter.Seconds())
			if seconds < 1 {
				seconds = 1
			}
			w.Header().Set("Retry-After", fmt.Sprintf("%d", seconds))
			writeError(w, http.StatusTooManyRequests, "RATE_LIMITED", "too many uploads", "wait a moment and try again")
			return
		}
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
	size := int64(len(data))

	// Quotas are checked against what this account already holds, and re-uploading
	// something it already has costs nothing.
	alreadyMine, err := a.uploads.Records.Owns(r.Context(), name, user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "could not store the image", "try again later")
		return
	}
	if !alreadyMine {
		ownerBytes, _, totalBytes, err := a.uploads.Records.Usage(r.Context(), user.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "could not store the image", "try again later")
			return
		}
		if a.uploads.PerAccountBytes > 0 && ownerBytes+size > a.uploads.PerAccountBytes {
			// A quota answer says what the limit is, because "413" alone gives
			// an author nothing to act on.
			writeError(w, http.StatusRequestEntityTooLarge, "UPLOAD_QUOTA_EXCEEDED",
				"you have used all your image storage",
				fmt.Sprintf("your images may total %d MB; delete some to make room", a.uploads.PerAccountBytes>>20))
			return
		}
		if a.uploads.TotalBytes > 0 && totalBytes+size > a.uploads.TotalBytes {
			writeError(w, http.StatusInsufficientStorage, "UPLOAD_STORAGE_FULL",
				"this server has no room for more images",
				"ask the administrator to raise ROLLBOARD_UPLOAD_TOTAL_MB or remove old images")
			return
		}
	}

	if err := os.MkdirAll(a.uploads.Dir, 0o755); err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "could not store the image", "try again later")
		return
	}
	path := filepath.Join(a.uploads.Dir, name)
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		if err := os.WriteFile(path, data, 0o644); err != nil {
			// A full or unwritable disk is the deployment's problem, not the
			// author's mistake, and it says so rather than "try again later".
			log.Printf("upload write failed: %v", err)
			writeError(w, http.StatusInsufficientStorage, "UPLOAD_STORAGE_FULL",
				"the server could not write the image", "check the server's upload directory and free space")
			return
		}
	}
	if err := a.uploads.Records.Record(r.Context(), name, user.ID, size, contentType); err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "could not store the image", "try again later")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{
		"url":         "/api/uploads/" + name,
		"contentType": contentType,
	})
}

func (a *API) handleUploadByName(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodDelete {
		a.handleUploadDelete(w, r)
		return
	}
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed", "use GET or DELETE")
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

// handleUploadDelete lets an author give back the room an image takes.
//
// Deleting is per account: the file goes only when the last account that
// uploaded it releases it, because the name is a content hash and two authors
// can legitimately have uploaded the same picture.
func (a *API) handleUploadDelete(w http.ResponseWriter, r *http.Request) {
	if a.uploads.Dir == "" || a.uploads.Records == nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "not found", "uploads are not enabled")
		return
	}
	user, ok := a.requireAccount(w, r)
	if !ok {
		return
	}
	if !requireCSRF(w, r) {
		return
	}
	name := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/uploads/"), "/")
	if !uploadNamePattern.MatchString(name) {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "image not found", "check the image URL")
		return
	}

	lastClaim, err := a.uploads.Records.Release(r.Context(), name, user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "could not delete the image", "try again later")
		return
	}
	if !lastClaim {
		// Either somebody else still holds it, or this account never did. Both
		// answer the same way: an account cannot learn what it does not own.
		if owns, _ := a.uploads.Records.Owns(r.Context(), name, user.ID); !owns {
			w.WriteHeader(http.StatusNoContent)
			return
		}
	}
	if lastClaim {
		if err := os.Remove(filepath.Join(a.uploads.Dir, name)); err != nil && !errors.Is(err, os.ErrNotExist) {
			log.Printf("upload delete failed: %v", err)
		}
	}
	w.WriteHeader(http.StatusNoContent)
}
