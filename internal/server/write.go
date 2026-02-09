package server

import (
	"context"
	"errors"
	"strings"

	"connectrpc.com/connect"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	pb "github.com/zboralski/ida-headless-mcp/ida/worker/v1"
)

func (s *Server) setComment(ctx context.Context, req *mcp.CallToolRequest, args SetCommentRequest) (*mcp.CallToolResult, any, error) {
	const op = "set_comment"
	s.logToolInvocation(op, args.SessionID, map[string]any{"address": args.Address, "repeatable": args.Repeatable})
	sess, ok := s.registry.Get(args.SessionID)
	if !ok {
		return s.handleToolError(sessionNotFound(op, args.SessionID))
	}
	sess.Touch()
	client, err := s.workers.GetClient(sess.ID)
	if err != nil {
		return s.handleToolError(workerUnavailable(op, sess.ID, err))
	}
	resp, err := (*client.Analysis).SetComment(ctx, connect.NewRequest(&pb.SetCommentRequest{
		Address:    args.Address,
		Comment:    args.Comment,
		Repeatable: args.Repeatable,
	}))
	if err != nil {
		return s.handleToolError(idaOperationFailed(op, sess.ID, err))
	}
	if msgErr := resp.Msg.GetError(); msgErr != "" {
		return s.handleToolError(idaOperationFailed(op, sess.ID, errors.New(msgErr)))
	}
	result, _ := s.marshalJSON(map[string]any{"success": resp.Msg.GetSuccess()})
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(result)}}}, nil, nil
}

func (s *Server) setFuncComment(ctx context.Context, req *mcp.CallToolRequest, args SetFuncCommentRequest) (*mcp.CallToolResult, any, error) {
	const op = "set_func_comment"
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
	resp, err := (*client.Analysis).SetFuncComment(ctx, connect.NewRequest(&pb.SetFuncCommentRequest{
		Address: args.Address,
		Comment: args.Comment,
	}))
	if err != nil {
		return s.handleToolError(idaOperationFailed(op, sess.ID, err))
	}
	if msgErr := resp.Msg.GetError(); msgErr != "" {
		return s.handleToolError(idaOperationFailed(op, sess.ID, errors.New(msgErr)))
	}
	result, _ := s.marshalJSON(map[string]any{"success": resp.Msg.GetSuccess()})
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(result)}}}, nil, nil
}

func (s *Server) setDecompilerComment(ctx context.Context, req *mcp.CallToolRequest, args SetDecompilerCommentRequest) (*mcp.CallToolResult, any, error) {
	const op = "set_decompiler_comment"
	s.logToolInvocation(op, args.SessionID, map[string]any{"function_address": args.FunctionAddress, "address": args.Address})
	if strings.TrimSpace(args.Comment) == "" {
		return s.handleToolError(invalidInput(op, "comment is required"))
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
	resp, err := (*client.Analysis).SetDecompilerComment(ctx, connect.NewRequest(&pb.SetDecompilerCommentRequest{
		FunctionAddress: args.FunctionAddress,
		Address:         args.Address,
		Comment:         args.Comment,
	}))
	if err != nil {
		return s.handleToolError(idaOperationFailed(op, sess.ID, err))
	}
	if msgErr := resp.Msg.GetError(); msgErr != "" {
		return s.handleToolError(idaOperationFailed(op, sess.ID, errors.New(msgErr)))
	}
	result, _ := s.marshalJSON(map[string]any{"success": resp.Msg.GetSuccess()})
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(result)}}}, nil, nil
}

