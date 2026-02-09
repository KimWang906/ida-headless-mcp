package server

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	pb "github.com/zboralski/ida-headless-mcp/ida/worker/v1"
)

func (s *Server) getBytes(ctx context.Context, req *mcp.CallToolRequest, args GetBytesRequest) (*mcp.CallToolResult, any, error) {
	const op = "get_bytes"
	sess, ok := s.registry.Get(args.SessionID)
	if !ok {
		return s.handleToolError(sessionNotFound(op, args.SessionID))
	}
	sess.Touch()
	client, err := s.workers.GetClient(sess.ID)
	if err != nil {
		return s.handleToolError(workerUnavailable(op, sess.ID, err))
	}
	resp, err := (*client.Analysis).GetBytes(ctx, connect.NewRequest(&pb.GetBytesRequest{
		Address: args.Address,
		Size:    args.Size,
	}))
	if err != nil {
		return s.handleToolError(idaOperationFailed(op, sess.ID, err))
	}
	if resp.Msg.Error != "" {
		return s.handleToolError(idaOperationFailed(op, sess.ID, errors.New(resp.Msg.Error)))
	}
	result, _ := s.marshalJSON(map[string]interface{}{
		"data": resp.Msg.Data,
	})
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: string(result)},
		},
	}, nil, nil
}

func (s *Server) getDisasm(ctx context.Context, req *mcp.CallToolRequest, args GetDisasmRequest) (*mcp.CallToolResult, any, error) {
	const op = "get_disasm"
	sess, ok := s.registry.Get(args.SessionID)
	if !ok {
		return s.handleToolError(sessionNotFound(op, args.SessionID))
	}
	sess.Touch()
	client, err := s.workers.GetClient(sess.ID)
	if err != nil {
		return s.handleToolError(workerUnavailable(op, sess.ID, err))
	}
	resp, err := (*client.Analysis).GetDisasm(ctx, connect.NewRequest(&pb.GetDisasmRequest{
		Address: args.Address,
	}))
	if err != nil {
		return s.handleToolError(idaOperationFailed(op, sess.ID, err))
	}
	if resp.Msg.Error != "" {
		return s.handleToolError(idaOperationFailed(op, sess.ID, errors.New(resp.Msg.Error)))
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: resp.Msg.Disasm},
		},
	}, nil, nil
}

func (s *Server) getFunctionDisasm(ctx context.Context, req *mcp.CallToolRequest, args GetFunctionDisasmRequest) (*mcp.CallToolResult, any, error) {
	const op = "get_function_disasm"
	sess, ok := s.registry.Get(args.SessionID)
	if !ok {
		return s.handleToolError(sessionNotFound(op, args.SessionID))
	}
	sess.Touch()
	client, err := s.workers.GetClient(sess.ID)
	if err != nil {
		return s.handleToolError(workerUnavailable(op, sess.ID, err))
	}
	resp, err := (*client.Analysis).GetFunctionDisasm(ctx, connect.NewRequest(&pb.GetFunctionDisasmRequest{
		Address: args.Address,
	}))
	if err != nil {
		return s.handleToolError(idaOperationFailed(op, sess.ID, err))
	}
	if msgErr := resp.Msg.GetError(); msgErr != "" {
		return s.handleToolError(idaOperationFailed(op, sess.ID, errors.New(msgErr)))
	}
	payload, _ := s.marshalJSON(map[string]any{"disassembly": resp.Msg.GetDisassembly()})
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(payload)}}}, nil, nil
}

func (s *Server) getDecompiled(ctx context.Context, req *mcp.CallToolRequest, args GetDecompiledRequest) (*mcp.CallToolResult, any, error) {
	const op = "get_decompiled"
	sess, ok := s.registry.Get(args.SessionID)
	if !ok {
		return s.handleToolError(sessionNotFound(op, args.SessionID))
	}
	sess.Touch()
	client, err := s.workers.GetClient(sess.ID)
	if err != nil {
		return s.handleToolError(workerUnavailable(op, sess.ID, err))
	}
	resp, err := (*client.Analysis).GetDecompiled(ctx, connect.NewRequest(&pb.GetDecompiledRequest{
		Address: args.Address,
	}))
	if err != nil {
		return s.handleToolError(idaOperationFailed(op, sess.ID, err))
	}
	if resp.Msg.Error != "" {
		return s.handleToolError(idaOperationFailed(op, sess.ID, errors.New(resp.Msg.Error)))
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: resp.Msg.Code},
		},
	}, nil, nil
}

