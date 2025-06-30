package transfer

import (
	"crypto/rand"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"os"
	"path"
)

// uploadManager takes care of managing uploads that are in progress
type uploadManager struct {
	incomingDirectory string
	uploads           map[string]*upload
}

const (
	incomingDirPermissions  = 0700
	incomingFilePermissions = 0600
	archiveDirPermissions   = 0700
	archiveFilePermissions  = 0700

	uploadIDNumBits = 128
)

// newManager creates a new upload manager
func newManager(incoming string) (*uploadManager, error) {
	err := os.MkdirAll(incoming, incomingDirPermissions)
	if err != nil {
		return nil, fmt.Errorf("unable to create incoming directory [%s]: %w", incoming, err)
	}

	return &uploadManager{
		incomingDirectory: incoming,
		uploads:           map[string]*upload{},
	}, nil
}

// CreateUpload creates a new upload
func (m *uploadManager) CreateUpload(size int64, meta []byte) (*upload, error) {
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

	upload := &upload{
		ID:       id,
		Size:     size,
		file:     uploadFile,
		Filename: fileName,
		Metadata: meta,
	}

	m.uploads[id] = upload

	return upload, nil
}

// GetUpload by id.  Returns nil if the upload does not exist.
func (m *uploadManager) GetUpload(id string) *upload {
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
func (m *uploadManager) Finish(id string) error {
	slog.Debug("finishing", "id", id)
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
func (m *uploadManager) Shutdown() error {
	var errs error
	for key := range m.uploads {
		errs = errors.Join(errs, m.Finish(key))
	}

	return errs
}
