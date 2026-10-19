// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"bytes"
	"fmt"
)

// splitFrontMatter extracts a leading "---\n...\n---\n" YAML block from src. It returns
// the YAML bytes (nil if there is no front matter), the remaining body bytes, and the
// number of lines consumed by the front matter block (including both delimiter lines),
// used to offset positions reported from the body.
func splitFrontMatter(src []byte) (yamlBytes, body []byte, lineOffset int, err error) {
	const delim = "---"
	if !bytes.HasPrefix(src, []byte(delim)) {
		return nil, src, 0, nil
	}
	rest := src[len(delim):]
	if len(rest) > 0 && rest[0] != '\n' && rest[0] != '\r' {
		// "---" followed by other characters is a thematic break or heading, not a
		// front matter delimiter.
		return nil, src, 0, nil
	}

	lines := bytes.SplitAfter(src, []byte("\n"))
	closing := -1
	for i := 1; i < len(lines); i++ {
		trimmed := bytes.TrimRight(lines[i], "\r\n")
		if string(trimmed) == delim {
			closing = i
			break
		}
	}
	if closing == -1 {
		return nil, nil, 0, fmt.Errorf("front matter opening %q has no closing delimiter", delim)
	}

	var fm bytes.Buffer
	for i := 1; i < closing; i++ {
		fm.Write(lines[i])
	}
	var rem bytes.Buffer
	for i := closing + 1; i < len(lines); i++ {
		rem.Write(lines[i])
	}
	return fm.Bytes(), rem.Bytes(), closing + 1, nil
}
