package transfer

import (
	"crypto/rand"
	"path"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestManager(t *testing.T) {
	tmpRoot := t.TempDir()
	incoming := path.Join(tmpRoot, "incoming")

	m, err := newManager(incoming)
	require.NoError(t, err)

	wg := sync.WaitGroup{}

	for range 3 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			upload, err := m.CreateUpload(1000, []byte{0, 0})
			require.NoError(t, err)

			defer m.Finish(upload.ID)

			buf := make([]byte, 100)

			for range 10 {
				n, err := rand.Read(buf)
				require.NoError(t, err)
				require.Equal(t, n, 100)

				nn, err := upload.Write(buf)
				require.NoError(t, err)
				require.Equal(t, nn, n)
			}

			// make sure we do not allow writing a larger file than announced
			_, err = upload.Write([]byte{0})
			require.ErrorIs(t, ErrAttemptToWriteLargerFile, err)

		}()
	}

	wg.Wait()

	require.NoError(t, m.Shutdown())
}
