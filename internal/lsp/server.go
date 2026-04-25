package lsp

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/greyfolk99/siba-lsp/internal/bridge"
)

// Server is the SIBA LSP server (calls siba CLI via bridge)
type Server struct {
	transport *Transport
	logger    *log.Logger

	mu        sync.Mutex
	rootURI   string
	rootPath  string
	bridge    *bridge.Bridge
	documents map[string]string // uri → current source text

	shutdown bool
}

// NewServer creates a new LSP server
func NewServer(r io.Reader, w io.Writer, logFile string) *Server {
	var logger *log.Logger
	if logFile != "" {
		f, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			logger = log.New(io.Discard, "", 0)
		} else {
			logger = log.New(f, "[siba-lsp] ", log.LstdFlags)
		}
	} else {
		logger = log.New(io.Discard, "", 0)
	}

	return &Server{
		transport: NewTransport(r, w),
		logger:    logger,
		documents: make(map[string]string),
	}
}

// Run starts the server loop
func (s *Server) Run() error {
	s.logger.Println("server started")

	for {
		msg, err := s.transport.ReadMessage()
		if err != nil {
			if err == io.EOF {
				s.logger.Println("connection closed")
				return nil
			}
			return fmt.Errorf("read error: %w", err)
		}

		if err := s.handleMessage(msg); err != nil {
			s.logger.Printf("handle error: %v", err)
		}
	}
}

func (s *Server) handleMessage(raw json.RawMessage) error {
	var req struct {
		ID     interface{}     `json:"id"`
		Method string          `json:"method"`
		Params json.RawMessage `json:"params"`
	}
	if err := json.Unmarshal(raw, &req); err != nil {
		return fmt.Errorf("unmarshal request: %w", err)
	}

	s.logger.Printf("← %s (id=%v)", req.Method, req.ID)

	switch req.Method {
	case "initialize":
		return s.handleInitialize(req.ID, req.Params)
	case "initialized":
		return s.handleInitialized()
	case "shutdown":
		return s.handleShutdown(req.ID)
	case "exit":
		os.Exit(0)
		return nil
	case "textDocument/didOpen":
		return s.handleDidOpen(req.Params)
	case "textDocument/didChange":
		return s.handleDidChange(req.Params)
	case "textDocument/didSave":
		return s.handleDidSave(req.Params)
	case "textDocument/didClose":
		return s.handleDidClose(req.Params)
	case "siba/render":
		return s.handleRender(req.ID, req.Params)
	default:
		if req.ID != nil {
			return s.sendResponse(req.ID, nil, &RespError{
				Code:    -32601,
				Message: fmt.Sprintf("method not found: %s", req.Method),
			})
		}
		return nil
	}
}

// --- Handlers ---

func (s *Server) handleInitialize(id interface{}, params json.RawMessage) error {
	var p InitializeParams
	if err := json.Unmarshal(params, &p); err != nil {
		return err
	}

	root := uriToPath(p.RootURI)

	s.mu.Lock()
	s.rootURI = p.RootURI
	s.rootPath = root
	s.bridge = bridge.New(root)
	s.mu.Unlock()

	s.logger.Printf("root: %s", root)

	return s.sendResponse(id, InitializeResult{
		Capabilities: ServerCapabilities{
			TextDocumentSync: TextDocumentSyncOptions{
				OpenClose: true,
				Change:    1, // Full sync
				Save:      &SaveOptions{IncludeText: true},
			},
		},
		ServerInfo: ServerInfo{
			Name:    "siba-lsp",
			Version: "0.2.0",
		},
	}, nil)
}

func (s *Server) handleInitialized() error {
	// run initial workspace check
	s.mu.Lock()
	b := s.bridge
	s.mu.Unlock()

	if b == nil {
		return nil
	}

	result, err := b.CheckWorkspace()
	if err != nil {
		s.logger.Printf("initial workspace check failed: %v", err)
		return nil
	}

	s.logger.Printf("workspace loaded: %d documents", result.Documents)

	// publish diagnostics for each file
	for _, f := range result.Files {
		uri := pathToURI(filepath.Join(s.rootPath, f.File))
		diags := convertBridgeDiagnostics(f.Diagnostics)
		s.publishDiagnostics(uri, diags)
	}

	return nil
}

func (s *Server) handleShutdown(id interface{}) error {
	s.mu.Lock()
	s.shutdown = true
	s.mu.Unlock()
	return s.sendResponse(id, nil, nil)
}

func (s *Server) handleDidOpen(params json.RawMessage) error {
	var p DidOpenTextDocumentParams
	if err := json.Unmarshal(params, &p); err != nil {
		return err
	}

	s.mu.Lock()
	s.documents[p.TextDocument.URI] = p.TextDocument.Text
	s.mu.Unlock()

	s.logger.Printf("opened: %s", p.TextDocument.URI)
	return s.checkAndPublish(p.TextDocument.URI)
}