func (s *Server) getFunctions(ctx context.Context, req *mcp.CallToolRequest, args GetFunctionsRequest) (*mcp.CallToolResult, any, error) {
	const op = "get_functions"
	s.logToolInvocation(op, args.SessionID, map[string]interface{}{
		"offset": args.Offset,
		"limit":  args.Limit,
		"regex":  args.Regex,
	})
	sess, ok := s.registry.Get(args.SessionID)
	if !ok {
		return s.handleToolError(sessionNotFound(op, args.SessionID))
	}
	sess.Touch()
	client, err := s.workers.GetClient(sess.ID)
	if err != nil {
		return s.handleToolError(workerUnavailable(op, sess.ID, err))
	}

	progress := s.progressReporter(ctx, req, sess.ID, op)
	cache := s.getSessionCache(sess.ID)
	functionsData, hit, err := cache.loadFunctions(sess.ID, s.logger, func() ([]*pb.Function, error) {
		return s.fetchAllFunctions(ctx, client, progress)
	})
	if err != nil {
		return s.handleToolError(idaOperationFailed(op, sess.ID, err))
	}
	if hit {
		s.emitProgress(progress, sess.ID, op, "Functions served from cache", 1, 1)
	}

	filtered := functionsData
	if args.Regex != "" {
		regex, err := compileRegex(args.Regex, args.CaseSens)
		if err != nil {
			return s.handleToolError(invalidInput(op, err.Error()))
		}
		tmp := make([]*pb.Function, 0, len(filtered))
		for _, fn := range filtered {
			if regex.MatchString(fn.Name) {
				tmp = append(tmp, fn)
			}
		}
		filtered = tmp
	}

	totalFunctions := len(filtered)
	offset, limit, err := normalizePagination(args.Offset, args.Limit)
	if err != nil {
		return s.handleToolError(invalidInput(op, err.Error()))
	}
	if offset > totalFunctions {
		offset = totalFunctions
	}
	end := offset + limit
	if end > totalFunctions {
		end = totalFunctions
	}

	functions := mapFunctionItems(filtered[offset:end])
	result, _ := s.marshalJSON(map[string]interface{}{
		"functions": functions,
		"total":     totalFunctions,
		"offset":    offset,
		"count":     len(functions),
		"limit":     limit,
		"regex":     args.Regex,
	})
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: string(result)},
		},
	}, nil, nil
}

