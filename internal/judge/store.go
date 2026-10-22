package judge

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type FileStore struct {
	root string
}

type StoredFile struct {
	Name         string
	Size         int64
	ETag         string
	LastModified time.Time
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
	_, err := s.PutProblemFile(domainID, pid, name, reader)
	return err
}

func (s *FileStore) PutProblemFile(domainID, pid, name string, reader io.Reader) (StoredFile, error) {
	path, err := s.path("problem", domainID, pid, "testdata", name)
	if err != nil {
		return StoredFile{}, err
	}
	return writeFileWithMetadata(path, name, reader)
}

func (s *FileStore) OpenProblemFile(domainID, pid, name string) (io.ReadCloser, error) {
	path, err := s.path("problem", domainID, pid, "testdata", name)
	if err != nil {
		return nil, err
	}
	return os.Open(path)
}

// DeleteProblemDir removes every stored file of one problem.
func (s *FileStore) DeleteProblemDir(domainID, pid string) error {
	path, err := s.path("problem", domainID, pid)
	if err != nil {
		return err
	}
	return os.RemoveAll(path)
}

func (s *FileStore) DeleteProblemFile(domainID, pid, name string) error {
	path, err := s.path("problem", domainID, pid, "testdata", name)
	if err != nil {
		return err
	}
	return removeFile(path)
}

func (s *FileStore) SaveAdditionalFile(domainID, pid, name string, reader io.Reader) error {
	_, err := s.PutAdditionalFile(domainID, pid, name, reader)
	return err
}

func (s *FileStore) PutAdditionalFile(domainID, pid, name string, reader io.Reader) (StoredFile, error) {
	path, err := s.path("problem", domainID, pid, "additional_file", name)
	if err != nil {
		return StoredFile{}, err
	}
	return writeFileWithMetadata(path, name, reader)
}

func (s *FileStore) OpenAdditionalFile(domainID, pid, name string) (io.ReadCloser, error) {
	path, err := s.path("problem", domainID, pid, "additional_file", name)
	if err != nil {
		return nil, err
	}
	return os.Open(path)
}

func (s *FileStore) DeleteAdditionalFile(domainID, pid, name string) error {
	path, err := s.path("problem", domainID, pid, "additional_file", name)
	if err != nil {
		return err
	}
	return removeFile(path)
}

func (s *FileStore) PutContestFile(domainID, tid, name string, reader io.Reader) (StoredFile, error) {
	return s.putContestFile("public", domainID, tid, name, reader)
}

func (s *FileStore) PutContestPrivateFile(domainID, tid, name string, reader io.Reader) (StoredFile, error) {
	return s.putContestFile("private", domainID, tid, name, reader)
}

func (s *FileStore) putContestFile(fileType, domainID, tid, name string, reader io.Reader) (StoredFile, error) {
	path, err := s.path("contest", domainID, tid, fileType, name)
	if err != nil {
		return StoredFile{}, err
	}
	return writeFileWithMetadata(path, name, reader)
}

func (s *FileStore) OpenContestFile(domainID, tid, name string) (io.ReadCloser, error) {
	return s.openContestFile("public", domainID, tid, name)
}

func (s *FileStore) OpenContestPrivateFile(domainID, tid, name string) (io.ReadCloser, error) {
	return s.openContestFile("private", domainID, tid, name)
}

func (s *FileStore) openContestFile(fileType, domainID, tid, name string) (io.ReadCloser, error) {
	path, err := s.path("contest", domainID, tid, fileType, name)
	if err != nil {
		return nil, err
	}
	return os.Open(path)
}

func (s *FileStore) DeleteContestFile(domainID, tid, name string) error {
	return s.deleteContestFile("public", domainID, tid, name)
}

func (s *FileStore) DeleteContestPrivateFile(domainID, tid, name string) error {
	return s.deleteContestFile("private", domainID, tid, name)
}

func (s *FileStore) deleteContestFile(fileType, domainID, tid, name string) error {
	path, err := s.path("contest", domainID, tid, fileType, name)
	if err != nil {
		return err
	}
	return removeFile(path)
}

func (s *FileStore) PutTrainingFile(domainID, tid, name string, reader io.Reader) (StoredFile, error) {
	path, err := s.path("training", domainID, tid, name)
	if err != nil {
		return StoredFile{}, err
	}
	return writeFileWithMetadata(path, name, reader)
}

func (s *FileStore) OpenTrainingFile(domainID, tid, name string) (io.ReadCloser, error) {
	path, err := s.path("training", domainID, tid, name)
	if err != nil {
		return nil, err
	}
	return os.Open(path)
}

func (s *FileStore) DeleteTrainingFile(domainID, tid, name string) error {
	path, err := s.path("training", domainID, tid, name)
	if err != nil {
		return err
	}
	return removeFile(path)
}

func (s *FileStore) SaveUpload(rid, name string, reader io.Reader) error {
	path, err := s.path("uploads", rid, name)
	if err != nil {
		return err
	}
	return writeFile(path, reader)
}

func (s *FileStore) PutUserFile(uid int64, name string, reader io.Reader) (StoredFile, error) {
	path, err := s.path("user", strconv.FormatInt(uid, 10), name)
	if err != nil {
		return StoredFile{}, err
	}
	return writeFileWithMetadata(path, name, reader)
}

func (s *FileStore) OpenUserFile(uid int64, name string) (io.ReadCloser, error) {
	path, err := s.path("user", strconv.FormatInt(uid, 10), name)
	if err != nil {
		return nil, err
	}
	return os.Open(path)
}

func (s *FileStore) DeleteUserFile(uid int64, name string) error {
	path, err := s.path("user", strconv.FormatInt(uid, 10), name)
	if err != nil {
		return err
	}
	return removeFile(path)
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
	_, err := writeFileWithMetadata(path, filepath.Base(path), reader)
	return err
}

func writeFileWithMetadata(path, name string, reader io.Reader) (StoredFile, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return StoredFile{}, err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o640)
	if err != nil {
		return StoredFile{}, err
	}
	hash := md5.New()
	size, copyErr := io.Copy(io.MultiWriter(file, hash), reader)
	closeErr := file.Close()
	if copyErr != nil {
		return StoredFile{}, copyErr
	}
	if closeErr != nil {
		return StoredFile{}, closeErr
	}
	info, err := os.Stat(path)
	if err != nil {
		return StoredFile{}, err
	}
	return StoredFile{
		Name:         name,
		Size:         size,
		ETag:         hex.EncodeToString(hash.Sum(nil)),
		LastModified: info.ModTime().UTC(),
	}, nil
}

func removeFile(path string) error {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
