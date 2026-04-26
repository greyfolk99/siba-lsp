// Package bridge provides an in-process interface to the siba core engine.
// No subprocess — directly imports siba/pkg packages.
package bridge

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"

	"github.com/greyfolk99/siba/pkg/ast"
	"github.com/greyfolk99/siba/pkg/parser"
	"github.com/greyfolk99/siba/pkg/render"
	"github.com/greyfolk99/siba/pkg/validate"
	"github.com/greyfolk99/siba/pkg/workspace"
)

// Diagnostic mirrors ast.Diagnostic for external consumers
type Diagnostic struct {
	File     string `json:"file"`
	Line     int    `json:"line"`
	Column   int    `json:"column"`
	EndLine  int    `json:"end_line"`
	EndCol   int    `json:"end_column"`
	Severity string `json:"severity"`
	Code     string `json:"code"`
	Message  string `json:"message"`
}

// CheckResult for single file check
type CheckResult struct {
	File        string       `json:"file"`
	DocName     string       `json:"doc_name"`
	ExtendsName string       `json:"extends"`
	IsTemplate  bool         `json:"is_template"`
	Variables   int          `json:"variables"`
	References  int          `json:"references"`
	Headings    int          `json:"headings"`
	Errors      int          `json:"errors"`
	Warnings    int          `json:"warnings"`
	Diagnostics []Diagnostic `json:"diagnostics"`
}

// CheckWorkspaceResult for workspace-wide check
type CheckWorkspaceResult struct {
	Root        string        `json:"root"`
	Version     string        `json:"version"`
	Documents   int           `json:"documents"`
	Templates   int           `json:"templates"`
	TotalErrors int           `json:"total_errors"`
	TotalWarns  int           `json:"total_warnings"`
	Files       []CheckResult `json:"files"`
	Workspace   []Diagnostic  `json:"workspace_diagnostics"`
}

// RenderResult for rendering
type RenderResult struct {
	Content string
	Error   string
}

// Bridge provides in-process access to the siba engine
type Bridge struct {
	WorkDir   string
	workspace *workspace.Workspace
}

// New creates a new Bridge
func New(workDir string) *Bridge {
	return &Bridge{WorkDir: workDir}
}

func (b *Bridge) loadWorkspace() (*workspace.Workspace, error) {
	if b.workspace != nil {
		return b.workspace, nil
	}
	ws, err := workspace.LoadWorkspace(b.WorkDir)
	if err != nil {
		return nil, err
	}
	b.workspace = ws
	return ws, nil
}

// RefreshFile re-parses a single file in the workspace
func (b *Bridge) RefreshFile(path string, source string) {
	if b.workspace != nil {
		b.workspace.RefreshDocument(path, source)
	}
}

// CheckFile validates a single document
func (b *Bridge) CheckFile(path string) (*CheckResult, error) {
	ws, err := b.loadWorkspace()
	if err != nil {
		return nil, err
	}

	doc := ws.GetDocumentByPath(path)
	if doc == nil {
		// try parsing directly
		source, err := readFile(b.WorkDir, path)
		if err != nil {
			return nil, err
		}
		doc = parser.ParseDocument(path, source)
	}

	allDiags := doc.Diagnostics
	allDiags = append(allDiags, validate.ValidateDocument(doc, ws)...)

	return buildCheckResult(path, doc, allDiags), nil
}

// CheckWorkspace validates the entire workspace
func (b *Bridge) CheckWorkspace() (*CheckWorkspaceResult, error) {
	ws, err := b.loadWorkspace()
	if err != nil {
		return nil, err
	}

	fileDiags, wsDiags := validate.ValidateWorkspace(ws)

	result := &CheckWorkspaceResult{
		Root:      b.WorkDir,
		Version:   ws.GetVersion(),
		Documents: len(ws.Documents),
		Templates: len(ws.Templates),
	}

	for path, doc := range ws.DocsByPath {
		diags := fileDiags[path]
		diags = append(diags, doc.Diagnostics...)
		cr := buildCheckResult(path, doc, diags)
		result.Files = append(result.Files, *cr)
		result.TotalErrors += cr.Errors
		result.TotalWarns += cr.Warnings
	}

	for _, d := range wsDiags {
		result.Workspace = append(result.Workspace, convertDiag(d))
		if d.Severity == ast.SeverityError {
			result.TotalErrors++
		}
	}

	return result, nil
}

