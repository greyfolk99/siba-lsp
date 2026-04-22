// Package mcp implements a minimal Model Context Protocol server for SIBA.
// MCP uses JSON-RPC 2.0 over stdio, similar to LSP but with a different
// capability negotiation and tool-based interaction model.
package mcp

// --- JSON-RPC 2.0 ---

type Request struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

type Response struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *RespError  `json:"error,omitempty"`
}

type RespError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type Notification struct {
	JSONRPC string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// --- Initialize ---

type InitializeParams struct {
	ProtocolVersion string           `json:"protocolVersion"`
	Capabilities    ClientCaps       `json:"capabilities"`
	ClientInfo      Implementation   `json:"clientInfo"`
}

type ClientCaps struct {
	Roots    *RootsCap    `json:"roots,omitempty"`
	Sampling *SamplingCap `json:"sampling,omitempty"`
}

type RootsCap struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

type SamplingCap struct{}

type Implementation struct {
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
}

type InitializeResult struct {
	ProtocolVersion string         `json:"protocolVersion"`
	Capabilities    ServerCaps     `json:"capabilities"`
	ServerInfo      Implementation `json:"serverInfo"`
}

type ServerCaps struct {
	Tools *ToolsCap `json:"tools,omitempty"`
}

type ToolsCap struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

// --- Tools ---

type ToolsListResult struct {
	Tools []Tool `json:"tools"`
}

type Tool struct {
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	InputSchema InputSchema `json:"inputSchema"`
}

type InputSchema struct {
	Type       string                 `json:"type"`
	Properties map[string]Property    `json:"properties,omitempty"`
	Required   []string               `json:"required,omitempty"`
}

type Property struct {
	Type        string `json:"type"`
	Description string `json:"description,omitempty"`
}

type ToolCallParams struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments,omitempty"`
}

type ToolCallResult struct {
	Content []ContentBlock `json:"content"`
	IsError bool           `json:"isError,omitempty"`
}

type ContentBlock struct {
	Type string `json:"type"` // "text"
	Text string `json:"text"`
}
