// Package upload is an upload manager
package upload

import (
	"crypto/rand"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"os"
	"path"
)

// Manager takes care of managing uploads that are in progress
type Manager struct {
	incomingDirectory string
	archiveDirectory  string
	uploads           map[string]*Upload
}

const (
	incomingDirPermissions  = 0700
	incomingFilePermissions = 0600
	archiveDirPermissions   = 0700
	archiveFilePermissions  = 0700

	uploadIDNumBits = 128
)

// NewManager creates a new upload manager
func NewManager(incoming, archive string) (*Manager, error) {
	err := os.MkdirAll(incoming, incomingDirPermissions)
	if err != nil {
		return nil, fmt.Errorf("unable to create incoming directory [%s]: %w", incoming, err)
	}

	err = os.MkdirAll(archive, archiveDirPermissions)
	if err != nil {
		return nil, fmt.Errorf("unable to create archive directory [%s]: %w", incoming, err)
	}

	return &Manager{
		incomingDirectory: incoming,
		archiveDirectory:  archive,
		uploads:           map[string]*Upload{},
	}, nil
}

// CreateUpload creates a new upload
func (m *Manager) CreateUpload(size int64, meta []byte) (*Upload, error) {
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), uploadIDNumBits))
	if err != nil {
		return nil, fmt.Errorf("unable to generate ID: %w", err)
	}

	id := serial.Text(36)

	_, ok := m.uploads[id]
	if ok {
		return nil, fmt.Errorf("inconsistency: id [%s] already exists", id)
	}

	fileName := path.Join(m.incomingDirectory, id)

	uploadFile, err := os.OpenFile(fileName, os.O_CREATE|os.O_EXCL|os.O_WRONLY|os.O_SYNC, incomingFilePermissions)
	if err != nil {
		return nil, fmt.Errorf("unable to create incoming file [%s]: %w", fileName, err)
	}

	upload := &Upload{
		ID:       id,
		Size:     size,
		file:     uploadFile,
		Filename: fileName,
		Meta:     meta,
	}

	m.uploads[id] = upload

	return upload, nil
}

// GetUpload by id.  Returns nil if the upload does not exist.
func (m *Manager) GetUpload(id string) *Upload {
	return m.uploads[id]
}

// GetUploads returns a slice of all the uploads currently in progress.
func (m *Manager) GetUploads() []*Upload {
	var uploads []*Upload
	for _, v := range m.uploads {
		uploads = append(uploads, v)
	}

	return uploads
}

// Finish upload and close file
func (m *Manager) Finish(id string) error {
	slog.Info("finishing", "id", id)
	upload, ok := m.uploads[id]
	if !ok {
		return fmt.Errorf("upload [%s] does not exist", id)
	}

	delete(m.uploads, id)

	err := upload.file.Close()
	if err != nil {
		return fmt.Errorf("failed to close upload file [%s]: %w", upload.Filename, err)
	}

	return nil
}

// Shutdown the manager.
func (m *Manager) Shutdown() error {
	var errs error
	for key := range m.uploads {
		errs = errors.Join(errs, m.Finish(key))
	}

	return errs
}
