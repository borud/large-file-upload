package transfer

import (
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"path"
)

// ID is the ID type.
type ID string

const (
	numIDBits    = 128
	encodingBase = 36
)

// errors
var (
	ErrInvalidID = errors.New("invalid ID")
)

// NewID creates a new ID
func NewID() (ID, error) {
	id, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), numIDBits))
	if err != nil {
		return "", fmt.Errorf("failed to generate ID: %w", err)
	}
	return ID(id.Text(encodingBase)), nil
}

// AsBigInt returns the ID as a *big.Int.  If the ID was invalid an error is returned.
func (id ID) AsBigInt() (*big.Int, error) {
	n, ok := new(big.Int).SetString(string(id), encodingBase)
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrInvalidID, id)
	}
	return n, nil
}

// ParseID is mainly offered so that we can verify that the IDs submitted
// by clients at least look like valid IDs.
func ParseID(s string) (ID, error) {
	if _, ok := new(big.Int).SetString(s, 36); !ok {
		return "", fmt.Errorf("%w: %s", ErrInvalidID, s)
	}
	return ID(s), nil
}

func (id ID) String() string {
	return string(id)
}

// LowerBitPathComponents is used to generate two levels of directory names
// from the lower 32 bits of the ID which can be used in file storage to get
// the files better distributed across directories.
func (id ID) LowerBitPathComponents() (string, error) {
	const dirBits = 32

	n, err := id.AsBigInt()
	if err != nil {
		return "", err
	}

	mask32 := big.NewInt(1)
	mask32.Lsh(mask32, dirBits).Sub(mask32, big.NewInt(1))

	low := new(big.Int).And(n, mask32)
	high := new(big.Int).Rsh(n, dirBits)
	high.And(high, mask32)

	return path.Join(low.Text(encodingBase), high.Text(encodingBase)), nil
}
