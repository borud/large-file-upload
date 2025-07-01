package transfer

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestID(t *testing.T) {
	id, err := NewID()
	require.NoError(t, err)
	require.NotEmpty(t, id)

	bi, err := id.AsBigInt()
	require.NoError(t, err)
	require.NotZero(t, bi)

	pathComponents, err := id.LowerBitPathComponents()
	require.NoError(t, err)
	require.NotZero(t, pathComponents)
	require.Contains(t, pathComponents, string(os.PathSeparator))

	id2, err := ParseID(id.String())
	require.NoError(t, err)
	require.Equal(t, id, id2)
}
