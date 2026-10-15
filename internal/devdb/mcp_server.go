package devdb

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"sync"
)

// ServerName is the MCP server name; its tools surface as mcp__devdb__<tool>.
const ServerName = "devdb"

type rpcRequest struct {
	ID     json.RawMessage `json:"id,omitempty"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

// Server is a minimal stdio MCP server (newline-delimited JSON-RPC 2.0) over
// one Engine. The connection is opened on the first tool call, so a database
// that is down only fails those calls, never the agent run.
type Server struct {
	tools []Tool
	open  func(context.Context) (Engine, error)

	mu     sync.Mutex
	engine Engine
}

// NewServer serves the database behind url; kind picks the advertised tools.
func NewServer(rawURL string) (*Server, error) {
	kind, err := Kind(rawURL)
	if err != nil {
		return nil, err
	}
	var e Engine = &postgres{}
	if kind == "mongodb" {
		e = &mongoDB{}
	}
	return &Server{tools: e.Tools(), open: func(ctx context.Context) (Engine, error) { return Open(ctx, rawURL) }}, nil
}

// Serve answers requests from in until it is closed.
func (s *Server) Serve(ctx context.Context, in io.Reader, out io.Writer) error {
	defer func() {
		if s.engine != nil {
			s.engine.Close()
		}
	}()
	sc := bufio.NewScanner(in)
	sc.Buffer(make([]byte, 1<<20), 16<<20)
	enc := json.NewEncoder(out)
	for sc.Scan() {
		var req rpcRequest
		if err := json.Unmarshal(sc.Bytes(), &req); err != nil {
			_ = enc.Encode(rpcResponse{JSONRPC: "2.0", ID: json.RawMessage("null"), Error: &rpcError{-32700, "parse error"}})
			continue
		}
		if len(req.ID) == 0 {
			continue // notification (e.g. notifications/initialized)
		}
		result, rerr := s.handle(ctx, req)
		if err := enc.Encode(rpcResponse{JSONRPC: "2.0", ID: req.ID, Result: result, Error: rerr}); err != nil {
			return err
		}
	}
	return sc.Err()
}

func (s *Server) handle(ctx context.Context, req rpcRequest) (any, *rpcError) {
	switch req.Method {
	case "initialize":
		var p struct {
			ProtocolVersion string `json:"protocolVersion"`
		}
		_ = json.Unmarshal(req.Params, &p)
		if p.ProtocolVersion == "" {
			p.ProtocolVersion = "2024-11-05"
		}
		return map[string]any{
			"protocolVersion": p.ProtocolVersion,
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      map[string]any{"name": ServerName, "version": "1"},
		}, nil
	case "ping":
		return map[string]any{}, nil
	case "tools/list":
		return map[string]any{"tools": s.tools}, nil
	case "tools/call":
		var p struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &p); err != nil {
			return nil, &rpcError{-32602, "invalid params"}
		}
		if len(p.Arguments) == 0 {
			p.Arguments = json.RawMessage("{}")
		}
		text, err := s.call(ctx, p.Name, p.Arguments)
		if err != nil {
			return toolResult("error: "+err.Error(), true), nil
		}
		return toolResult(text, false), nil
	}
	return nil, &rpcError{-32601, "method not found: " + req.Method}
}

func (s *Server) call(ctx context.Context, tool string, args json.RawMessage) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.engine == nil {
		cctx, cancel := context.WithTimeout(ctx, queryTimeout)
		e, err := s.open(cctx)
		cancel()
		if err != nil {
			return "", err
		}
		s.engine = e
	}
	return s.engine.Call(ctx, tool, args)
}

func toolResult(text string, isErr bool) map[string]any {
	return map[string]any{"content": []map[string]any{{"type": "text", "text": text}}, "isError": isErr}
}
