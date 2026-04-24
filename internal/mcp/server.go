package mcp

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"sync"

	"github.com/hjseo/siba-lsp/internal/bridge"
	"github.com/hjseo/siba-lsp/internal/lsp"
)

// Server is the SIBA MCP server. Exposes siba CLI tools via MCP protocol.
type Server struct {
	transport *lsp.Transport // reuse LSP transport (same JSON-RPC framing)
	logger    *log.Logger
	bridge    *bridge.Bridge

	mu       sync.Mutex
	workDir  string
	shutdown bool
}

// NewServer creates a new MCP server
func NewServer(r io.Reader, w io.Writer, logFile string, workDir string) *Server {
	var logger *log.Logger
	if logFile != "" {
		f, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			logger = log.New(io.Discard, "", 0)
		} else {
			logger = log.New(f, "[siba-mcp] ", log.LstdFlags)
		}
	} else {
		logger = log.New(io.Discard, "", 0)
	}

	b := bridge.New(workDir)

	return &Server{
		transport: lsp.NewTransport(r, w),
		logger:    logger,
		bridge:    b,
		workDir:   workDir,
	}
}

// Run starts the MCP server loop
func (s *Server) Run() error {
	s.logger.Println("MCP server started")

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
		return fmt.Errorf("unmarshal: %w", err)
	}

	s.logger.Printf("← %s (id=%v)", req.Method, req.ID)

	switch req.Method {
	case "initialize":
		return s.handleInitialize(req.ID, req.Params)
	case "initialized":
		return nil
	case "tools/list":
		return s.handleToolsList(req.ID)
	case "tools/call":
		return s.handleToolsCall(req.ID, req.Params)
	case "ping":
		return s.sendResponse(req.ID, map[string]interface{}{}, nil)
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

func (s *Server) handleInitialize(id interface{}, params json.RawMessage) error {
	var p InitializeParams
	if err := json.Unmarshal(params, &p); err != nil {
		// non-fatal, use defaults
		s.logger.Printf("initialize params parse error (non-fatal): %v", err)
	}

	s.logger.Printf("client: %s %s", p.ClientInfo.Name, p.ClientInfo.Version)

	return s.sendResponse(id, InitializeResult{
		ProtocolVersion: "2024-11-05",
		Capabilities: ServerCaps{
			Tools: &ToolsCap{ListChanged: false},
		},
		ServerInfo: Implementation{
			Name:    "siba-mcp",
			Version: "0.1.0",
		},
	}, nil)
}

func (s *Server) handleToolsList(id interface{}) error {
	tools := []Tool{
		{
			Name:        "siba_check",
			Description: "Check a SIBA document or workspace for errors. Returns diagnostics as JSON.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"file": {
						Type:        "string",
						Description: "Path to a .md file to check. Omit for workspace-wide check.",
					},
				},
			},
		},
		{
			Name:        "siba_cat",
			Description: "Render a SIBA document (streaming). Returns clean markdown with all directives processed.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"file": {
						Type:        "string",
						Description: "Path to the .md file to render. Use file.md#section for specific section.",
					},
				},
				Required: []string{"file"},
			},
		},
		{
			Name:        "siba_ls",
			Description: "List all documents and templates in the workspace, or symbols in a specific file.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"file": {
						Type:        "string",
						Description: "Path to a .md file. Omit for workspace-wide listing.",
					},
				},
			},
		},
		{
			Name:        "siba_tree",
			Description: "Show heading tree for a file, or dependency tree for the workspace (--deps).",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"file": {
						Type:        "string",
						Description: "Path to a .md file. Omit for workspace overview.",
					},
				},
			},
		},
		{
			Name:        "siba_find",
			Description: "Search the workspace for a keyword in document content.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"query": {
						Type:        "string",
						Description: "Search query.",
					},
				},
				Required: []string{"query"},
			},
		},
		{
			Name:        "siba_help",
			Description: "Show SIBA syntax reference and documentation.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"topic": {
						Type:        "string",
						Description: "Help topic: directives, variables, templates, references, control, packages, types. Omit for overview.",
					},
				},
			},
		},
	}

	return s.sendResponse(id, ToolsListResult{Tools: tools}, nil)
}

