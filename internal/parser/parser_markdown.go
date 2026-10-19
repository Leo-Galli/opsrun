// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"context"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
	"gopkg.in/yaml.v3"

	"github.com/Leo-Galli/opsrun/internal/runbook"
)

// ParseError is a parse-time error with a position in the source document.
type ParseError struct {
	Message  string
	Position runbook.Position
}

// Error implements the error interface.
func (e *ParseError) Error() string {
	if e.Position.Line > 0 {
		return fmt.Sprintf("%d:%d: %s", e.Position.Line, e.Position.Column, e.Message)
	}
	return e.Message
}

// frontMatterVariable is the front matter shape of a declared runbook variable.
type frontMatterVariable struct {
	Type       string `yaml:"type"`
	Default    any    `yaml:"default"`
	Required   bool   `yaml:"required"`
	Secret     bool   `yaml:"secret"`
	Validation string `yaml:"validation"`
}

type frontMatterTarget struct {
	Host  string `yaml:"host"`
	Group string `yaml:"group"`
}

type frontMatterApproval struct {
	Mode    string `yaml:"mode"`
	Timeout string `yaml:"timeout"`
}

type frontMatter struct {
	Name     string                         `yaml:"name"`
	Version  string                         `yaml:"version"`
	Service  string                         `yaml:"service"`
	Severity string                         `yaml:"severity"`
	Author   string                         `yaml:"author"`
	Targets  []frontMatterTarget            `yaml:"targets"`
	Vars     map[string]frontMatterVariable `yaml:"vars"`
	Approval frontMatterApproval            `yaml:"approval"`
}

// stepDirectives is the shape of the optional directive block immediately following a
// step heading.
type stepDirectives struct {
	DependsOn       []string `yaml:"depends_on"`
	Confirm         bool     `yaml:"confirm"`
	Retries         int      `yaml:"retries"`
	ContinueOnError bool     `yaml:"continue_on_error"`
	Mode            string   `yaml:"mode"`
	ReadOnly        bool     `yaml:"read_only"`
	ApprovalMode    string   `yaml:"approval_mode"`
	ApprovalTimeout string   `yaml:"approval_timeout"`
}

var knownDirectiveKeys = map[string]bool{
	"depends_on": true, "confirm": true, "retries": true,
	"continue_on_error": true, "mode": true, "read_only": true,
	"approval_mode": true, "approval_timeout": true,
}

var stepHeadingPattern = regexp.MustCompile(`^Step:\s*([A-Za-z0-9_-]+)\s*-\s*(.+)$`)

// MarkdownParser implements RunbookParser using goldmark to walk the Markdown AST.
type MarkdownParser struct{}

// NewMarkdownParser returns a RunbookParser backed by goldmark.
func NewMarkdownParser() *MarkdownParser {
	return &MarkdownParser{}
}

// Parse implements RunbookParser.
func (p *MarkdownParser) Parse(_ context.Context, r io.Reader) (*runbook.Runbook, error) {
	raw, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("read runbook: %w", err)
	}

	fmBytes, body, headerLines, err := splitFrontMatter(raw)
	if err != nil {
		return nil, &ParseError{Message: err.Error(), Position: runbook.Position{Line: 1, Column: 1}}
	}
	if fmBytes == nil {
		return nil, &ParseError{Message: "runbook must start with a YAML front matter block delimited by \"---\"", Position: runbook.Position{Line: 1, Column: 1}}
	}

	var fm frontMatter
	dec := yaml.NewDecoder(strings.NewReader(string(fmBytes)))
	dec.KnownFields(true)
	if err := dec.Decode(&fm); err != nil {
		return nil, &ParseError{Message: fmt.Sprintf("parse front matter: %s", err), Position: runbook.Position{Line: 1, Column: 1}}
	}

	rb, err := buildRunbookFromFrontMatter(fm)
	if err != nil {
		return nil, &ParseError{Message: err.Error(), Position: runbook.Position{Line: 1, Column: 1}}
	}

	li := newLineIndex(body)
	offset := func(byteOffset int) runbook.Position {
		line, col := li.Position(byteOffset)
		return runbook.Position{Line: line + headerLines, Column: col}
	}

	md := goldmark.New()
	doc := md.Parser().Parse(text.NewReader(body))

	steps, err := walkSteps(doc, body, offset)
	if err != nil {
		return nil, err
	}
	rb.Steps = steps

	return rb, nil
}

