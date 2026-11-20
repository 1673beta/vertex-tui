package tools

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"google.golang.org/genai"
)

// ToolSpec describes a coding agent tool for display in the TUI.
type ToolSpec struct {
	Name        string
	Description string
}

// Executor manages workspace-scoped coding tools for the Vertex AI coding agent.
type Executor struct {
	WorkDir        string
	CommandTimeout time.Duration
}

// NewExecutor creates a new tool executor rooted at workDir.
func NewExecutor(workDir string) *Executor {
	if workDir == "" {
		if wd, err := os.Getwd(); err == nil {
			workDir = wd
		} else {
			workDir = "."
		}
	}
	abs, err := filepath.Abs(workDir)
	if err == nil {
		workDir = abs
	}
	return &Executor{
		WorkDir:        workDir,
		CommandTimeout: 30 * time.Second,
	}
}

// Specs returns the human-readable list of coding agent tools.
func (e *Executor) Specs() []ToolSpec {
	return []ToolSpec{
		{Name: "read_file", Description: "ファイルの内容を読み取ります (行番号範囲指定可) / Read file contents with optional line range"},
		{Name: "write_file", Description: "ファイルを新規作成または上書き保存します / Create or overwrite a file in the workspace"},
		{Name: "edit_file", Description: "既存ファイル内の指定文字列を置換します / Edit an existing file by replacing target text"},
		{Name: "list_files", Description: "ディレクトリ内のファイル一覧を取得します / List files and directories in the workspace"},
		{Name: "search_files", Description: "ワークスペース内のファイルから文字列を検索します / Search for text across workspace files"},
		{Name: "run_command", Description: "シェルコマンドを実行して出力を取得します / Execute a shell command in the workspace"},
	}
}

// Declarations returns the Google GenAI SDK Tool declarations for Vertex AI function calling.
func (e *Executor) Declarations() []*genai.Tool {
	return []*genai.Tool{
		{
			FunctionDeclarations: []*genai.FunctionDeclaration{
				{
					Name:        "read_file",
					Description: "Read the contents of a file in the workspace. Returns numbered lines.",
					Parameters: &genai.Schema{
						Type: genai.TypeObject,
						Properties: map[string]*genai.Schema{
							"path": {
								Type:        genai.TypeString,
								Description: "Relative or absolute path to the file inside the workspace.",
							},
							"start_line": {
								Type:        genai.TypeInteger,
								Description: "Optional 1-based start line number (inclusive).",
							},
							"end_line": {
								Type:        genai.TypeInteger,
								Description: "Optional 1-based end line number (inclusive).",
							},
						},
						Required: []string{"path"},
					},
				},
				{
					Name:        "write_file",
					Description: "Create a new file or overwrite an existing file with the provided content.",
					Parameters: &genai.Schema{
						Type: genai.TypeObject,
						Properties: map[string]*genai.Schema{
							"path": {
								Type:        genai.TypeString,
								Description: "Relative or absolute path to the file inside the workspace.",
							},
							"content": {
								Type:        genai.TypeString,
								Description: "Full text content to write to the file.",
							},
						},
						Required: []string{"path", "content"},
					},
				},
				{
					Name:        "edit_file",
					Description: "Edit an existing file by replacing an exact target string with a replacement string.",
					Parameters: &genai.Schema{
						Type: genai.TypeObject,
						Properties: map[string]*genai.Schema{
							"path": {
								Type:        genai.TypeString,
								Description: "Relative or absolute path to the file inside the workspace.",
							},
							"target": {
								Type:        genai.TypeString,
								Description: "Exact existing text to search for in the file.",
							},
							"replacement": {
								Type:        genai.TypeString,
								Description: "Replacement text that will replace target.",
							},
						},
						Required: []string{"path", "target", "replacement"},
					},
				},
				{
					Name:        "list_files",
					Description: "List files and directories under a directory in the workspace.",
					Parameters: &genai.Schema{
						Type: genai.TypeObject,
						Properties: map[string]*genai.Schema{
							"path": {
								Type:        genai.TypeString,
								Description: "Optional relative directory path (defaults to workspace root '.').",
							},
							"max_depth": {
								Type:        genai.TypeInteger,
								Description: "Optional maximum directory traversal depth (default 3).",
							},
						},
					},
				},
				{
					Name:        "search_files",
					Description: "Search for a text query across files in the workspace and return matching lines.",
					Parameters: &genai.Schema{
						Type: genai.TypeObject,
						Properties: map[string]*genai.Schema{
							"query": {
								Type:        genai.TypeString,
								Description: "Text substring to search for.",
							},
							"path": {
								Type:        genai.TypeString,
								Description: "Optional subdirectory or file path to restrict the search.",
							},
						},
						Required: []string{"query"},
					},
				},
				{
					Name:        "run_command",
					Description: "Execute a shell command in the workspace directory and return stdout/stderr.",
					Parameters: &genai.Schema{
						Type: genai.TypeObject,
						Properties: map[string]*genai.Schema{
							"command": {
								Type:        genai.TypeString,
								Description: "Shell command to execute (e.g. 'go test ./...', 'git status').",
							},
						},
						Required: []string{"command"},
					},
				},
			},
		},
	}
}