func (s *Server) getImports(ctx context.Context, req *mcp.CallToolRequest, args GetImportsRequest) (*mcp.CallToolResult, any, error) {
	const op = "get_imports"
	s.logToolInvocation(op, args.SessionID, map[string]interface{}{
		"offset": args.Offset,
		"limit":  args.Limit,
		"module": args.Module,
		"regex":  args.Regex,
	})
	sess, ok := s.registry.Get(args.SessionID)
	if !ok {
		return s.handleToolError(sessionNotFound(op, args.SessionID))
	}
	sess.Touch()
	client, err := s.workers.GetClient(sess.ID)
	if err != nil {
		return s.handleToolError(workerUnavailable(op, sess.ID, err))
	}

	progress := s.progressReporter(ctx, req, sess.ID, op)
	cache := s.getSessionCache(sess.ID)
	importsData, hit, err := cache.loadImports(sess.ID, s.logger, func() ([]*pb.Import, error) {
		return s.fetchAllImports(ctx, client, progress)
	})
	if err != nil {
		return s.handleToolError(idaOperationFailed(op, sess.ID, err))
	}
	if hit {
		s.emitProgress(progress, sess.ID, op, "Imports served from cache", 1, 1)
	}

	filtered := importsData
	if args.Module != "" {
		tmp := make([]*pb.Import, 0, len(filtered))
		for _, imp := range filtered {
			if matchModule(imp.Module, args.Module, args.CaseSens) {
				tmp = append(tmp, imp)
			}
		}
		filtered = tmp
	}
	if args.Regex != "" {
		regex, err := compileRegex(args.Regex, args.CaseSens)
		if err != nil {
			return s.handleToolError(invalidInput(op, err.Error()))
		}
		tmp := make([]*pb.Import, 0, len(filtered))
		for _, imp := range filtered {
			if regex.MatchString(imp.Name) {
				tmp = append(tmp, imp)
			}
		}
		filtered = tmp
	}

	totalImports := len(filtered)
	offset, limit, err := normalizePagination(args.Offset, args.Limit)
	if err != nil {
		return s.handleToolError(invalidInput(op, err.Error()))
	}
	if offset > totalImports {
		offset = totalImports
	}
	end := offset + limit
	if end > totalImports {
		end = totalImports
	}

	imports := mapImportItems(filtered[offset:end])
	result, _ := s.marshalJSON(map[string]interface{}{
		"imports": imports,
		"total":   totalImports,
		"offset":  offset,
		"count":   len(imports),
		"limit":   limit,
		"module":  args.Module,
		"regex":   args.Regex,
	})
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: string(result)},
		},
	}, nil, nil
}

func (s *Server) getExports(ctx context.Context, req *mcp.CallToolRequest, args GetExportsRequest) (*mcp.CallToolResult, any, error) {
	const op = "get_exports"
	s.logToolInvocation(op, args.SessionID, map[string]interface{}{
		"offset": args.Offset,
		"limit":  args.Limit,
		"regex":  args.Regex,
	})
	sess, ok := s.registry.Get(args.SessionID)
	if !ok {
		return s.handleToolError(sessionNotFound(op, args.SessionID))
	}
	sess.Touch()
	client, err := s.workers.GetClient(sess.ID)
	if err != nil {
		return s.handleToolError(workerUnavailable(op, sess.ID, err))
	}

	progress := s.progressReporter(ctx, req, sess.ID, op)
	cache := s.getSessionCache(sess.ID)
	exportsData, hit, err := cache.loadExports(sess.ID, s.logger, func() ([]*pb.Export, error) {
		return s.fetchAllExports(ctx, client, progress)
	})
	if err != nil {
		return s.handleToolError(idaOperationFailed(op, sess.ID, err))
	}
	if hit {
		s.emitProgress(progress, sess.ID, op, "Exports served from cache", 1, 1)
	}

	filtered := exportsData
	if args.Regex != "" {
		regex, err := compileRegex(args.Regex, args.CaseSens)
		if err != nil {
			return s.handleToolError(invalidInput(op, err.Error()))
		}
		tmp := make([]*pb.Export, 0, len(filtered))
		for _, exp := range filtered {
			if regex.MatchString(exp.Name) {
				tmp = append(tmp, exp)
			}
		}
		filtered = tmp
	}

	totalExports := len(filtered)
	offset, limit, err := normalizePagination(args.Offset, args.Limit)
	if err != nil {
		return s.handleToolError(invalidInput(op, err.Error()))
	}
	if offset > totalExports {
		offset = totalExports
	}
	end := offset + limit
	if end > totalExports {
		end = totalExports
	}

	exports := mapExportItems(filtered[offset:end])
	result, _ := s.marshalJSON(map[string]interface{}{
		"exports": exports,
		"total":   totalExports,
		"offset":  offset,
		"count":   len(exports),
		"limit":   limit,
		"regex":   args.Regex,
	})
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: string(result)},
		},
	}, nil, nil
}

