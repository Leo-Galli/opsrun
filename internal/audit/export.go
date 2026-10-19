// SPDX-License-Identifier: Apache-2.0

package audit

import (
	"fmt"
	"io"
	"strconv"
	"strings"
)

// ExportMarkdown renders every record in the audit log at path as a Markdown
// post-mortem timeline, optionally restricted to a single run id.
func ExportMarkdown(path, runID string, w io.Writer) error {
	records, err := ReadAll(path)
	if err != nil {
		return err
	}

	fmt.Fprintf(w, "# Incident timeline\n\n")
	if runID != "" {
		fmt.Fprintf(w, "Run: `%s`\n\n", runID)
	}
	fmt.Fprintf(w, "| Time (UTC) | Run | Step | Host | Event | Command | Exit |\n")
	fmt.Fprintf(w, "|---|---|---|---|---|---|---|\n")

	for _, r := range records {
		if runID != "" && r.RunID != runID {
			continue
		}
		exit := ""
		if r.ExitCode != nil {
			exit = strconv.Itoa(*r.ExitCode)
		}
		fmt.Fprintf(w, "| %s | %s | %s | %s | %s | %s | %s |\n",
			r.Timestamp, mdCell(r.RunID), mdCell(r.StepID), mdCell(r.Host),
			mdCell(string(r.Type)), mdCell(r.Command), exit)
	}
	return nil
}

func mdCell(s string) string {
	if s == "" {
		return ""
	}
	s = strings.ReplaceAll(s, "|", `\|`)
	s = strings.ReplaceAll(s, "\n", " ")
	return s
}