func (s *Server) handleDidChange(params json.RawMessage) error {
	var p DidChangeTextDocumentParams
	if err := json.Unmarshal(params, &p); err != nil {
		return err
	}

	if len(p.ContentChanges) == 0 {
		return nil
	}

	text := p.ContentChanges[len(p.ContentChanges)-1].Text
	s.mu.Lock()
	s.documents[p.TextDocument.URI] = text
	s.mu.Unlock()

	return s.checkAndPublish(p.TextDocument.URI)
}

func (s *Server) handleDidSave(params json.RawMessage) error {
	var p DidSaveTextDocumentParams
	if err := json.Unmarshal(params, &p); err != nil {
		return err
	}

	if p.Text != nil {
		s.mu.Lock()
		s.documents[p.TextDocument.URI] = *p.Text
		s.mu.Unlock()
	}

	return s.checkAndPublish(p.TextDocument.URI)
}

func (s *Server) handleDidClose(params json.RawMessage) error {
	var p DidCloseTextDocumentParams
	if err := json.Unmarshal(params, &p); err != nil {
		return err
	}

	s.mu.Lock()
	delete(s.documents, p.TextDocument.URI)
	s.mu.Unlock()

	s.logger.Printf("closed: %s", p.TextDocument.URI)
	return s.publishDiagnostics(p.TextDocument.URI, nil)
}

func (s *Server) handleRender(id interface{}, params json.RawMessage) error {
	var p RenderParams
	if err := json.Unmarshal(params, &p); err != nil {
		return s.sendResponse(id, nil, &RespError{Code: -32602, Message: "invalid params"})
	}

	path := uriToPath(p.URI)
	relPath := s.relativePath(path)

	s.mu.Lock()
	b := s.bridge
	s.mu.Unlock()

	if b == nil {
		return s.sendResponse(id, RenderResult{Error: "no workspace"}, nil)
	}

	result, err := b.RenderFile(relPath)
	if err != nil {
		return s.sendResponse(id, RenderResult{Error: err.Error()}, nil)
	}

	if result.Error != "" {
		return s.sendResponse(id, RenderResult{Error: result.Error}, nil)
	}

	return s.sendResponse(id, RenderResult{Content: result.Content}, nil)
}

// --- Check + Publish ---

func (s *Server) checkAndPublish(uri string) error {
	path := uriToPath(uri)
	relPath := s.relativePath(path)

	s.mu.Lock()
	b := s.bridge
	s.mu.Unlock()

	if b == nil {
		return nil
	}

	result, err := b.CheckFile(relPath)
	if err != nil {
		s.logger.Printf("check failed for %s: %v", relPath, err)
		return s.publishDiagnostics(uri, nil)
	}

	diags := convertBridgeDiagnostics(result.Diagnostics)
	return s.publishDiagnostics(uri, diags)
}

func (s *Server) publishDiagnostics(uri string, diags []Diagnostic) error {
	if diags == nil {
		diags = []Diagnostic{}
	}
	return s.sendNotification("textDocument/publishDiagnostics", PublishDiagnosticsParams{
		URI:         uri,
		Diagnostics: diags,
	})
}

func convertBridgeDiagnostics(bdiags []bridge.Diagnostic) []Diagnostic {
	seen := make(map[string]bool)
	var result []Diagnostic

	for _, d := range bdiags {
		key := fmt.Sprintf("%s:%d:%s", d.Code, d.Line, d.Message)
		if seen[key] {
			continue
		}
		seen[key] = true

		severity := 1
		switch d.Severity {
		case "warning":
			severity = 2
		case "info":
			severity = 3
		case "hint":
			severity = 4
		}

		// bridge returns 1-based lines; LSP uses 0-based
		startLine := d.Line
		if startLine > 0 {
			startLine--
		}
		endLine := d.EndLine
		if endLine > 0 {
			endLine--
		}

		result = append(result, Diagnostic{
			Range: Range{
				Start: Position{Line: startLine, Character: d.Column},
				End:   Position{Line: endLine, Character: d.EndCol},
			},
			Severity: severity,
			Code:     d.Code,
			Source:   "siba",
			Message:  d.Message,
		})
	}
	return result
}

// --- Transport helpers ---

func (s *Server) sendResponse(id interface{}, result interface{}, respErr *RespError) error {
	resp := Response{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
		Error:   respErr,
	}
	s.logger.Printf("→ response (id=%v)", id)
	return s.transport.WriteMessage(resp)
}

func (s *Server) sendNotification(method string, params interface{}) error {
	notif := Notification{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
	}
	s.logger.Printf("→ %s", method)
	return s.transport.WriteMessage(notif)
}

// --- Utilities ---

func uriToPath(uri string) string {
	if strings.HasPrefix(uri, "file://") {
		u, err := url.Parse(uri)
		if err != nil {
			return ""
		}
		return u.Path
	}
	return uri
}

func pathToURI(path string) string {
	absPath, err := filepath.Abs(path)
	if err != nil {
		absPath = path
	}
	return "file://" + absPath
}

func (s *Server) relativePath(absPath string) string {
	s.mu.Lock()
	root := s.rootPath
	s.mu.Unlock()

	if root == "" {
		return filepath.Base(absPath)
	}

	rel, err := filepath.Rel(root, absPath)
	if err != nil {
		return filepath.Base(absPath)
	}
	return rel
}
