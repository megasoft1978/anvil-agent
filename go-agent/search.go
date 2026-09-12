package main

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"syscall"
)

// skipDirs excludes version control, dependency, and build-output directories from repo-wide
// search: they are large, not source the model should edit, and would otherwise dominate results.
var skipDirs = map[string]bool{
	".git": true, "node_modules": true, "dist": true, "build": true,
	".next": true, "vendor": true, "target": true, "__pycache__": true, ".cache": true,
}

// walk visits every file and directory under dir (relative to t.Root), skipping skipDirs,
// in a bounded, deterministic (sorted) order.
func (t *Tools) walk(dir string, depth int, visit func(path string, isDir bool) error) error {
	if depth > 40 {
		return fmt.Errorf("directory depth limit exceeded")
	}
	file, err := t.Root.OpenFile(dir, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return err
	}
	entries, err := file.ReadDir(-1)
	file.Close()
	if err != nil {
		return err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	for _, entry := range entries {
		name := entry.Name()
		if skipDirs[name] {
			continue
		}
		child := name
		if dir != "." {
			child = dir + "/" + name
		}
		if entry.IsDir() {
			if err := visit(child, true); err != nil {
				return err
			}
			if err := t.walk(child, depth+1, visit); err != nil {
				return err
			}
		} else if err := visit(child, false); err != nil {
			return err
		}
	}
	return nil
}

const searchMaxMatches = 20
const searchMaxFilesScanned = 4000

// search finds literal text across the worktree (or one file/subtree, if path is given),
// skipping generated directories and unreadable (binary, oversized, non-UTF-8) files, and
// returns a bounded set of matches with nearby source so the model does not have to guess
// filenames or reread whole files to locate relevant code.
func (t *Tools) search(ctx context.Context, raw string) (any, error) {
	var args struct {
		Path string `json:"path"`
		Text string `json:"text"`
	}
	if err := decodeArguments(raw, &args); err != nil {
		return nil, err
	}
	if args.Path == "" {
		args.Path = "."
	}
	if err := checkedPath(args.Path); err != nil {
		return nil, err
	}
	if args.Text == "" || len(args.Text) > 1024 || strings.ContainsAny(args.Text, "\r\n") {
		return nil, fmt.Errorf("text must be a non-empty single-line literal up to 1024 bytes")
	}
	entry, err := t.Root.OpenFile(args.Path, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	info, err := entry.Stat()
	entry.Close()
	if err != nil {
		return nil, err
	}
	var result strings.Builder
	matches := 0
	filesScanned := 0
	truncated := false
	scanFile := func(path string) error {
		if truncated {
			return nil
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if filesScanned >= searchMaxFilesScanned {
			truncated = true
			return nil
		}
		filesScanned++
		file, err := t.Root.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK, 0)
		if err != nil {
			return nil // unreadable entry (permissions, races): skip, do not fail the whole search
		}
		content, err := readText(file)
		file.Close()
		if err != nil {
			return nil // binary, oversized, or non-UTF-8: not a source-search candidate
		}
		lines := strings.Split(content, "\n")
		for i, line := range lines {
			if !strings.Contains(line, args.Text) {
				continue
			}
			var snippet strings.Builder
			fmt.Fprintf(&snippet, "%s:\n", path)
			for n := max(0, i-2); n < min(len(lines), i+3); n++ {
				fmt.Fprintf(&snippet, "%d: %s\n", n+1, lines[n])
			}
			if matches >= searchMaxMatches || result.Len()+snippet.Len()+1 > maxOutputBytes {
				truncated = true
				return nil
			}
			result.WriteString(snippet.String())
			result.WriteByte('\n')
			matches++
		}
		return nil
	}
	if info.IsDir() {
		err = t.walk(args.Path, 0, func(path string, isDir bool) error {
			if isDir {
				return nil
			}
			return scanFile(path)
		})
	} else {
		err = scanFile(args.Path)
	}
	if err != nil {
		return nil, err
	}
	return map[string]any{"path": args.Path, "text": args.Text, "matches": matches,
		"files_scanned": filesScanned, "truncated": truncated, "content": result.String()}, nil
}