func (s *Server) setName(ctx context.Context, req *mcp.CallToolRequest, args SetNameRequest) (*mcp.CallToolResult, any, error) {
	const op = "set_name"
	s.logToolInvocation(op, args.SessionID, map[string]any{"address": args.Address, "name": args.Name})
	sess, ok := s.registry.Get(args.SessionID)
	if !ok {
		return s.handleToolError(sessionNotFound(op, args.SessionID))
	}
	sess.Touch()
	client, err := s.workers.GetClient(sess.ID)
	if err != nil {
		return s.handleToolError(workerUnavailable(op, sess.ID, err))
	}
	resp, err := (*client.Analysis).SetName(ctx, connect.NewRequest(&pb.SetNameRequest{
		Address: args.Address,
		Name:    args.Name,
	}))
	if err != nil {
		return s.handleToolError(idaOperationFailed(op, sess.ID, err))
	}
	if msgErr := resp.Msg.GetError(); msgErr != "" {
		return s.handleToolError(idaOperationFailed(op, sess.ID, errors.New(msgErr)))
	}
	result, _ := s.marshalJSON(map[string]any{"success": resp.Msg.GetSuccess()})
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(result)}}}, nil, nil
}

func (s *Server) deleteName(ctx context.Context, req *mcp.CallToolRequest, args DeleteNameRequest) (*mcp.CallToolResult, any, error) {
	const op = "delete_name"
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
	resp, err := (*client.Analysis).DeleteName(ctx, connect.NewRequest(&pb.DeleteNameRequest{
		Address: args.Address,
	}))
	if err != nil {
		return s.handleToolError(idaOperationFailed(op, sess.ID, err))
	}
	if msgErr := resp.Msg.GetError(); msgErr != "" {
		return s.handleToolError(idaOperationFailed(op, sess.ID, errors.New(msgErr)))
	}
	result, _ := s.marshalJSON(map[string]any{"success": resp.Msg.GetSuccess()})
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(result)}}}, nil, nil
}

func (s *Server) setLvarType(ctx context.Context, req *mcp.CallToolRequest, args SetLvarTypeRequest) (*mcp.CallToolResult, any, error) {
	const op = "set_lvar_type"
	s.logToolInvocation(op, args.SessionID, map[string]any{"function_address": args.FunctionAddress, "lvar": args.LvarName})
	if strings.TrimSpace(args.LvarType) == "" {
		return s.handleToolError(invalidInput(op, "lvar_type is required"))
	}
	if args.FunctionAddress == 0 {
		return s.handleToolError(invalidInput(op, "function_address is required"))
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
	resp, err := (*client.Analysis).SetLvarType(ctx, connect.NewRequest(&pb.SetLvarTypeRequest{
		FunctionAddress: args.FunctionAddress,
		LvarName:        args.LvarName,
		LvarType:        args.LvarType,
	}))
	if err != nil {
		return s.handleToolError(idaOperationFailed(op, sess.ID, err))
	}
	if msgErr := resp.Msg.GetError(); msgErr != "" {
		return s.handleToolError(idaOperationFailed(op, sess.ID, errors.New(msgErr)))
	}
	result, _ := s.marshalJSON(map[string]any{"success": resp.Msg.GetSuccess()})
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(result)}}}, nil, nil
}

func (s *Server) renameLvar(ctx context.Context, req *mcp.CallToolRequest, args RenameLvarRequest) (*mcp.CallToolResult, any, error) {
	const op = "rename_lvar"
	s.logToolInvocation(op, args.SessionID, map[string]any{"function_address": args.FunctionAddress, "lvar": args.LvarName})
	if strings.TrimSpace(args.NewName) == "" {
		return s.handleToolError(invalidInput(op, "new_name is required"))
	}
	if args.FunctionAddress == 0 {
		return s.handleToolError(invalidInput(op, "function_address is required"))
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
	resp, err := (*client.Analysis).RenameLvar(ctx, connect.NewRequest(&pb.RenameLvarRequest{
		FunctionAddress: args.FunctionAddress,
		LvarName:        args.LvarName,
		NewName:         args.NewName,
	}))
	if err != nil {
		return s.handleToolError(idaOperationFailed(op, sess.ID, err))
	}
	if msgErr := resp.Msg.GetError(); msgErr != "" {
		return s.handleToolError(idaOperationFailed(op, sess.ID, errors.New(msgErr)))
	}
	result, _ := s.marshalJSON(map[string]any{"success": resp.Msg.GetSuccess()})
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(result)}}}, nil, nil
}

