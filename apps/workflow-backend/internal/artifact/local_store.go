package artifact

import (
	"compress/gzip"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type LocalStore struct {
	rootDir string
}

func NewLocalStore(rootDir string) *LocalStore {
	return &LocalStore{rootDir: rootDir}
}

func (store *LocalStore) RootDir() string {
	return store.rootDir
}

func (store *LocalStore) Put(_ context.Context, request PutRequest) (Artifact, error) {
	if err := validatePutRequest(request); err != nil {
		return Artifact{}, err
	}

	id := newArtifactID()
	relativePath := filepath.Join(
		"artifacts",
		"projects",
		safePathSegment(request.ProjectID),
		string(request.OwnerType),
		safePathSegment(request.OwnerID),
		id+"-"+string(request.Kind)+".gz",
	)
	fullPath, err := store.resolvePath(relativePath)
	if err != nil {
		return Artifact{}, err
	}
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		return Artifact{}, err
	}

	file, err := os.Create(fullPath)
	if err != nil {
		return Artifact{}, err
	}
	defer file.Close()

	hasher := sha256.New()
	counter := &countingWriter{}
	gzipWriter := gzip.NewWriter(io.MultiWriter(file, hasher, counter))
	if _, err := io.Copy(gzipWriter, request.Reader); err != nil {
		gzipWriter.Close()
		return Artifact{}, err
	}
	if err := gzipWriter.Close(); err != nil {
		return Artifact{}, err
	}

	return Artifact{
		ID:             id,
		ProjectID:      request.ProjectID,
		OwnerType:      request.OwnerType,
		OwnerID:        request.OwnerID,
		Kind:           request.Kind,
		ContentType:    request.ContentType,
		SizeBytes:      counter.n,
		SHA256:         hex.EncodeToString(hasher.Sum(nil)),
		StorageBackend: "local",
		RelativePath:   relativePath,
		CreatedAt:      time.Now().UTC(),
	}, nil
}

func (store *LocalStore) Get(_ context.Context, artifact Artifact) (io.ReadCloser, error) {
	fullPath, err := store.resolvePath(artifact.RelativePath)
	if err != nil {
		return nil, err
	}
	return os.Open(fullPath)
}

func (store *LocalStore) Delete(_ context.Context, artifact Artifact) error {
	fullPath, err := store.resolvePath(artifact.RelativePath)
	if err != nil {
		return err
	}
	err = os.Remove(fullPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func (store *LocalStore) resolvePath(relativePath string) (string, error) {
	if relativePath == "" || !filepath.IsLocal(relativePath) {
		return "", fmt.Errorf("artifact path must be local: %q", relativePath)
	}
	root, err := filepath.Abs(store.rootDir)
	if err != nil {
		return "", err
	}
	fullPath := filepath.Join(root, relativePath)
	if !strings.HasPrefix(fullPath, root+string(filepath.Separator)) && fullPath != root {
		return "", fmt.Errorf("artifact path escapes data dir: %q", relativePath)
	}
	return fullPath, nil
}

type countingWriter struct {
	n int64
}

func (writer *countingWriter) Write(p []byte) (int, error) {
	writer.n += int64(len(p))
	return len(p), nil
}

func validatePutRequest(request PutRequest) error {
	if request.ProjectID == "" {
		return errors.New("project id is required")
	}
	if request.OwnerType == "" {
		return errors.New("owner type is required")
	}
	if request.OwnerID == "" {
		return errors.New("owner id is required")
	}
	if request.Reader == nil {
		return errors.New("reader is required")
	}
	if !isAllowedKind(request.Kind) {
		return fmt.Errorf("artifact kind %q is not allowed", request.Kind)
	}
	if isImageOrVideo(request.ContentType) {
		return fmt.Errorf("artifact content type %q is not allowed", request.ContentType)
	}
	return nil
}

func isAllowedKind(kind ArtifactKind) bool {
	switch kind {
	case KindSession, KindBrowserState, KindDOMSnapshot, KindWorkflowUseSource, KindDebugBundle:
		return true
	default:
		return false
	}
}

func isImageOrVideo(contentType string) bool {
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		mediaType = strings.ToLower(strings.TrimSpace(contentType))
	}
	return strings.HasPrefix(mediaType, "image/") || strings.HasPrefix(mediaType, "video/")
}

func safePathSegment(segment string) string {
	replacer := strings.NewReplacer("/", "_", "\\", "_", "..", "_", " ", "_")
	return replacer.Replace(segment)
}

func newArtifactID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("art_%d", time.Now().UnixNano())
	}
	return "art_" + hex.EncodeToString(b[:])
}
