// SPDX-License-Identifier: Apache-2.0

package parser

// lineIndex converts byte offsets within a buffer to 1-based line and column numbers.
type lineIndex struct {
	starts []int
}

func newLineIndex(src []byte) *lineIndex {
	li := &lineIndex{starts: []int{0}}
	for i, b := range src {
		if b == '\n' {
			li.starts = append(li.starts, i+1)
		}
	}
	return li
}

// Position returns the 1-based line and column for a byte offset.
func (li *lineIndex) Position(offset int) (line, col int) {
	if offset < 0 {
		return 1, 1
	}
	// Binary search for the last line start <= offset.
	lo, hi := 0, len(li.starts)-1
	for lo < hi {
		mid := (lo + hi + 1) / 2
		if li.starts[mid] <= offset {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	return lo + 1, offset - li.starts[lo] + 1
}
