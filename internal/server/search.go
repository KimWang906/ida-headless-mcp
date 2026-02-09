package server

import (
	"context"
	"errors"
	"strings"

	"connectrpc.com/connect"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	pb "github.com/zboralski/ida-headless-mcp/ida/worker/v1"
)

func (s *Server) dataReadString(ctx context.Context, req *mcp.CallToolRequest, args DataReadStringRequest) (*mcp.CallToolResult, any, error) {
	const op = "data_read_string"
	s.logToolInvocation(op, args.SessionID, map[string]any{"address": args.Address, "max_length": args.MaxLength})
	sess, ok := s.registry.Get(args.SessionID)
	if !ok {
		return s.handleToolError(sessionNotFound(op, args.SessionID))
	}
	sess.Touch()
	client, err := s.workers.GetClient(sess.ID)
	if err != nil {
		return s.handleToolError(workerUnavailable(op, sess.ID, err))
	}
	maxLen := args.MaxLength
	if maxLen <= 0 {
		maxLen = 256
	}
	resp, err := (*client.Analysis).DataReadString(ctx, connect.NewRequest(&pb.DataReadStringRequest{Address: args.Address, MaxLength: uint32(maxLen)}))
	if err != nil {
		return s.handleToolError(idaOperationFailed(op, sess.ID, err))
	}
	if msgErr := resp.Msg.GetError(); msgErr != "" {
		return s.handleToolError(idaOperationFailed(op, sess.ID, errors.New(msgErr)))
	}
	result, _ := s.marshalJSON(map[string]any{"value": resp.Msg.GetValue()})
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(result)}}}, nil, nil
}

func (s *Server) dataReadByte(ctx context.Context, req *mcp.CallToolRequest, args DataReadByteRequest) (*mcp.CallToolResult, any, error) {
	const op = "data_read_byte"
	s.logToolInvocation(op, args.SessionID, map[string]any{"address": args.Address})
	sess, ok := s.registry.Get(args.SessionID)
	if !ok {
		return s.handleToolError(sessionNotFound(op, args.SessionID))
	}
	sess.Touch()
	client, err := s.workers.GetClient(sess.ID)
	if err != nil {
		return s.handleToolError(workerUnavailable(op, sess.ID, err))
	}
	resp, err := (*client.Analysis).DataReadByte(ctx, connect.NewRequest(&pb.DataReadByteRequest{Address: args.Address}))
	if err != nil {
		return s.handleToolError(idaOperationFailed(op, sess.ID, err))
	}
	if msgErr := resp.Msg.GetError(); msgErr != "" {
		return s.handleToolError(idaOperationFailed(op, sess.ID, errors.New(msgErr)))
	}
	result, _ := s.marshalJSON(map[string]any{"value": resp.Msg.GetValue()})
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(result)}}}, nil, nil
}

func (s *Server) findBinary(ctx context.Context, req *mcp.CallToolRequest, args FindBinaryRequest) (*mcp.CallToolResult, any, error) {
	const op = "find_binary"
	s.logToolInvocation(op, args.SessionID, map[string]any{"pattern": args.Pattern})
	if strings.TrimSpace(args.Pattern) == "" {
		return s.handleToolError(invalidInput(op, "pattern is required"))
	}
	sess, ok := s.registry.Get(args.SessionID)
	if !ok {
		return s.handleToolError(sessionNotFound(op, args.SessionID))
	}
	sess.Touch()
	client, err := s.workers.GetClient(sess.ID)
	if err != nil {
		return s.handleToolError(workerUnavailable(op, sess.ID, err))
	}
	resp, err := (*client.Analysis).FindBinary(ctx, connect.NewRequest(&pb.FindBinaryRequest{
		Start:    args.Start,
		End:      args.End,
		Pattern:  args.Pattern,
		SearchUp: args.SearchUp,
	}))
	if err != nil {
		return s.handleToolError(idaOperationFailed(op, sess.ID, err))
	}
	if msgErr := resp.Msg.GetError(); msgErr != "" {
		return s.handleToolError(idaOperationFailed(op, sess.ID, errors.New(msgErr)))
	}
	result, _ := s.marshalJSON(map[string]any{"addresses": resp.Msg.GetAddresses()})
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(result)}}}, nil, nil
}

func (s *Server) findText(ctx context.Context, req *mcp.CallToolRequest, args FindTextRequest) (*mcp.CallToolResult, any, error) {
	const op = "find_text"
	s.logToolInvocation(op, args.SessionID, map[string]any{"needle": args.Needle})
	if strings.TrimSpace(args.Needle) == "" {
		return s.handleToolError(invalidInput(op, "needle is required"))
	}
	sess, ok := s.registry.Get(args.SessionID)
	if !ok {
		return s.handleToolError(sessionNotFound(op, args.SessionID))
	}
	sess.Touch()
	client, err := s.workers.GetClient(sess.ID)
	if err != nil {
		return s.handleToolError(workerUnavailable(op, sess.ID, err))
	}
	resp, err := (*client.Analysis).FindText(ctx, connect.NewRequest(&pb.FindTextRequest{
		Start:         args.Start,
		End:           args.End,
		Needle:        args.Needle,
		CaseSensitive: args.CaseSensitive,
		Unicode:       args.Unicode,
	}))
	if err != nil {
		return s.handleToolError(idaOperationFailed(op, sess.ID, err))
	}
	if msgErr := resp.Msg.GetError(); msgErr != "" {
		return s.handleToolError(idaOperationFailed(op, sess.ID, errors.New(msgErr)))
	}
	result, _ := s.marshalJSON(map[string]any{"addresses": resp.Msg.GetAddresses()})
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(result)}}}, nil, nil
}