func buildRunbookFromFrontMatter(fm frontMatter) (*runbook.Runbook, error) {
	if fm.Name == "" {
		return nil, fmt.Errorf("front matter is missing required field \"name\"")
	}

	rb := &runbook.Runbook{
		Name:     fm.Name,
		Version:  fm.Version,
		Service:  fm.Service,
		Severity: runbook.Severity(fm.Severity),
		Author:   fm.Author,
		Vars:     make(map[string]runbook.Variable, len(fm.Vars)),
	}

	for _, t := range fm.Targets {
		rb.Targets = append(rb.Targets, runbook.Target{Host: t.Host, Group: t.Group})
	}

	for name, v := range fm.Vars {
		rb.Vars[name] = runbook.Variable{
			Name:       name,
			Type:       runbook.VariableType(v.Type),
			Default:    v.Default,
			Required:   v.Required,
			Secret:     v.Secret,
			Validation: v.Validation,
		}
	}

	rb.Approval.Mode = runbook.ApprovalModeNone
	if fm.Approval.Mode != "" {
		rb.Approval.Mode = runbook.ApprovalMode(fm.Approval.Mode)
	}
	if fm.Approval.Timeout != "" {
		d, err := time.ParseDuration(fm.Approval.Timeout)
		if err != nil {
			return nil, fmt.Errorf("invalid approval.timeout %q: %w", fm.Approval.Timeout, err)
		}
		rb.Approval.Timeout = d
	}

	return rb, nil
}

type positionFunc func(byteOffset int) runbook.Position

func walkSteps(doc ast.Node, source []byte, pos positionFunc) ([]*runbook.Step, error) {
	var steps []*runbook.Step
	var cur *runbook.Step
	directivesPending := false

	blockPos := func(n ast.Node) runbook.Position {
		if n.Lines().Len() > 0 {
			return pos(n.Lines().At(0).Start)
		}
		return pos(0)
	}

	for n := doc.FirstChild(); n != nil; n = n.NextSibling() {
		switch node := n.(type) {
		case *ast.Heading:
			if node.Level != 2 {
				continue
			}
			headingText := string(node.Text(source))
			m := stepHeadingPattern.FindStringSubmatch(strings.TrimSpace(headingText))
			if m == nil {
				continue
			}
			cur = &runbook.Step{ID: m[1], Title: m[2], Position: blockPos(node), Mode: runbook.ExecutionModeParallel}
			steps = append(steps, cur)
			directivesPending = true

		case *ast.Paragraph:
			if cur == nil {
				continue
			}
			raw := string(node.Text(source))
			if directivesPending {
				directivesPending = false
				if applied, err := applyDirectivesIfPresent(cur, raw, blockPos(node)); err != nil {
					return nil, err
				} else if applied {
					continue
				}
			}
			cur.Description = strings.TrimSpace(strings.Join([]string{cur.Description, raw}, "\n"))

		case *ast.FencedCodeBlock:
			if cur == nil {
				continue
			}
			directivesPending = false
			p := blockPos(node)
			if node.Info != nil {
				p = pos(node.Info.Segment.Start)
			}
			if err := applyAction(cur, node, source, p); err != nil {
				return nil, err
			}

		default:
			directivesPending = false
		}
	}

	return steps, nil
}

