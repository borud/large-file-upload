package transfer

import (
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
)

// FileStore is a file store for the transfer service. If multiple
// implementations are required this can be turned into an interface.
type FileStore struct {
	root string
}

const (
	dirPermissions  = 0700
	filePermissions = 0600
)

// CreateFileStore creates a new FileStore instance.  If the root directory does
// not already exist it will be created.
func CreateFileStore(root string) (*FileStore, error) {
	// this is a no-op if the directory already exists
	err := os.MkdirAll(root, dirPermissions)
	if err != nil {
		return nil, fmt.Errorf("error creating filestore root directory: %w", err)
	}

	return &FileStore{root: root}, err
}

// Create file for append only.
func (f *FileStore) Create(id ID) (*os.File, error) {
	path, err := f.Map(id)
	if err != nil {
		return nil, err
	}

	err = os.MkdirAll(filepath.Dir(path), dirPermissions)
	if err != nil {
		return nil, fmt.Errorf("path %s: %w", path, err)
	}

	fd, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY|os.O_SYNC, filePermissions)
	if err != nil {
		return nil, fmt.Errorf("path %s: %w", path, err)
	}
	return fd, nil
}

// OpenReadOnly open file for read only
func (f *FileStore) OpenReadOnly(id ID) (*os.File, error) {
	path, err := f.Map(id)
	if err != nil {
		return nil, err
	}

	fd, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("path %s: %w", path, err)
	}
	return fd, nil
}

// Map id to filename
func (f *FileStore) Map(id ID) (string, error) {
	pathComponents, err := id.LowerBitPathComponents()
	if err != nil {
		return "", err
	}

	return path.Join(f.root, pathComponents, id.String()), nil
}

// Remove file by id.
func (f *FileStore) Remove(id ID) error {
	path, err := f.Map(id)
	if err != nil {
		return err
	}

	err = os.Remove(path)
	if err != nil {
		return fmt.Errorf("path %s: %w", path, err)
	}

	return removeEmptyDirsUpTo(filepath.Dir(path), f.root)
}

// removeEmptyDirsUpTo removes all empty directories up to, but not including, root.
func removeEmptyDirsUpTo(path, root string) error {
	path = filepath.Clean(path)
	root = filepath.Clean(root)

	for {
		if path == root || len(path) <= len(root) {
			break
		}

		empty, err := isDirEmpty(path)
		if err != nil || !empty {
			break
		}

		if err := os.Remove(path); err != nil {
			return err
		}

		path = filepath.Dir(path)
	}

	return nil
}

func isDirEmpty(path string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()

	_, err = f.Readdirnames(1)
	if err == io.EOF {
		return true, nil
	}
	return false, err
}
