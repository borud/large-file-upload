package upload

import (
	"crypto/rand"
	"fmt"
	"path"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestManager(t *testing.T) {
	tmpRoot := t.TempDir()
	incoming := path.Join(tmpRoot, "incoming")
	archive := path.Join(tmpRoot, "archive")

	m, err := NewManager(incoming, archive)
	require.NoError(t, err)

	wg := sync.WaitGroup{}

	for i := range 10 {
		wg.Add(1)
		go func(num int) {
			defer wg.Done()
			upload, err := m.CreateUpload(1234*10, []byte{0, 0})
			require.NoError(t, err)

			defer m.Finish(upload.ID)

			buf := make([]byte, 1234)

			for range 10 {
				n, err := rand.Read(buf)
				require.NoError(t, err)
				require.Equal(t, n, 1234)

				nn, err := upload.Write(buf)
				require.NoError(t, err)
				require.Equal(t, nn, n)

				fmt.Printf("%d offset=%d\n", i, upload.Offset())
			}

			// make sure we do not allow writing a larger file than announced
			_, err = upload.Write([]byte{0})
			require.ErrorIs(t, ErrAttemptToWriteLargerFile, err)

		}(i)
	}

	wg.Wait()

	require.NoError(t, m.Shutdown())
}