func (s *Server) setGlobalType(ctx context.Context, req *mcp.CallToolRequest, args SetGlobalTypeRequest) (*mcp.CallToolResult, any, error) {
	const op = "set_global_type"
	s.logToolInvocation(op, args.SessionID, map[string]any{"address": args.Address})
	if strings.TrimSpace(args.Type) == "" {
		return s.handleToolError(invalidInput(op, "type is required"))
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
	resp, err := (*client.Analysis).SetGlobalType(ctx, connect.NewRequest(&pb.SetGlobalTypeRequest{Address: args.Address, Type: args.Type}))
	if err != nil {
		return s.handleToolError(idaOperationFailed(op, sess.ID, err))
	}
	if msgErr := resp.Msg.GetError(); msgErr != "" {
		return s.handleToolError(idaOperationFailed(op, sess.ID, errors.New(msgErr)))
	}
	result, _ := s.marshalJSON(map[string]any{"success": resp.Msg.GetSuccess()})
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(result)}}}, nil, nil
}

func (s *Server) renameGlobal(ctx context.Context, req *mcp.CallToolRequest, args RenameGlobalRequest) (*mcp.CallToolResult, any, error) {
	const op = "rename_global"
	s.logToolInvocation(op, args.SessionID, map[string]any{"address": args.Address})
	if strings.TrimSpace(args.NewName) == "" {
		return s.handleToolError(invalidInput(op, "new_name is required"))
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
	resp, err := (*client.Analysis).RenameGlobal(ctx, connect.NewRequest(&pb.RenameGlobalRequest{Address: args.Address, NewName: args.NewName}))
	if err != nil {
		return s.handleToolError(idaOperationFailed(op, sess.ID, err))
	}
	if msgErr := resp.Msg.GetError(); msgErr != "" {
		return s.handleToolError(idaOperationFailed(op, sess.ID, errors.New(msgErr)))
	}
	result, _ := s.marshalJSON(map[string]any{"success": resp.Msg.GetSuccess()})
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(result)}}}, nil, nil
}

func (s *Server) setFunctionType(ctx context.Context, req *mcp.CallToolRequest, args SetFunctionTypeRequest) (*mcp.CallToolResult, any, error) {
	const op = "set_function_type"
	s.logToolInvocation(op, args.SessionID, map[string]any{"address": args.Address})
	if strings.TrimSpace(args.Prototype) == "" {
		return s.handleToolError(invalidInput(op, "prototype is required"))
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
	resp, err := (*client.Analysis).SetFunctionType(ctx, connect.NewRequest(&pb.SetFunctionTypeRequest{
		Address:   args.Address,
		Prototype: args.Prototype,
	}))
	if err != nil {
		return s.handleToolError(idaOperationFailed(op, sess.ID, err))
	}
	if msgErr := resp.Msg.GetError(); msgErr != "" {
		return s.handleToolError(idaOperationFailed(op, sess.ID, errors.New(msgErr)))
	}
	result, _ := s.marshalJSON(map[string]any{"success": resp.Msg.GetSuccess()})
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(result)}}}, nil, nil
}

func (s *Server) makeFunction(ctx context.Context, req *mcp.CallToolRequest, args MakeFunctionRequest) (*mcp.CallToolResult, any, error) {
	const op = "make_function"
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
	resp, err := (*client.Analysis).MakeFunction(ctx, connect.NewRequest(&pb.MakeFunctionRequest{
		Address: args.Address,
	}))
	if err != nil {
		return s.handleToolError(idaOperationFailed(op, sess.ID, err))
	}
	if msgErr := resp.Msg.GetError(); msgErr != "" {
		return s.handleToolError(idaOperationFailed(op, sess.ID, errors.New(msgErr)))
	}

	if resp.Msg.GetSuccess() {
		s.deleteSessionCache(sess.ID)
	}
	result, _ := s.marshalJSON(map[string]any{"success": resp.Msg.GetSuccess()})
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(result)}}}, nil, nil
}