func (s *Server) getStrings(ctx context.Context, req *mcp.CallToolRequest, args GetStringsRequest) (*mcp.CallToolResult, any, error) {
	const op = "get_strings"
	s.logToolInvocation(op, args.SessionID, map[string]interface{}{
		"offset": args.Offset,
		"limit":  args.Limit,
		"regex":  args.Regex,
	})
	sess, ok := s.registry.Get(args.SessionID)
	if !ok {
		return s.handleToolError(sessionNotFound(op, args.SessionID))
	}
	sess.Touch()
	client, err := s.workers.GetClient(sess.ID)
	if err != nil {
		return s.handleToolError(workerUnavailable(op, sess.ID, err))
	}

	progress := s.progressReporter(ctx, req, sess.ID, op)
	cache := s.getSessionCache(sess.ID)
	stringsData, hit, err := cache.loadStrings(sess.ID, s.logger, func() ([]*pb.StringItem, error) {
		return s.fetchAllStrings(ctx, client, progress)
	})
	if err != nil {
		return s.handleToolError(idaOperationFailed(op, sess.ID, err))
	}
	if hit {
		s.emitProgress(progress, sess.ID, op, "Strings served from cache", 1, 1)
	}

	filtered := stringsData
	if args.Regex != "" {
		regex, err := compileRegex(args.Regex, args.CaseSens)
		if err != nil {
			return s.handleToolError(invalidInput(op, err.Error()))
		}
		tmp := make([]*pb.StringItem, 0, len(filtered))
		for _, item := range filtered {
			if regex.MatchString(item.Value) {
				tmp = append(tmp, item)
			}
		}
		filtered = tmp
	}

	totalStrings := len(filtered)
	offset, limit, err := normalizePagination(args.Offset, args.Limit)
	if err != nil {
		return s.handleToolError(invalidInput(op, err.Error()))
	}
	if offset > totalStrings {
		offset = totalStrings
	}
	end := offset + limit
	if end > totalStrings {
		end = totalStrings
	}
	selection := mapStringItems(filtered[offset:end])
	result, _ := s.marshalJSON(map[string]interface{}{
		"strings": selection,
		"total":   totalStrings,
		"offset":  offset,
		"count":   len(selection),
		"limit":   limit,
		"regex":   args.Regex,
	})
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: string(result)},
		},
	}, nil, nil
}

func (s *Server) getXRefsTo(ctx context.Context, req *mcp.CallToolRequest, args XRefRequest) (*mcp.CallToolResult, any, error) {
	const op = "get_xrefs_to"
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
	resp, err := (*client.Analysis).GetXRefsTo(ctx, connect.NewRequest(&pb.GetXRefsToRequest{Address: args.Address}))
	if err != nil {
		return s.handleToolError(idaOperationFailed(op, sess.ID, err))
	}
	if msgErr := resp.Msg.GetError(); msgErr != "" {
		return s.handleToolError(idaOperationFailed(op, sess.ID, errors.New(msgErr)))
	}
	entries := make([]map[string]any, 0, len(resp.Msg.GetXrefs()))
	for _, x := range resp.Msg.GetXrefs() {
		entries = append(entries, map[string]any{
			"from": x.GetFrom(),
			"to":   x.GetTo(),
			"type": x.GetType(),
		})
	}
	result, _ := s.marshalJSON(map[string]any{"xrefs": entries, "count": len(entries)})
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(result)}}}, nil, nil
}

func (s *Server) getXRefsFrom(ctx context.Context, req *mcp.CallToolRequest, args XRefRequest) (*mcp.CallToolResult, any, error) {
	const op = "get_xrefs_from"
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
	resp, err := (*client.Analysis).GetXRefsFrom(ctx, connect.NewRequest(&pb.GetXRefsFromRequest{Address: args.Address}))
	if err != nil {
		return s.handleToolError(idaOperationFailed(op, sess.ID, err))
	}
	if msgErr := resp.Msg.GetError(); msgErr != "" {
		return s.handleToolError(idaOperationFailed(op, sess.ID, errors.New(msgErr)))
	}
	entries := make([]map[string]any, 0, len(resp.Msg.GetXrefs()))
	for _, x := range resp.Msg.GetXrefs() {
		entries = append(entries, map[string]any{
			"from": x.GetFrom(),
			"to":   x.GetTo(),
			"type": x.GetType(),
		})
	}
	result, _ := s.marshalJSON(map[string]any{"xrefs": entries, "count": len(entries)})
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(result)}}}, nil, nil
}