// Execute invokes the requested FunctionCall and returns a structured map suitable for genai.NewPartFromFunctionResponse.
func (e *Executor) Execute(ctx context.Context, call *genai.FunctionCall) map[string]any {
	if call == nil {
		return map[string]any{"error": "nil function call"}
	}
	args := call.Args
	if args == nil {
		args = map[string]any{}
	}

	var (
		res map[string]any
		err error
	)

	switch call.Name {
	case "read_file":
		res, err = e.readFile(args)
	case "write_file":
		res, err = e.writeFile(args)
	case "edit_file":
		res, err = e.editFile(args)
	case "list_files":
		res, err = e.listFiles(args)
	case "search_files":
		res, err = e.searchFiles(args)
	case "run_command":
		res, err = e.runCommand(ctx, args)
	default:
		err = fmt.Errorf("unknown tool %q", call.Name)
	}

	if err != nil {
		return map[string]any{
			"status": "error",
			"error":  err.Error(),
		}
	}
	if res == nil {
		res = map[string]any{}
	}
	if _, ok := res["status"]; !ok {
		res["status"] = "ok"
	}
	return res
}

func (e *Executor) resolvePath(rawPath string) (string, error) {
	p := strings.TrimSpace(rawPath)
	if p == "" {
		p = "."
	}
	if !filepath.IsAbs(p) {
		p = filepath.Join(e.WorkDir, p)
	}
	absTarget, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}
	absRoot, err := filepath.Abs(e.WorkDir)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(absRoot, absTarget)
	if err != nil {
		return "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path %q is outside workspace root %q", rawPath, e.WorkDir)
	}
	return absTarget, nil
}

func (e *Executor) readFile(args map[string]any) (map[string]any, error) {
	pathStr, _ := getStringArg(args, "path")
	if pathStr == "" {
		return nil, fmt.Errorf("missing required parameter 'path'")
	}
	target, err := e.resolvePath(pathStr)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(target)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(data), "\n")
	startLine := getIntArg(args, "start_line", 1)
	endLine := getIntArg(args, "end_line", len(lines))
	if startLine < 1 {
		startLine = 1
	}
	if endLine > len(lines) || endLine <= 0 {
		endLine = len(lines)
	}
	if startLine > endLine && len(lines) > 0 {
		return nil, fmt.Errorf("start_line (%d) cannot be greater than end_line (%d)", startLine, endLine)
	}

	var sb strings.Builder
	for i := startLine; i <= endLine && i <= len(lines); i++ {
		fmt.Fprintf(&sb, "%4d | %s\n", i, lines[i-1])
	}

	return map[string]any{
		"path":        pathStr,
		"total_lines": len(lines),
		"start_line":  startLine,
		"end_line":    endLine,
		"content":     sb.String(),
	}, nil
}

func (e *Executor) writeFile(args map[string]any) (map[string]any, error) {
	pathStr, _ := getStringArg(args, "path")
	if pathStr == "" {
		return nil, fmt.Errorf("missing required parameter 'path'")
	}
	content, ok := getStringArg(args, "content")
	if !ok {
		return nil, fmt.Errorf("missing required parameter 'content'")
	}
	target, err := e.resolvePath(pathStr)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return nil, err
	}
	if err := os.WriteFile(target, []byte(content), 0o644); err != nil {
		return nil, err
	}
	return map[string]any{
		"path":          pathStr,
		"bytes_written": len(content),
	}, nil
}

