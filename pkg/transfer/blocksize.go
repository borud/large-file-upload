package transfer

const (
	minBlockSize     = 10 * 1024
	maxBlockSize     = 2 * 1024 * 1024
	defaultBlockSize = maxBlockSize / 2
)

// clampBlockSize returns block size clamped between minBlockSize and
// maxBlockSize.  If bs is zero we use default block size.
func clampBlockSize(bs int64) int64 {
	if bs == 0 {
		return defaultBlockSize
	}
	return min(max(bs, minBlockSize), maxBlockSize)
}