func (s *Server) getDataRefs(ctx context.Context, req *mcp.CallToolRequest, args DataRefRequest) (*mcp.CallToolResult, any, error) {
	const op = "get_data_refs"
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
	resp, err := (*client.Analysis).GetDataRefs(ctx, connect.NewRequest(&pb.GetDataRefsRequest{Address: args.Address}))
	if err != nil {
		return s.handleToolError(idaOperationFailed(op, sess.ID, err))
	}
	if msgErr := resp.Msg.GetError(); msgErr != "" {
		return s.handleToolError(idaOperationFailed(op, sess.ID, errors.New(msgErr)))
	}
	entries := make([]map[string]any, 0, len(resp.Msg.GetRefs()))
	for _, ref := range resp.Msg.GetRefs() {
		entries = append(entries, map[string]any{
			"from": ref.GetFrom(),
			"type": ref.GetType(),
		})
	}
	result, _ := s.marshalJSON(map[string]any{"refs": entries, "count": len(entries)})
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(result)}}}, nil, nil
}

func (s *Server) getStringXRefs(ctx context.Context, req *mcp.CallToolRequest, args StringXRefRequest) (*mcp.CallToolResult, any, error) {
	const op = "get_string_xrefs"
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
	resp, err := (*client.Analysis).GetStringXRefs(ctx, connect.NewRequest(&pb.GetStringXRefsRequest{Address: args.Address}))
	if err != nil {
		return s.handleToolError(idaOperationFailed(op, sess.ID, err))
	}
	if msgErr := resp.Msg.GetError(); msgErr != "" {
		return s.handleToolError(idaOperationFailed(op, sess.ID, errors.New(msgErr)))
	}
	entries := make([]map[string]any, 0, len(resp.Msg.GetRefs()))
	for _, ref := range resp.Msg.GetRefs() {
		entries = append(entries, map[string]any{
			"address":          ref.GetAddress(),
			"function_address": ref.GetFunctionAddress(),
			"function_name":    ref.GetFunctionName(),
		})
	}
	result, _ := s.marshalJSON(map[string]any{"refs": entries, "count": len(entries)})
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(result)}}}, nil, nil
}

func (s *Server) getComment(ctx context.Context, req *mcp.CallToolRequest, args GetCommentRequest) (*mcp.CallToolResult, any, error) {
	const op = "get_comment"
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
	resp, err := (*client.Analysis).GetComment(ctx, connect.NewRequest(&pb.GetCommentRequest{
		Address:    args.Address,
		Repeatable: args.Repeatable,
	}))
	if err != nil {
		return s.handleToolError(idaOperationFailed(op, sess.ID, err))
	}
	if msgErr := resp.Msg.GetError(); msgErr != "" {
		return s.handleToolError(idaOperationFailed(op, sess.ID, errors.New(msgErr)))
	}
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: resp.Msg.GetComment()}}}, nil, nil
}

func (s *Server) getFuncComment(ctx context.Context, req *mcp.CallToolRequest, args GetFuncCommentRequest) (*mcp.CallToolResult, any, error) {
	const op = "get_func_comment"
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
	resp, err := (*client.Analysis).GetFuncComment(ctx, connect.NewRequest(&pb.GetFuncCommentRequest{
		Address: args.Address,
	}))
	if err != nil {
		return s.handleToolError(idaOperationFailed(op, sess.ID, err))
	}
	if msgErr := resp.Msg.GetError(); msgErr != "" {
		return s.handleToolError(idaOperationFailed(op, sess.ID, errors.New(msgErr)))
	}
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: resp.Msg.GetComment()}}}, nil, nil
}

