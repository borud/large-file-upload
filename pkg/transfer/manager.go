package transfer

import (
	"errors"
	"fmt"
	"log/slog"
)

// uploadManager takes care of managing uploads that are in progress
type uploadManager struct {
	uploads   map[ID]*upload
	fileStore *FileStore
}

// newManager creates a new upload manager
func newManager(fileStore *FileStore) (*uploadManager, error) {
	return &uploadManager{
		uploads:   map[ID]*upload{},
		fileStore: fileStore,
	}, nil
}

// CreateUpload creates a new upload
func (m *uploadManager) CreateUpload(size int64, meta []byte) (*upload, error) {
	id, err := NewID()
	if err != nil {
		return nil, err
	}

	_, ok := m.uploads[id]
	if ok {
		return nil, fmt.Errorf("inconsistency: id [%s] already exists", id)
	}

	uploadFile, err := m.fileStore.Create(id)
	if err != nil {
		return nil, fmt.Errorf("unable to create incoming file: %w", err)
	}

	upload := &upload{
		ID:       id,
		Size:     size,
		file:     uploadFile,
		Metadata: meta,
	}

	m.uploads[id] = upload

	return upload, nil
}

// GetUpload by id.  Returns nil if the upload does not exist.
func (m *uploadManager) GetUpload(id ID) *upload {
	return m.uploads[id]
}

// GetUploads returns a slice of all the uploads currently in progress.
func (m *uploadManager) GetUploads() []*upload {
	var uploads []*upload
	for _, v := range m.uploads {
		uploads = append(uploads, v)
	}

	return uploads
}

// Finish upload and close file
func (m *uploadManager) Finish(id ID) error {
	slog.Debug("finishing", "id", id)
	upload, ok := m.uploads[id]
	if !ok {
		return fmt.Errorf("upload [%s] does not exist", id)
	}

	delete(m.uploads, id)

	err := upload.file.Close()
	if err != nil {
		return fmt.Errorf("failed to close upload file [%s]: %w", upload.Filename(), err)
	}

	return nil
}

// Shutdown the manager.  Closes any remaining unclosed files.
func (m *uploadManager) Shutdown() error {
	var errs error
	for key := range m.uploads {
		errs = errors.Join(errs, m.Finish(key))
	}

	return errs
}
