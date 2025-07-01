package transfer

import (
	"crypto/rand"
	"os"
	"path"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFileStore(t *testing.T) {
	root := t.TempDir()

	fs, err := CreateFileStore(root)
	require.NoError(t, err)
	require.NotNil(t, fs)

	id, err := NewID()
	require.NoError(t, err)
	require.NotNil(t, id)

	fullpath, err := fs.Map(id)
	require.NoError(t, err)
	require.NotEmpty(t, fullpath)

	f, err := fs.Create(id)
	require.NoError(t, err)
	require.NotNil(t, f)

	// make some random data
	buf := make([]byte, 1024)
	n, err := rand.Read(buf)
	require.NoError(t, err)
	require.Equal(t, 1024, n)

	// write the data
	nn, err := f.Write(buf)
	require.NoError(t, err)
	require.Equal(t, 1024, nn)
	require.NoError(t, f.Close())

	// read the data
	readbuf := make([]byte, 1024)
	rf, err := fs.OpenReadOnly(id)
	require.NoError(t, err)
	n, err = rf.Read(readbuf)
	require.NoError(t, err)
	require.Equal(t, 1024, n)
	require.Equal(t, buf, readbuf)

	// dump a file in the second to last dir
	full := strings.Split(fullpath, string(os.PathSeparator))
	target := strings.Join(full[:len(full)-2], string(os.PathSeparator))

	err = os.WriteFile(path.Join(target, "cuckoo"), []byte("this is a test"), 0600)
	require.NoError(t, err)

	// remove the original file.  Since we dumped a file in the directory two above this
	// one we should not remove the intermediate dir.
	require.NoError(t, fs.Remove(id))

	require.NoFileExists(t, fullpath)
	require.NoDirExists(t, strings.Join(full[:len(full)-1], string(os.PathSeparator)))
	require.DirExists(t, target)

	// now remove the cuckoo and try again
	require.NoError(t, os.Remove(path.Join(target, "cuckoo")))

	removeEmptyDirsUpTo(target, root)
	require.NoDirExists(t, target)
	require.DirExists(t, root)
}