func (s *Server) getName(ctx context.Context, req *mcp.CallToolRequest, args GetNameRequest) (*mcp.CallToolResult, any, error) {
	const op = "get_name"
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
	resp, err := (*client.Analysis).GetName(ctx, connect.NewRequest(&pb.GetNameRequest{
		Address: args.Address,
	}))
	if err != nil {
		return s.handleToolError(idaOperationFailed(op, sess.ID, err))
	}
	if msgErr := resp.Msg.GetError(); msgErr != "" {
		return s.handleToolError(idaOperationFailed(op, sess.ID, errors.New(msgErr)))
	}
	result, _ := s.marshalJSON(map[string]any{"name": resp.Msg.GetName()})
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(result)}}}, nil, nil
}

func (s *Server) getFunctionInfo(ctx context.Context, req *mcp.CallToolRequest, args GetFunctionInfoRequest) (*mcp.CallToolResult, any, error) {
	const op = "get_function_info"
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
	resp, err := (*client.Analysis).GetFunctionInfo(ctx, connect.NewRequest(&pb.GetFunctionInfoRequest{Address: args.Address}))
	if err != nil {
		return s.handleToolError(idaOperationFailed(op, sess.ID, err))
	}
	if msgErr := resp.Msg.GetError(); msgErr != "" {
		return s.handleToolError(idaOperationFailed(op, sess.ID, errors.New(msgErr)))
	}
	flags := resp.Msg.GetFlags()
	body, _ := s.marshalJSON(map[string]any{
		"address":    resp.Msg.GetAddress(),
		"name":       resp.Msg.GetName(),
		"start":      resp.Msg.GetStart(),
		"end":        resp.Msg.GetEnd(),
		"size":       resp.Msg.GetSize(),
		"frame_size": resp.Msg.GetFrameSize(),
		"flags": map[string]any{
			"is_library": flags.GetIsLibrary(),
			"is_thunk":   flags.GetIsThunk(),
			"no_return":  flags.GetNoReturn(),
			"has_farseg": flags.GetHasFarseg(),
			"is_static":  flags.GetIsStatic(),
		},
		"calling_convention": resp.Msg.GetCallingConvention(),
		"return_type":        resp.Msg.GetReturnType(),
		"num_args":           resp.Msg.GetNumArgs(),
	})
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(body)}}}, nil, nil
}

func (s *Server) getSegments(ctx context.Context, req *mcp.CallToolRequest, args GetSegmentsRequest) (*mcp.CallToolResult, any, error) {
	const op = "get_segments"
	s.logToolInvocation(op, args.SessionID, nil)
	sess, ok := s.registry.Get(args.SessionID)
	if !ok {
		return s.handleToolError(sessionNotFound(op, args.SessionID))
	}
	sess.Touch()
	client, err := s.workers.GetClient(sess.ID)
	if err != nil {
		return s.handleToolError(workerUnavailable(op, sess.ID, err))
	}
	resp, err := (*client.Analysis).GetSegments(ctx, connect.NewRequest(&pb.GetSegmentsRequest{}))
	if err != nil {
		return s.handleToolError(idaOperationFailed(op, sess.ID, err))
	}
	if msgErr := resp.Msg.GetError(); msgErr != "" {
		return s.handleToolError(idaOperationFailed(op, sess.ID, errors.New(msgErr)))
	}

	segments := make([]map[string]any, 0, len(resp.Msg.GetSegments()))
	for _, seg := range resp.Msg.GetSegments() {
		segments = append(segments, map[string]any{
			"start":       seg.GetStart(),
			"end":         seg.GetEnd(),
			"name":        seg.GetName(),
			"class":       seg.GetSegClass(),
			"permissions": seg.GetPermissions(),
			"bitness":     seg.GetBitness(),
		})
	}

	result, _ := s.marshalJSON(map[string]any{
		"segments": segments,
		"count":    len(segments),
	})
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(result)}}}, nil, nil
}

