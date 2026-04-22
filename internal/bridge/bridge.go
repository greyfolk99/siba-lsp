// Package bridge calls the siba CLI as a subprocess and parses JSON output.
package bridge

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
)

// Diagnostic from siba check --json
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

// CheckResult from siba check --json <file>
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

// CheckWorkspaceResult from siba check --json (workspace)
type CheckWorkspaceResult struct {
	Root        string       `json:"root"`
	Version     string       `json:"version"`
	Documents   int          `json:"documents"`
	Templates   int          `json:"templates"`
	TotalErrors int          `json:"total_errors"`
	TotalWarns  int          `json:"total_warnings"`
	Files       []CheckResult `json:"files"`
	Workspace   []Diagnostic `json:"workspace_diagnostics"`
}

// RenderResult from siba render --json <file>
type RenderResult struct {
	File    string `json:"file"`
	Content string `json:"content"`
	Error   string `json:"error"`
}

// Bridge communicates with the siba CLI
type Bridge struct {
	SibaPath string // path to siba binary, default "siba"
	WorkDir  string // working directory for siba commands
}

// New creates a new Bridge
func New(workDir string) *Bridge {
	return &Bridge{
		SibaPath: "siba",
		WorkDir:  workDir,
	}
}

// CheckFile runs siba check --json on a single file
func (b *Bridge) CheckFile(path string) (*CheckResult, error) {
	stdout, _, err := b.run("check", "--json", path)
	if err != nil {
		// siba exits 1 on errors but still produces JSON
		if len(stdout) == 0 {
			return nil, fmt.Errorf("siba check failed: %w", err)
		}
	}

	var result CheckResult
	if err := json.Unmarshal(stdout, &result); err != nil {
		return nil, fmt.Errorf("parse check result: %w", err)
	}
	return &result, nil
}

// CheckWorkspace runs siba check --json on the whole workspace
func (b *Bridge) CheckWorkspace() (*CheckWorkspaceResult, error) {
	stdout, _, err := b.run("check", "--json")
	if err != nil {
		if len(stdout) == 0 {
			return nil, fmt.Errorf("siba check failed: %w", err)
		}
	}

	var result CheckWorkspaceResult
	if err := json.Unmarshal(stdout, &result); err != nil {
		return nil, fmt.Errorf("parse workspace check result: %w", err)
	}
	return &result, nil
}

// RenderFile runs siba render --json on a single file
func (b *Bridge) RenderFile(path string) (*RenderResult, error) {
	stdout, _, err := b.run("render", "--json", path)
	if err != nil {
		if len(stdout) == 0 {
			return nil, fmt.Errorf("siba render failed: %w", err)
		}
	}

	var result RenderResult
	if err := json.Unmarshal(stdout, &result); err != nil {
		return nil, fmt.Errorf("parse render result: %w", err)
	}
	return &result, nil
}

func (b *Bridge) run(args ...string) ([]byte, []byte, error) {
	cmd := exec.Command(b.SibaPath, args...)
	cmd.Dir = b.WorkDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	return stdout.Bytes(), stderr.Bytes(), err
}
