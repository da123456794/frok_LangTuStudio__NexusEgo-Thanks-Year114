package function

import (
	"sort"
	"time"
)

const postImportVerifyTimeout = 2 * time.Second

func sortChunkKeys(chunks [][2]int) {
	sort.Slice(chunks, func(i, j int) bool {
		if chunks[i][0] != chunks[j][0] {
			return chunks[i][0] < chunks[j][0]
		}
		return chunks[i][1] < chunks[j][1]
	})
}

func floorMod(value, divisor int) int {
	mod := value % divisor
	if mod < 0 {
		mod += divisor
	}
	return mod
}