func (s *Server) getFunctionName(ctx context.Context, req *mcp.CallToolRequest, args GetFunctionNameRequest) (*mcp.CallToolResult, any, error) {
	const op = "get_function_name"
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
	resp, err := (*client.Analysis).GetFunctionName(ctx, connect.NewRequest(&pb.GetFunctionNameRequest{
		Address: args.Address,
	}))
	if err != nil {
		return s.handleToolError(idaOperationFailed(op, sess.ID, err))
	}
	if msgErr := resp.Msg.GetError(); msgErr != "" {
		return s.handleToolError(idaOperationFailed(op, sess.ID, errors.New(msgErr)))
	}
	result, _ := s.marshalJSON(map[string]any{"name": resp.Msg.GetName()})
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(result)}}}, nil, nil
}

func (s *Server) getEntryPoint(ctx context.Context, req *mcp.CallToolRequest, args GetEntryPointRequest) (*mcp.CallToolResult, any, error) {
	const op = "get_entry_point"
	s.logToolInvocation(op, args.SessionID, nil)
	sess, ok := s.registry.Get(args.SessionID)
	if !ok {
		return s.handleToolError(sessionNotFound(op, args.SessionID))
	}
	sess.Touch()
	client, err := s.workers.GetClient(sess.ID)
	if err != nil {
		return s.handleToolError(workerUnavailable(op, sess.ID, err))
	}
	resp, err := (*client.Analysis).GetEntryPoint(ctx, connect.NewRequest(&pb.GetEntryPointRequest{}))
	if err != nil {
		return s.handleToolError(idaOperationFailed(op, sess.ID, err))
	}
	if msgErr := resp.Msg.GetError(); msgErr != "" {
		return s.handleToolError(idaOperationFailed(op, sess.ID, errors.New(msgErr)))
	}
	result, _ := s.marshalJSON(map[string]any{"address": resp.Msg.GetAddress()})
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(result)}}}, nil, nil
}

func (s *Server) getDwordAt(ctx context.Context, req *mcp.CallToolRequest, args GetDwordAtRequest) (*mcp.CallToolResult, any, error) {
	const op = "get_dword_at"
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
	resp, err := (*client.Analysis).GetDwordAt(ctx, connect.NewRequest(&pb.GetDwordAtRequest{Address: args.Address}))
	if err != nil {
		return s.handleToolError(idaOperationFailed(op, sess.ID, err))
	}
	if msgErr := resp.Msg.GetError(); msgErr != "" {
		return s.handleToolError(idaOperationFailed(op, sess.ID, errors.New(msgErr)))
	}
	result, _ := s.marshalJSON(map[string]any{"value": resp.Msg.GetValue()})
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(result)}}}, nil, nil
}

func (s *Server) getQwordAt(ctx context.Context, req *mcp.CallToolRequest, args GetQwordAtRequest) (*mcp.CallToolResult, any, error) {
	const op = "get_qword_at"
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
	resp, err := (*client.Analysis).GetQwordAt(ctx, connect.NewRequest(&pb.GetQwordAtRequest{Address: args.Address}))
	if err != nil {
		return s.handleToolError(idaOperationFailed(op, sess.ID, err))
	}
	if msgErr := resp.Msg.GetError(); msgErr != "" {
		return s.handleToolError(idaOperationFailed(op, sess.ID, errors.New(msgErr)))
	}
	result, _ := s.marshalJSON(map[string]any{"value": resp.Msg.GetValue()})
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(result)}}}, nil, nil
}

func (s *Server) getInstructionLength(ctx context.Context, req *mcp.CallToolRequest, args GetInstructionLengthRequest) (*mcp.CallToolResult, any, error) {
	const op = "get_instruction_length"
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
	resp, err := (*client.Analysis).GetInstructionLength(ctx, connect.NewRequest(&pb.GetInstructionLengthRequest{Address: args.Address}))
	if err != nil {
		return s.handleToolError(idaOperationFailed(op, sess.ID, err))
	}
	if msgErr := resp.Msg.GetError(); msgErr != "" {
		return s.handleToolError(idaOperationFailed(op, sess.ID, errors.New(msgErr)))
	}
	result, _ := s.marshalJSON(map[string]any{"length": resp.Msg.GetLength()})
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(result)}}}, nil, nil
}