// applyDirectivesIfPresent attempts to interpret raw as the step's YAML directive block.
// It returns applied=true if raw was recognized and consumed as directives.
func applyDirectivesIfPresent(step *runbook.Step, raw string, p runbook.Position) (bool, error) {
	var probe map[string]any
	if err := yaml.Unmarshal([]byte(raw), &probe); err != nil || probe == nil {
		return false, nil
	}
	for k := range probe {
		if !knownDirectiveKeys[k] {
			return false, nil
		}
	}

	var d stepDirectives
	dec := yaml.NewDecoder(strings.NewReader(raw))
	dec.KnownFields(true)
	if err := dec.Decode(&d); err != nil {
		return false, &ParseError{Message: fmt.Sprintf("step %q: invalid directive block: %s", step.ID, err), Position: p}
	}

	step.DependsOn = d.DependsOn
	step.Confirm = d.Confirm
	step.Retries = d.Retries
	step.ContinueOnError = d.ContinueOnError
	step.ReadOnly = d.ReadOnly
	if d.Mode != "" {
		step.Mode = runbook.ExecutionMode(d.Mode)
	}
	if d.ApprovalMode != "" {
		policy := runbook.ApprovalPolicy{Mode: runbook.ApprovalMode(d.ApprovalMode)}
		if d.ApprovalTimeout != "" {
			dur, err := time.ParseDuration(d.ApprovalTimeout)
			if err != nil {
				return false, &ParseError{Message: fmt.Sprintf("step %q: invalid approval_timeout %q: %s", step.ID, d.ApprovalTimeout, err), Position: p}
			}
			policy.Timeout = dur
		}
		step.Approval = &policy
	}
	return true, nil
}

func applyAction(step *runbook.Step, node *ast.FencedCodeBlock, source []byte, p runbook.Position) error {
	info := ""
	if node.Info != nil {
		info = string(node.Info.Segment.Value(source))
	}
	fields := strings.Fields(info)
	if len(fields) < 2 || !strings.HasPrefix(fields[1], "opsrun:") {
		return nil // not an OpsRun action block; leave it as illustrative content
	}
	kind := strings.TrimPrefix(fields[1], "opsrun:")

	a := &runbook.Action{Position: p}
	switch kind {
	case "run":
		a.Kind = runbook.ActionKindRun
	case "check":
		a.Kind = runbook.ActionKindCheck
	case "rollback":
		a.Kind = runbook.ActionKindRollback
	default:
		return &ParseError{Message: fmt.Sprintf("step %q: unknown action kind %q", step.ID, kind), Position: p}
	}

	for _, kv := range fields[2:] {
		key, val, ok := strings.Cut(kv, "=")
		if !ok {
			return &ParseError{Message: fmt.Sprintf("step %q: malformed code block attribute %q", step.ID, kv), Position: p}
		}
		switch key {
		case "target":
			a.Target = val
		case "timeout":
			d, err := time.ParseDuration(val)
			if err != nil {
				return &ParseError{Message: fmt.Sprintf("step %q: invalid timeout %q: %s", step.ID, val, err), Position: p}
			}
			a.Timeout = d
		case "expect_exit":
			code, err := strconv.Atoi(val)
			if err != nil {
				return &ParseError{Message: fmt.Sprintf("step %q: invalid expect_exit %q: %s", step.ID, val, err), Position: p}
			}
			a.ExpectExit = code
		default:
			return &ParseError{Message: fmt.Sprintf("step %q: unknown code block attribute %q", step.ID, key), Position: p}
		}
	}

	var buf strings.Builder
	lines := node.Lines()
	for i := 0; i < lines.Len(); i++ {
		seg := lines.At(i)
		buf.Write(seg.Value(source))
	}
	a.Command = strings.TrimRight(buf.String(), "\n")

	switch a.Kind {
	case runbook.ActionKindRun:
		step.Run = a
	case runbook.ActionKindCheck:
		step.Checks = append(step.Checks, a)
	case runbook.ActionKindRollback:
		step.Rollback = a
	}
	return nil
}
