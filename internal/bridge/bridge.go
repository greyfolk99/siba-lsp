// Package bridge calls the siba CLI as a subprocess and parses JSON output.
package bridge

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
)

// JSONEnvelope is the common JSON output wrapper from siba CLI
type JSONEnvelope struct {
	OK     bool            `json:"ok"`
	Data   json.RawMessage `json:"data"`
	Errors json.RawMessage `json:"errors"`
}

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
	Root        string        `json:"root"`
	Version     string        `json:"version"`
	Documents   int           `json:"documents"`
	Templates   int           `json:"templates"`
	TotalErrors int           `json:"total_errors"`
	TotalWarns  int           `json:"total_warnings"`
	Files       []CheckResult `json:"files"`
	Workspace   []Diagnostic  `json:"workspace_diagnostics"`
}

// RenderResult from siba cat
type RenderResult struct {
	Content string
	Error   string
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

// unwrapEnvelope extracts data from JSON envelope {ok, data, errors}
func unwrapEnvelope(stdout []byte) (json.RawMessage, error) {
	var env JSONEnvelope
	if err := json.Unmarshal(stdout, &env); err != nil {
		return nil, fmt.Errorf("parse envelope: %w", err)
	}
	return env.Data, nil
}

// CheckFile runs siba check --json on a single file
func (b *Bridge) CheckFile(path string) (*CheckResult, error) {
	stdout, _, err := b.run("check", "--json", path)
	if err != nil {
		if len(stdout) == 0 {
			return nil, fmt.Errorf("siba check failed: %w", err)
		}
	}

	data, err := unwrapEnvelope(stdout)
	if err != nil {
		return nil, err
	}

	var result CheckResult
	if err := json.Unmarshal(data, &result); err != nil {
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

	data, err := unwrapEnvelope(stdout)
	if err != nil {
		return nil, err
	}

	var result CheckWorkspaceResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parse workspace check result: %w", err)
	}
	return &result, nil
}

// RenderFile runs siba cat on a single file (streaming render to stdout)
func (b *Bridge) RenderFile(path string) (*RenderResult, error) {
	stdout, stderr, err := b.run("cat", path)
	if err != nil {
		errMsg := string(stderr)
		if errMsg == "" {
			errMsg = err.Error()
		}
		return &RenderResult{Error: errMsg}, nil
	}

	return &RenderResult{Content: string(stdout)}, nil
}

// Ls runs siba ls --json
func (b *Bridge) Ls() (json.RawMessage, error) {
	stdout, _, err := b.run("ls", "--json")
	if err != nil {
		if len(stdout) == 0 {
			return nil, fmt.Errorf("siba ls failed: %w", err)
		}
	}
	return unwrapEnvelope(stdout)
}

// Tree runs siba tree --json [file]
func (b *Bridge) Tree(file string) (json.RawMessage, error) {
	args := []string{"tree", "--json"}
	if file != "" {
		args = append(args, file)
	}
	stdout, _, err := b.run(args...)
	if err != nil {
		if len(stdout) == 0 {
			return nil, fmt.Errorf("siba tree failed: %w", err)
		}
	}
	return unwrapEnvelope(stdout)
}

// Find runs siba find --json <query>
func (b *Bridge) Find(query string) (json.RawMessage, error) {
	stdout, _, err := b.run("find", "--json", query)
	if err != nil {
		if len(stdout) == 0 {
			return nil, fmt.Errorf("siba find failed: %w", err)
		}
	}
	return unwrapEnvelope(stdout)
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