func (s *Server) handleToolsCall(id interface{}, params json.RawMessage) error {
	var p ToolCallParams
	if err := json.Unmarshal(params, &p); err != nil {
		return s.sendResponse(id, ToolCallResult{
			Content: []ContentBlock{{Type: "text", Text: "invalid params"}},
			IsError: true,
		}, nil)
	}

	s.logger.Printf("tool call: %s %v", p.Name, p.Arguments)

	switch p.Name {
	case "siba_check":
		return s.toolCheck(id, p.Arguments)
	case "siba_cat":
		return s.toolCat(id, p.Arguments)
	case "siba_ls":
		return s.toolLs(id, p.Arguments)
	case "siba_tree":
		return s.toolTree(id, p.Arguments)
	case "siba_find":
		return s.toolFind(id, p.Arguments)
	case "siba_help":
		return s.toolHelp(id, p.Arguments)
	default:
		return s.sendResponse(id, ToolCallResult{
			Content: []ContentBlock{{Type: "text", Text: fmt.Sprintf("unknown tool: %s", p.Name)}},
			IsError: true,
		}, nil)
	}
}

// --- Tool implementations ---

func (s *Server) toolCheck(id interface{}, args map[string]interface{}) error {
	file, _ := args["file"].(string)

	var resultJSON []byte
	var err error

	if file != "" {
		result, e := s.bridge.CheckFile(file)
		if e != nil {
			return s.textResult(id, fmt.Sprintf("check failed: %v", e), true)
		}
		resultJSON, err = json.MarshalIndent(result, "", "  ")
	} else {
		result, e := s.bridge.CheckWorkspace()
		if e != nil {
			return s.textResult(id, fmt.Sprintf("workspace check failed: %v", e), true)
		}
		resultJSON, err = json.MarshalIndent(result, "", "  ")
	}

	if err != nil {
		return s.textResult(id, fmt.Sprintf("json error: %v", err), true)
	}

	return s.textResult(id, string(resultJSON), false)
}

func (s *Server) toolCat(id interface{}, args map[string]interface{}) error {
	file, ok := args["file"].(string)
	if !ok || file == "" {
		return s.textResult(id, "file parameter is required", true)
	}

	result, err := s.bridge.RenderFile(file)
	if err != nil {
		return s.textResult(id, fmt.Sprintf("render failed: %v", err), true)
	}

	if result.Error != "" {
		return s.textResult(id, fmt.Sprintf("render error: %s", result.Error), true)
	}

	return s.textResult(id, result.Content, false)
}

func (s *Server) toolLs(id interface{}, args map[string]interface{}) error {
	data, err := s.bridge.Ls()
	if err != nil {
		return s.textResult(id, fmt.Sprintf("ls failed: %v", err), true)
	}
	return s.textResult(id, string(data), false)
}

func (s *Server) toolTree(id interface{}, args map[string]interface{}) error {
	file, _ := args["file"].(string)
	data, err := s.bridge.Tree(file)
	if err != nil {
		return s.textResult(id, fmt.Sprintf("tree failed: %v", err), true)
	}
	return s.textResult(id, string(data), false)
}

func (s *Server) toolFind(id interface{}, args map[string]interface{}) error {
	query, ok := args["query"].(string)
	if !ok || query == "" {
		return s.textResult(id, "query parameter is required", true)
	}
	data, err := s.bridge.Find(query)
	if err != nil {
		return s.textResult(id, fmt.Sprintf("find failed: %v", err), true)
	}
	return s.textResult(id, string(data), false)
}

func (s *Server) toolHelp(id interface{}, args map[string]interface{}) error {
	topic, _ := args["topic"].(string)
	topic = strings.ToLower(strings.TrimSpace(topic))

	text := helpText(topic)
	return s.textResult(id, text, false)
}

// --- Helpers ---

func (s *Server) textResult(id interface{}, text string, isError bool) error {
	return s.sendResponse(id, ToolCallResult{
		Content: []ContentBlock{{Type: "text", Text: text}},
		IsError: isError,
	}, nil)
}

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
