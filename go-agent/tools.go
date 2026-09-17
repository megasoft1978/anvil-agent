package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"syscall"
	"unicode/utf8"
)

const maxFileBytes = 1 << 20
const maxOutputBytes = 32 << 10

type Tools struct {
	Root              *os.Root
	Trace             *Trace
	Edited            map[string]bool
	SearchEnabled     bool
	WriteEnabled      bool
	ReadDisabled      bool
	RichEditFeedback  bool
	ReadOutputLimit   int
	SearchOutputLimit int
}

// editContextLines bounds the source window returned around an edit outcome: enough to show
// the whole affected function in most files without re-dumping the file into the conversation.
const editContextLines = 12

// lineWindow returns the lines around charIndex in content, +/- contextLines lines.
// A negative charIndex is treated as the start of the file.
func lineWindow(content string, charIndex int, contextLines int) string {
	if charIndex < 0 {
		charIndex = 0
	}
	if charIndex > len(content) {
		charIndex = len(content)
	}
	target := strings.Count(content[:charIndex], "\n")
	lines := strings.Split(content, "\n")
	start := max(0, target-contextLines)
	end := min(len(lines), target+contextLines+1)
	return strings.Join(lines[start:end], "\n")
}

func decodeArguments(raw string, target any) error {
	if !strings.HasPrefix(strings.TrimSpace(raw), "{") {
		return fmt.Errorf("arguments must be a JSON object")
	}
	// encoding/json otherwise accepts duplicate keys and case-insensitive struct fields. The tool
	// schema allows neither ambiguity: validate exact field names before decoding into the struct.
	allowed := map[string]bool{}
	typ := reflect.TypeOf(target).Elem()
	for index := 0; index < typ.NumField(); index++ {
		allowed[typ.Field(index).Tag.Get("json")] = true
	}
	keys := json.NewDecoder(strings.NewReader(raw))
	if _, err := keys.Token(); err != nil {
		return err
	}
	seen := map[string]bool{}
	for keys.More() {
		key, err := keys.Token()
		if err != nil {
			return err
		}
		name, ok := key.(string)
		if !ok || !allowed[name] || seen[name] {
			return fmt.Errorf("unknown or duplicate argument %q", key)
		}
		seen[name] = true
		var value json.RawMessage
		if err := keys.Decode(&value); err != nil {
			return err
		}
		if bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return fmt.Errorf("argument %q cannot be null", name)
		}
	}
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if decoder.Decode(new(any)) != io.EOF {
		return fmt.Errorf("unexpected trailing arguments")
	}
	return nil
}

// mkdirAllInRoot creates dir and every missing ancestor under root, in order, tolerating
// components that already exist -- os.Root has no MkdirAll, only single-level Mkdir.
func mkdirAllInRoot(root *os.Root, dir string) error {
	parts := strings.Split(filepath.ToSlash(dir), "/")
	built := ""
	for _, part := range parts {
		if part == "" || part == "." {
			continue
		}
		if built == "" {
			built = part
		} else {
			built = built + "/" + part
		}
		if err := root.Mkdir(built, 0700); err != nil && !os.IsExist(err) {
			return err
		}
	}
	return nil
}

func checkedPath(path string) error {
	if !filepath.IsLocal(path) {
		return fmt.Errorf("path must be relative and inside the worktree")
	}
	for _, part := range strings.Split(filepath.ToSlash(path), "/") {
		if part == ".git" {
			return fmt.Errorf(".git is not available to file tools")
		}
	}
	return nil
}

func readText(file *os.File) (string, error) {
	info, err := file.Stat()
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("expected a regular file")
	}
	data, err := io.ReadAll(io.LimitReader(file, maxFileBytes+1))
	if err != nil {
		return "", err
	}
	if len(data) > maxFileBytes {
		return "", fmt.Errorf("file exceeds %d bytes", maxFileBytes)
	}
	if !utf8.Valid(data) || bytes.IndexByte(data, 0) >= 0 {
		return "", fmt.Errorf("expected a UTF-8 text file")
	}
	return string(data), nil
}

