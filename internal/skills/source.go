package skills

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

// Source fetches registry files (the index and each skill's files) by a
// registry-relative path. Implementations back it with a URL, a local dir, or an
// in-memory map (tests).
type Source interface {
	Fetch(relpath string) ([]byte, error)
}

// indexPath is the registry-relative path of the catalog.
const indexPath = "registry.json"

// LoadIndex fetches and parses the registry index from src.
func LoadIndex(src Source) (*Index, error) {
	data, err := src.Fetch(indexPath)
	if err != nil {
		return nil, fmt.Errorf("skills: fetch index: %w", err)
	}
	var idx Index
	if err := json.Unmarshal(data, &idx); err != nil {
		return nil, fmt.Errorf("skills: parse index: %w", err)
	}
	return &idx, nil
}

// HTTPSource fetches from a base URL (e.g. a raw git host).
type HTTPSource struct {
	Base   string
	Client *http.Client
}

// NewHTTPSource returns an HTTPSource for base with a sane default client.
func NewHTTPSource(base string) *HTTPSource {
	return &HTTPSource{
		Base:   strings.TrimRight(base, "/") + "/",
		Client: &http.Client{Timeout: 30 * time.Second},
	}
}

// Fetch GETs base+relpath and returns the body.
func (s *HTTPSource) Fetch(relpath string) ([]byte, error) {
	if err := safeRel(relpath); err != nil {
		return nil, err
	}
	resp, err := s.Client.Get(s.Base + relpath)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("skills: %s: HTTP %d", relpath, resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

// DirSource fetches from a local directory (a cloned registry, or tests).
type DirSource struct{ Root string }

// Fetch reads Root/relpath, guarding against path escape.
func (s DirSource) Fetch(relpath string) ([]byte, error) {
	if err := safeRel(relpath); err != nil {
		return nil, err
	}
	return os.ReadFile(filepath.Join(s.Root, filepath.FromSlash(relpath)))
}

// safeRel rejects absolute or parent-escaping registry paths so a malicious
// index can't read or write outside the registry/prefix.
func safeRel(relpath string) error {
	if relpath == "" || path.IsAbs(relpath) || strings.Contains(relpath, "..") {
		return fmt.Errorf("skills: unsafe path %q", relpath)
	}
	return nil
}
