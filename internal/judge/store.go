package judge

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type FileStore struct {
	root string
}

func NewFileStore(root string) *FileStore {
	return &FileStore{root: root}
}

func (s *FileStore) Ensure() error {
	return os.MkdirAll(s.root, 0o750)
}

func (s *FileStore) SaveSubmission(id string, reader io.Reader) error {
	path, err := s.path("submissions", id, "code")
	if err != nil {
		return err
	}
	return writeFile(path, reader)
}

func (s *FileStore) OpenSubmission(id string) (io.ReadCloser, error) {
	path, err := s.path("submissions", id, "code")
	if err != nil {
		return nil, err
	}
	return os.Open(path)
}

func (s *FileStore) SaveProblemFile(domainID, pid, name string, reader io.Reader) error {
	path, err := s.path("problems", domainID, pid, name)
	if err != nil {
		return err
	}
	return writeFile(path, reader)
}

func (s *FileStore) OpenProblemFile(domainID, pid, name string) (io.ReadCloser, error) {
	path, err := s.path("problems", domainID, pid, name)
	if err != nil {
		return nil, err
	}
	return os.Open(path)
}

func (s *FileStore) SaveUpload(rid, name string, reader io.Reader) error {
	path, err := s.path("uploads", rid, name)
	if err != nil {
		return err
	}
	return writeFile(path, reader)
}

func (s *FileStore) path(parts ...string) (string, error) {
	root, err := filepath.Abs(s.root)
	if err != nil {
		return "", err
	}
	all := append([]string{root}, parts...)
	joined := filepath.Join(all...)
	rel, err := filepath.Rel(root, joined)
	if err != nil || rel == "." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || rel == ".." || filepath.IsAbs(rel) {
		return "", fmt.Errorf("invalid file path %q", filepath.Join(parts...))
	}
	return joined, nil
}

func writeFile(path string, reader io.Reader) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o640)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = io.Copy(file, reader)
	return err
}
