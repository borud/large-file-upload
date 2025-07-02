package transfer

import (
	"errors"
	"os"
	"sync"
)

// upload represents an active upload.  It keeps track of the offset of the upload.
// The offset is protected by a mutex so any changes to the underlying file will
// be in sync with the writeOffset.
type upload struct {
	ID          ID
	Size        int64
	Metadata    []byte
	FileSHA256  []byte
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
func (u *upload) Write(b []byte) (int, error) {
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
func (u *upload) Offset() int64 {
	u.mu.RLock()
	defer u.mu.RUnlock()

	return u.writeOffset
}

// Filename returns the name of the file we are writing to and "" if there is
// no file. The name returned is the same that was presented to the Open()
// call.
func (u *upload) Filename() string {
	u.mu.RLock()
	defer u.mu.RUnlock()

	if u.file == nil {
		return ""
	}

	return u.file.Name()
}