// RenderFile renders a single document
func (b *Bridge) RenderFile(path string) (*RenderResult, error) {
	ws, _ := b.loadWorkspace()

	source, err := readFile(b.WorkDir, path)
	if err != nil {
		return &RenderResult{Error: err.Error()}, nil
	}

	doc := parser.ParseDocument(path, source)

	var buf bytes.Buffer
	if err := render.StreamRender(doc, &buf, ws); err != nil {
		return &RenderResult{Error: err.Error()}, nil
	}

	return &RenderResult{Content: buf.String()}, nil
}

// Ls returns workspace listing as JSON
func (b *Bridge) Ls() (json.RawMessage, error) {
	ws, err := b.loadWorkspace()
	if err != nil {
		return nil, err
	}

	type docInfo struct {
		Name       string `json:"name"`
		Path       string `json:"path"`
		IsTemplate bool   `json:"is_template"`
	}

	var docs []docInfo
	for name, doc := range ws.Templates {
		docs = append(docs, docInfo{Name: name, Path: doc.Path, IsTemplate: true})
	}
	for name, doc := range ws.Documents {
		docs = append(docs, docInfo{Name: name, Path: doc.Path, IsTemplate: false})
	}

	data, _ := json.Marshal(docs)
	return data, nil
}

// Tree returns heading tree as JSON
func (b *Bridge) Tree(file string) (json.RawMessage, error) {
	ws, err := b.loadWorkspace()
	if err != nil {
		return nil, err
	}

	if file != "" {
		doc := ws.GetDocumentByPath(file)
		if doc == nil {
			return json.Marshal(nil)
		}
		return json.Marshal(doc.Headings)
	}

	result := make(map[string]interface{})
	for path, doc := range ws.DocsByPath {
		result[path] = doc.Headings
	}
	return json.Marshal(result)
}

// Find searches workspace for a query
func (b *Bridge) Find(query string) (json.RawMessage, error) {
	ws, err := b.loadWorkspace()
	if err != nil {
		return nil, err
	}

	type match struct {
		File string `json:"file"`
		Line int    `json:"line"`
		Text string `json:"text"`
	}

	var matches []match
	for path, doc := range ws.DocsByPath {
		lines := strings.Split(doc.Source, "\n")
		for i, line := range lines {
			if strings.Contains(line, query) {
				matches = append(matches, match{
					File: path,
					Line: i + 1,
					Text: line,
				})
			}
		}
	}

	return json.Marshal(matches)
}

// --- helpers ---

func buildCheckResult(path string, doc *ast.Document, diags []ast.Diagnostic) *CheckResult {
	cr := &CheckResult{
		File:        path,
		DocName:     doc.Name,
		ExtendsName: doc.ExtendsName,
		IsTemplate:  doc.IsTemplate,
		Variables:   len(doc.Variables),
		References:  len(doc.References),
		Headings:    countHeadings(doc.Headings),
	}
	for _, d := range diags {
		cr.Diagnostics = append(cr.Diagnostics, convertDiag(d))
		switch d.Severity {
		case ast.SeverityError:
			cr.Errors++
		case ast.SeverityWarning:
			cr.Warnings++
		}
	}
	if cr.Diagnostics == nil {
		cr.Diagnostics = []Diagnostic{}
	}
	return cr
}

func convertDiag(d ast.Diagnostic) Diagnostic {
	sev := "error"
	switch d.Severity {
	case ast.SeverityWarning:
		sev = "warning"
	case ast.SeverityInfo:
		sev = "info"
	case ast.SeverityHint:
		sev = "hint"
	}
	return Diagnostic{
		File:     d.File,
		Line:     d.Range.Start.Line,
		Column:   d.Range.Start.Column,
		EndLine:  d.Range.End.Line,
		EndCol:   d.Range.End.Column,
		Severity: sev,
		Code:     d.Code,
		Message:  d.Message,
	}
}

func countHeadings(headings []*ast.Heading) int {
	n := len(headings)
	for _, h := range headings {
		n += countHeadings(h.Children)
	}
	return n
}

func readFile(workDir, path string) (string, error) {
	fullPath := path
	if workDir != "" && !strings.HasPrefix(path, "/") {
		fullPath = workDir + "/" + path
	}
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
