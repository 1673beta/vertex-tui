package tools

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"google.golang.org/genai"
)

func TestExecutorReadWriteEditListSearchRun(t *testing.T) {
	tmpDir := t.TempDir()
	exec := NewExecutor(tmpDir)

	if len(exec.Declarations()) == 0 || len(exec.Declarations()[0].FunctionDeclarations) != 6 {
		t.Fatalf("expected 6 function declarations")
	}

	// 1. write_file
	writeRes := exec.Execute(context.Background(), &genai.FunctionCall{
		Name: "write_file",
		Args: map[string]any{
			"path":    "pkg/hello.go",
			"content": "package pkg\n\nfunc Greet() string {\n\treturn \"hello\"\n}\n",
		},
	})
	if writeRes["status"] != "ok" {
		t.Fatalf("write_file failed: %+v", writeRes)
	}

	// 2. read_file
	readRes := exec.Execute(context.Background(), &genai.FunctionCall{
		Name: "read_file",
		Args: map[string]any{
			"path": "pkg/hello.go",
		},
	})
	if readRes["status"] != "ok" {
		t.Fatalf("read_file failed: %+v", readRes)
	}
	content, _ := readRes["content"].(string)
	if !strings.Contains(content, "func Greet()") {
		t.Fatalf("unexpected read_file content: %s", content)
	}

	// 3. edit_file
	editRes := exec.Execute(context.Background(), &genai.FunctionCall{
		Name: "edit_file",
		Args: map[string]any{
			"path":        "pkg/hello.go",
			"target":      "\"hello\"",
			"replacement": "\"hello vertex\"",
		},
	})
	if editRes["status"] != "ok" {
		t.Fatalf("edit_file failed: %+v", editRes)
	}

	// 4. search_files
	searchRes := exec.Execute(context.Background(), &genai.FunctionCall{
		Name: "search_files",
		Args: map[string]any{
			"query": "hello vertex",
		},
	})
	if searchRes["status"] != "ok" {
		t.Fatalf("search_files failed: %+v", searchRes)
	}
	if searchRes["count"] != 1 {
		t.Fatalf("expected 1 search match, got %+v", searchRes)
	}

	// 5. list_files
	listRes := exec.Execute(context.Background(), &genai.FunctionCall{
		Name: "list_files",
		Args: map[string]any{},
	})
	if listRes["status"] != "ok" {
		t.Fatalf("list_files failed: %+v", listRes)
	}

	// 6. run_command
	cmdRes := exec.Execute(context.Background(), &genai.FunctionCall{
		Name: "run_command",
		Args: map[string]any{
			"command": "echo vertex-agent-test",
		},
	})
	if cmdRes["status"] != "ok" || !strings.Contains(cmdRes["stdout"].(string), "vertex-agent-test") {
		t.Fatalf("run_command failed: %+v", cmdRes)
	}

	// 7. Path traversal check
	escapeRes := exec.Execute(context.Background(), &genai.FunctionCall{
		Name: "read_file",
		Args: map[string]any{
			"path": filepath.Join("..", "outside.txt"),
		},
	})
	if escapeRes["status"] != "error" {
		t.Fatalf("expected path outside workspace to fail, got %+v", escapeRes)
	}
}
