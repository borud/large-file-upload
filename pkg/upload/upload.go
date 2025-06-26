package upload

import (
	"errors"
	"os"
	"sync"
)

// Upload represents an active upload.  It keeps track of the offset of the upload.
// The offset is protected by a mutex so any changes to the underlying file will
// be in sync with the writeOffset.
type Upload struct {
	ID          string
	Size        int64
	Filename    string
	Meta        []byte
	mu          sync.RWMutex
	file        *os.File
	writeOffset int64
}

var (
	// ErrAttemptToWriteLargerFile is returned from Write() if we try to write
	// more bytes than the file was declared to hold.
	ErrAttemptToWriteLargerFile = errors.New("attempted to write more bytes than declared file size")
)

// Write bytes to file and update the write offset.  We use a mutex around this
// since the write mutates what the value represents.
func (u *Upload) Write(b []byte) (int, error) {
	u.mu.Lock()
	defer u.mu.Unlock()

	if (u.writeOffset + int64(len(b))) > u.Size {
		return 0, ErrAttemptToWriteLargerFile
	}

	n, err := u.file.Write(b)
	if err != nil {
		return n, err
	}

	u.writeOffset += int64(n)
	return n, nil
}

// Offset returns the current offset of the upload.
func (u *Upload) Offset() int64 {
	u.mu.RLock()
	defer u.mu.RUnlock()

	return u.writeOffset
}