func (e *Executor) editFile(args map[string]any) (map[string]any, error) {
	pathStr, _ := getStringArg(args, "path")
	if pathStr == "" {
		return nil, fmt.Errorf("missing required parameter 'path'")
	}
	targetText, ok := getStringArg(args, "target")
	if !ok || targetText == "" {
		return nil, fmt.Errorf("missing required parameter 'target'")
	}
	replacementText, ok := getStringArg(args, "replacement")
	if !ok {
		return nil, fmt.Errorf("missing required parameter 'replacement'")
	}

	filePath, err := e.resolvePath(pathStr)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	original := string(data)
	count := strings.Count(original, targetText)
	if count == 0 {
		return nil, fmt.Errorf("target text not found in %s", pathStr)
	}
	updated := strings.ReplaceAll(original, targetText, replacementText)
	if err := os.WriteFile(filePath, []byte(updated), 0o644); err != nil {
		return nil, err
	}
	return map[string]any{
		"path":         pathStr,
		"replacements": count,
	}, nil
}

func (e *Executor) listFiles(args map[string]any) (map[string]any, error) {
	pathStr, _ := getStringArg(args, "path")
	if pathStr == "" {
		pathStr = "."
	}
	maxDepth := getIntArg(args, "max_depth", 3)
	if maxDepth <= 0 {
		maxDepth = 3
	}

	root, err := e.resolvePath(pathStr)
	if err != nil {
		return nil, err
	}

	var entries []string
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if path == root {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		name := d.Name()
		if strings.HasPrefix(name, ".") && name != ".env.example" && name != ".gitignore" {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		depth := strings.Count(rel, string(filepath.Separator)) + 1
		if depth > maxDepth {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		display := filepath.ToSlash(rel)
		if d.IsDir() {
			display += "/"
		}
		entries = append(entries, display)
		if len(entries) >= 300 {
			return fs.SkipAll
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(entries)
	return map[string]any{
		"path":  pathStr,
		"count": len(entries),
		"files": entries,
	}, nil
}

func (e *Executor) searchFiles(args map[string]any) (map[string]any, error) {
	query, _ := getStringArg(args, "query")
	if query == "" {
		return nil, fmt.Errorf("missing required parameter 'query'")
	}
	pathStr, _ := getStringArg(args, "path")
	if pathStr == "" {
		pathStr = "."
	}
	root, err := e.resolvePath(pathStr)
	if err != nil {
		return nil, err
	}

	var matches []string
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if d.IsDir() {
			name := d.Name()
			if strings.HasPrefix(name, ".") || name == "vendor" || name == "node_modules" {
				if path != root {
					return filepath.SkipDir
				}
			}
			return nil
		}
		info, err := d.Info()
		if err != nil || info.Size() > 512*1024 {
			return nil
		}
		f, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer f.Close()

		rel, err := filepath.Rel(e.WorkDir, path)
		if err != nil {
			rel = path
		}
		rel = filepath.ToSlash(rel)

		scanner := bufio.NewScanner(f)
		lineNum := 0
		for scanner.Scan() {
			lineNum++
			line := scanner.Text()
			if strings.Contains(line, query) {
				matches = append(matches, fmt.Sprintf("%s:%d: %s", rel, lineNum, line))
				if len(matches) >= 100 {
					return fs.SkipAll
				}
			}
		}
		return nil
	})

	return map[string]any{
		"query":   query,
		"count":   len(matches),
		"matches": matches,
	}, nil
}

func (e *Executor) runCommand(ctx context.Context, args map[string]any) (map[string]any, error) {
	cmdStr, _ := getStringArg(args, "command")
	if strings.TrimSpace(cmdStr) == "" {
		return nil, fmt.Errorf("missing required parameter 'command'")
	}
	timeout := e.CommandTimeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	cmdCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, "sh", "-c", cmdStr)
	cmd.Dir = e.WorkDir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
	}

	outStr := truncateOutput(stdout.String(), 8000)
	errStr := truncateOutput(stderr.String(), 8000)

	result := map[string]any{
		"command":   cmdStr,
		"exit_code": exitCode,
		"stdout":    outStr,
		"stderr":    errStr,
	}
	if err != nil {
		result["error"] = err.Error()
	}
	return result, nil
}

func truncateOutput(s string, maxBytes int) string {
	if len(s) <= maxBytes {
		return s
	}
	return s[:maxBytes] + "\n... [truncated]"
}

func getStringArg(args map[string]any, key string) (string, bool) {
	v, ok := args[key]
	if !ok || v == nil {
		return "", false
	}
	switch val := v.(type) {
	case string:
		return val, true
	default:
		return fmt.Sprintf("%v", val), true
	}
}

func getIntArg(args map[string]any, key string, defaultVal int) int {
	v, ok := args[key]
	if !ok || v == nil {
		return defaultVal
	}
	switch val := v.(type) {
	case int:
		return val
	case int32:
		return int(val)
	case int64:
		return int(val)
	case float64:
		return int(val)
	case float32:
		return int(val)
	default:
		return defaultVal
	}
}