func (t *Tools) Execute(ctx context.Context, call ToolCall) (any, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	switch call.Function.Name {
	case "search":
		if !t.SearchEnabled || t.ReadDisabled {
			return nil, fmt.Errorf("search is not enabled")
		}
		return t.search(ctx, call.Function.Arguments)
	case "read":
		if t.ReadDisabled {
			return nil, fmt.Errorf("read is disabled; use the complete source supplied in the task")
		}
		var args struct {
			Path   string `json:"path"`
			Offset *int   `json:"offset"`
		}
		if err := decodeArguments(call.Function.Arguments, &args); err != nil {
			return nil, err
		}
		if err := checkedPath(args.Path); err != nil {
			return nil, err
		}
		offset := 1
		if args.Offset != nil {
			offset = *args.Offset
		}
		if offset < 1 {
			return nil, fmt.Errorf("offset must be positive")
		}
		file, err := t.Root.OpenFile(args.Path, os.O_RDONLY|syscall.O_NONBLOCK, 0)
		if err != nil {
			return nil, err
		}
		defer file.Close()
		info, err := file.Stat()
		if err != nil {
			return nil, err
		}
		var lines []string
		if info.IsDir() {
			entries, err := file.ReadDir(-1)
			if err != nil {
				return nil, err
			}
			for _, entry := range entries {
				name := entry.Name()
				if name == ".git" {
					continue
				}
				if entry.IsDir() {
					name += "/"
				}
				lines = append(lines, name)
			}
			sort.Strings(lines)
		} else {
			content, err := readText(file)
			if err != nil {
				return nil, err
			}
			lines = strings.Split(content, "\n")
		}
		start := min(offset-1, len(lines))
		end := min(start+200, len(lines))
		output := strings.Join(lines[start:end], "\n")
		limit := t.ReadOutputLimit
		if limit <= 0 {
			limit = maxOutputBytes
		}
		for len(output) > limit && end > start+1 {
			end--
			output = strings.Join(lines[start:end], "\n")
		}
		if len(output) > limit {
			return nil, fmt.Errorf("line exceeds output limit of %d bytes", limit)
		}
		next := 0
		if end < len(lines) {
			next = end + 1
		}
		return map[string]any{"path": args.Path, "content": output, "offset": offset, "next_offset": next, "total": len(lines), "is_dir": info.IsDir()}, nil
	case "edit":
		var args struct {
			Path    string  `json:"path"`
			OldText string  `json:"oldText"`
			NewText *string `json:"newText"`
		}
		if err := decodeArguments(call.Function.Arguments, &args); err != nil {
			return nil, err
		}
		if err := checkedPath(args.Path); err != nil {
			return nil, err
		}
		if args.OldText == "" || args.NewText == nil {
			return nil, fmt.Errorf("oldText must be non-empty and newText is required")
		}
		file, err := t.Root.OpenFile(args.Path, os.O_RDWR|syscall.O_NONBLOCK, 0)
		if err != nil {
			return nil, err
		}
		defer file.Close()
		before, err := readText(file)
		if err != nil {
			return nil, err
		}
		count := strings.Count(before, args.OldText)
		if count != 1 {
			if !t.RichEditFeedback {
				return nil, fmt.Errorf("oldText must occur exactly once; read the file and retry")
			}
			reason := "oldText was not found in the current file"
			location := -1
			if count > 1 {
				reason = fmt.Sprintf("oldText occurs %d times; it must be unique", count)
				location = strings.Index(before, args.OldText)
			} else {
				// The whole block does not match (possibly stale after an earlier edit); try each of
				// its lines in turn so a location is still found when only the first line has drifted.
				for _, candidate := range strings.Split(args.OldText, "\n") {
					if candidate == "" {
						continue
					}
					if index := strings.Index(before, candidate); index >= 0 {
						location = index
						break
					}
				}
			}
			return map[string]any{"path": args.Path, "applied": false, "reason": reason,
				"current_context": lineWindow(before, location, editContextLines),
				"content_sha256":  fmt.Sprintf("%x", sha256.Sum256([]byte(before)))}, nil
		}
		after := strings.Replace(before, args.OldText, *args.NewText, 1)
		if len(after) > maxFileBytes || !utf8.ValidString(after) || strings.ContainsRune(after, 0) {
			return nil, fmt.Errorf("replacement must be UTF-8 text within the file limit")
		}
		if after == before {
			if !t.RichEditFeedback {
				return nil, fmt.Errorf("replacement makes no change")
			}
			return map[string]any{"path": args.Path, "applied": false, "reason": "newText produced no change to the file",
				"current_context": lineWindow(before, strings.Index(before, args.OldText), editContextLines),
				"content_sha256":  fmt.Sprintf("%x", sha256.Sum256([]byte(before)))}, nil
		}
		// Durable before/after evidence precedes the write, so even an interrupted edit is recoverable.
		if err := t.Trace.Event("edit_backup", map[string]any{"path": args.Path, "before": before, "after": after}); err != nil {
			return nil, err
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if _, err := file.WriteAt([]byte(after), 0); err != nil {
			return nil, err
		}
		if err := file.Truncate(int64(len(after))); err != nil {
			return nil, err
		}
		if err := file.Sync(); err != nil {
			return nil, err
		}
		t.Edited[args.Path] = true
		result := map[string]any{"path": args.Path, "applied": true,
			"before_sha256": fmt.Sprintf("%x", sha256.Sum256([]byte(before))),
			"after_sha256":  fmt.Sprintf("%x", sha256.Sum256([]byte(after)))}
		if t.RichEditFeedback {
			result["current_context"] = lineWindow(after, strings.Index(after, *args.NewText), editContextLines)
		}
		return result, nil
	case "write":
		if !t.WriteEnabled {
			return nil, fmt.Errorf("unknown tool %q", call.Function.Name)
		}
		var args struct {
			Path    string `json:"path"`
			Content string `json:"content"`
		}
		if err := decodeArguments(call.Function.Arguments, &args); err != nil {
			return nil, err
		}
		if err := checkedPath(args.Path); err != nil {
			return nil, err
		}
		if args.Path == "" {
			return nil, fmt.Errorf("path must be non-empty")
		}
		if len(args.Content) > maxFileBytes || !utf8.ValidString(args.Content) || strings.ContainsRune(args.Content, 0) {
			return nil, fmt.Errorf("content must be UTF-8 text within the file limit")
		}
		if dir := filepath.Dir(args.Path); dir != "." {
			if err := mkdirAllInRoot(t.Root, dir); err != nil {
				return nil, err
			}
		}
		file, err := t.Root.OpenFile(args.Path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			if os.IsExist(err) {
				return nil, fmt.Errorf("file already exists; use edit to modify it")
			}
			return nil, err
		}
		defer file.Close()
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if _, err := file.WriteString(args.Content); err != nil {
			return nil, err
		}
		if err := file.Sync(); err != nil {
			return nil, err
		}
		t.Edited[args.Path] = true
		if err := t.Trace.Event("write", map[string]any{"path": args.Path, "content": args.Content}); err != nil {
			return nil, err
		}
		return map[string]any{"path": args.Path, "created": true, "bytes": len(args.Content),
			"content_sha256": fmt.Sprintf("%x", sha256.Sum256([]byte(args.Content)))}, nil
	default:
		return nil, fmt.Errorf("unknown tool %q", call.Function.Name)
	}
}
