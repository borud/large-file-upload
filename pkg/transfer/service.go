// Package transfer implements the gRPC service for uploads.
package transfer

import "fmt"

// Service implements the upload service
type Service struct {
	UploadManager *uploadManager
	fileStore     *FileStore
	config        Config
}

// Config for transfer service. Make sure that the PreferredBlockSize is set to something
// sensible.
type Config struct {
	IncomingDir        string
	PreferredBlockSize int64
	UploadFinishedHook HookFunc
	UploadProgressHook HookFunc
	UploadCreatedHook  HookFunc
}

// HookFunc defines the callback hook function type.
type HookFunc func(filename string, size int64, offset int64, metadata []byte)

// NewService creates a new transfer service
func NewService(c Config) (*Service, error) {
	fileStore, err := CreateFileStore(c.IncomingDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create filestore: %w", err)
	}

	uploadManager, err := newManager(fileStore)
	if err != nil {
		return nil, err
	}

	return &Service{
		UploadManager: uploadManager,
		config:        c,
		fileStore:     fileStore,
	}, nil
}
