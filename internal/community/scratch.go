package community

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"
)

// One requester's text filesystem, living for one rollout. Deliberately not the
// job Workspace in workspace.go. See docs/sirens-echo-scratchpad.md.

const (
	// maxScratchFileBytes bounds one file. A Discord message can ask for an
	// unbounded write and the volume is shared with the pod.
	maxScratchFileBytes = 256 * 1024
	// maxScratchPartitionBytes bounds one requester's footprint, so a single
	// account cannot fill the volume for every other account on it.
	maxScratchPartitionBytes = 4 * 1024 * 1024
	// maxScratchEntries bounds a listing, keeping a result inside the turn
	// budget rather than returning a directory of unknown size.
	maxScratchEntries = 200
	// maxScratchMatches bounds a search result for the same reason.
	maxScratchMatches = 100
	// maxScratchDepth bounds nesting so a walk stays cheap.
	maxScratchDepth = 8
	// scratchPermissions keep a partition readable only by this process, and
	// deny the execute bit that text-only is enforced by rather than described.
	scratchPermissions     = 0o700
	scratchFilePermissions = 0o600
)

// requesterKey carries the turn's requester to a tool layer that is
// process-wide. See docs/sirens-echo-scratchpad.md.
type requesterKey struct{}

// WithRequester marks a context as belonging to one requesting principal.
func WithRequester(ctx context.Context, requesterID string) context.Context {
	trimmed := strings.TrimSpace(requesterID)
	if trimmed == "" {
		return ctx
	}
	return context.WithValue(ctx, requesterKey{}, trimmed)
}

// RequesterFrom reports the requesting principal a context carries.
func RequesterFrom(ctx context.Context) string {
	value, _ := ctx.Value(requesterKey{}).(string)
	return value
}

// ErrNoScratchRequester refuses a scratch turn carrying no principal. Absence
// is denial, matching the admission gate and the grant table.
var ErrNoScratchRequester = errors.New(
	"scratchpad refused: the turn carries no requesting principal to partition by",
)

// ScratchProvider exposes a per-requester text filesystem as tools. An empty
// Root offers no tools at all, rather than tools that fail.
type ScratchProvider struct {
	Root string
}

// Open partitions the scratchpad for this turn's requester, supplying the
// attribution executionguard.go refuses execution for lacking.
func (p *ScratchProvider) Open(ctx context.Context) (ToolSession, error) {
	if p == nil || strings.TrimSpace(p.Root) == "" {
		return &scratchSession{}, nil
	}
	requester := RequesterFrom(ctx)
	if requester == "" {
		return nil, ErrNoScratchRequester
	}
	partition := filepath.Join(p.Root, scratchPartitionName(requester))
	if err := os.MkdirAll(partition, scratchPermissions); err != nil {
		return nil, fmt.Errorf("prepare scratchpad partition: %w", err)
	}
	return &scratchSession{root: partition, enabled: true}, nil
}

// scratchPartitionName derives a flat directory name from the requester, by
// hashing rather than stripping. See docs/sirens-echo-scratchpad-partitions.md.
func scratchPartitionName(requesterID string) string {
	trimmed := strings.TrimSpace(requesterID)
	if trimmed == "" {
		return "unattributed"
	}
	// A caller-asserted identifier reaches this, so two requesters differing
	// only in punctuation must not land in one partition.
	sum := sha256.Sum256([]byte(trimmed))
	return hex.EncodeToString(sum[:16])
}

type scratchSession struct {
	root    string
	enabled bool
}

func (s *scratchSession) Grounding() []GroundingDocument { return nil }

func (s *scratchSession) Unavailable() []string { return nil }

func (s *scratchSession) Close() error { return nil }

// Tools offers nothing when the capability is off, so no schema reaches the
// prompt on a deployment that mounts no scratchpad.
func (s *scratchSession) Tools() []ToolDefinition {
	if !s.enabled {
		return nil
	}
	return []ToolDefinition{
		{
			Name:     "scratch_list",
			Original: "scratch_list",
			Server:   "scratchpad",
			Description: "List files in your scratchpad. The scratchpad is private to " +
				"you and is erased when the service restarts.",
			InputSchema: scratchObjectSchema(map[string]any{
				"path": scratchStringProperty(
					"Directory to list, relative to the scratchpad root. Defaults to the root.",
				),
			}, nil),
		},
		{
			Name:        "scratch_read",
			Original:    "scratch_read",
			Server:      "scratchpad",
			Description: "Read a UTF-8 text file from your scratchpad.",
			InputSchema: scratchObjectSchema(map[string]any{
				"path": scratchStringProperty("File to read, relative to the scratchpad root."),
			}, []string{"path"}),
		},
		{
			Name:     "scratch_write",
			Original: "scratch_write",
			Server:   "scratchpad",
			Description: "Write a UTF-8 text file to your scratchpad, creating or " +
				"replacing it. Text only.",
			InputSchema: scratchObjectSchema(map[string]any{
				"path":    scratchStringProperty("File to write, relative to the scratchpad root."),
				"content": scratchStringProperty("UTF-8 text to store."),
			}, []string{"path", "content"}),
		},
		{
			Name:        "scratch_search",
			Original:    "scratch_search",
			Server:      "scratchpad",
			Description: "Find lines matching a substring across your scratchpad.",
			InputSchema: scratchObjectSchema(map[string]any{
				"query": scratchStringProperty("Substring to look for. Matching is case-insensitive."),
				"path": scratchStringProperty(
					"Directory to search, relative to the scratchpad root. Defaults to the root.",
				),
			}, []string{"query"}),
		},
	}
}

func scratchObjectSchema(properties map[string]any, required []string) map[string]any {
	schema := map[string]any{
		"type":       "object",
		"properties": properties,
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

func scratchStringProperty(description string) map[string]any {
	return map[string]any{"type": "string", "description": description}
}

// Call runs one scratchpad verb. A refusal comes back as an error result the
// model can read and correct, not as a Go error, which would fail the turn.
func (s *scratchSession) Call(
	ctx context.Context,
	name string,
	arguments map[string]any,
) (ToolResult, error) {
	if !s.enabled {
		return ToolResult{}, fmt.Errorf("model requested unavailable scratchpad tool %q", name)
	}
	if err := ctx.Err(); err != nil {
		return ToolResult{}, err
	}
	switch name {
	case "scratch_list":
		return s.list(scratchStringArg(arguments, "path"))
	case "scratch_read":
		return s.read(scratchStringArg(arguments, "path"))
	case "scratch_write":
		return s.write(scratchStringArg(arguments, "path"), scratchStringArg(arguments, "content"))
	case "scratch_search":
		return s.search(scratchStringArg(arguments, "query"), scratchStringArg(arguments, "path"))
	default:
		return ToolResult{}, fmt.Errorf("model requested unavailable scratchpad tool %q", name)
	}
}

func scratchStringArg(arguments map[string]any, key string) string {
	value, _ := arguments[key].(string)
	return value
}

func scratchRefusal(format string, args ...any) (ToolResult, error) {
	return ToolResult{Text: fmt.Sprintf(format, args...), IsError: true}, nil
}

// reservedScratchPath reports a path landing in the reserved directory. It
// cleans first, so where a path lands decides it rather than how it is spelled.
func reservedScratchPath(relative string) bool {
	cleaned := path.Clean("/" + strings.ReplaceAll(strings.TrimSpace(relative), "\\", "/"))
	trimmed := strings.TrimPrefix(cleaned, "/")
	first, _, _ := strings.Cut(trimmed, "/")
	return strings.EqualFold(first, scratchReservedDir)
}

// resolve confines a model-supplied path to the partition, deciding on where it
// lands rather than on how it is spelled.
func (s *scratchSession) resolve(relative string) (string, error) {
	trimmed := strings.TrimSpace(relative)
	if trimmed == "" {
		return s.root, nil
	}
	if filepath.IsAbs(trimmed) || strings.HasPrefix(trimmed, "/") {
		return "", errors.New("path must be relative to the scratchpad root")
	}
	if strings.ContainsRune(trimmed, 0) {
		return "", errors.New("path contains a null byte")
	}
	// Refused rather than normalized away, which would write elsewhere while
	// reporting the path asked for. See docs/sirens-echo-scratchpad.md.
	for _, segment := range strings.Split(filepath.ToSlash(trimmed), "/") {
		if segment == ".." {
			return "", errors.New("path must not contain a parent segment")
		}
	}
	cleaned := strings.TrimPrefix(path.Clean("/"+filepath.ToSlash(trimmed)), "/")
	if cleaned == "" || cleaned == "." {
		return s.root, nil
	}
	if strings.Count(cleaned, "/") >= maxScratchDepth {
		return "", fmt.Errorf("path nests deeper than %d directories", maxScratchDepth)
	}
	candidate := filepath.Join(s.root, filepath.FromSlash(cleaned))
	return candidate, s.confine(candidate)
}

// confine rejects a target escaping the partition once symlinks are followed,
// evaluating the nearest existing ancestor so a new path is still checked.
func (s *scratchSession) confine(candidate string) error {
	rootReal, err := filepath.EvalSymlinks(s.root)
	if err != nil {
		return fmt.Errorf("resolve scratchpad root: %w", err)
	}
	probe := candidate
	for {
		resolved, resolveErr := filepath.EvalSymlinks(probe)
		if resolveErr == nil {
			if !scratchWithinRoot(rootReal, resolved) {
				return errors.New("path escapes the scratchpad root")
			}
			return nil
		}
		if !errors.Is(resolveErr, fs.ErrNotExist) {
			return fmt.Errorf("resolve scratchpad path: %w", resolveErr)
		}
		parent := filepath.Dir(probe)
		if parent == probe {
			return errors.New("path escapes the scratchpad root")
		}
		probe = parent
	}
}

func scratchWithinRoot(root, target string) bool {
	if target == root {
		return true
	}
	return strings.HasPrefix(target, root+string(os.PathSeparator))
}

func (s *scratchSession) list(relative string) (ToolResult, error) {
	target, err := s.resolve(relative)
	if err != nil {
		return scratchRefusal("%v", err)
	}
	entries := make([]string, 0, 16)
	walkErr := filepath.WalkDir(target, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if len(entries) >= maxScratchEntries {
			return fs.SkipAll
		}
		if p == target {
			return nil
		}
		rel, relErr := filepath.Rel(s.root, p)
		if relErr != nil {
			return relErr
		}
		if d.IsDir() {
			entries = append(entries, filepath.ToSlash(rel)+"/")
			return nil
		}
		info, infoErr := d.Info()
		if infoErr != nil {
			return infoErr
		}
		entries = append(entries, fmt.Sprintf("%s (%d bytes)", filepath.ToSlash(rel), info.Size()))
		return nil
	})
	if errors.Is(walkErr, fs.ErrNotExist) {
		return scratchRefusal("no such directory: %s", scratchDisplayPath(relative))
	}
	if walkErr != nil {
		return ToolResult{}, fmt.Errorf("list scratchpad: %w", walkErr)
	}
	if len(entries) == 0 {
		return ToolResult{Text: "the scratchpad is empty"}, nil
	}
	sort.Strings(entries)
	return ToolResult{Text: strings.Join(entries, "\n")}, nil
}

func (s *scratchSession) read(relative string) (ToolResult, error) {
	if strings.TrimSpace(relative) == "" {
		return scratchRefusal("path is required")
	}
	target, err := s.resolve(relative)
	if err != nil {
		return scratchRefusal("%v", err)
	}
	info, err := os.Stat(target)
	if errors.Is(err, fs.ErrNotExist) {
		return scratchRefusal("no such file: %s", scratchDisplayPath(relative))
	}
	if err != nil {
		return ToolResult{}, fmt.Errorf("stat scratchpad file: %w", err)
	}
	if info.IsDir() {
		return scratchRefusal("%s is a directory, use scratch_list", scratchDisplayPath(relative))
	}
	if info.Size() > maxScratchFileBytes {
		return scratchRefusal(
			"%s is %d bytes, over the %d byte read limit",
			scratchDisplayPath(relative), info.Size(), maxScratchFileBytes,
		)
	}
	data, err := os.ReadFile(target)
	if err != nil {
		return ToolResult{}, fmt.Errorf("read scratchpad file: %w", err)
	}
	if !utf8.Valid(data) {
		return scratchRefusal("%s is not UTF-8 text", scratchDisplayPath(relative))
	}
	return ToolResult{Text: string(data)}, nil
}

// scratchReservedDir holds what the runtime wrote. The model cannot write here,
// so provenance is a property rather than a convention.
const scratchReservedDir = "tool-output"

// WriteReserved writes on the runtime's behalf, into the directory the model is
// refused. See docs/sirens-echo-scratchpad-partitions.md.
func (s *scratchSession) WriteReserved(relative, content string) (ToolResult, error) {
	return s.writeAt(relative, content, true)
}

func (s *scratchSession) write(relative, content string) (ToolResult, error) {
	return s.writeAt(relative, content, false)
}

// writeAt refuses a model-authored write into the reserved directory, so a file
// found there was written by the runtime and not by something imitating it.
func (s *scratchSession) writeAt(relative, content string, runtime bool) (ToolResult, error) {
	if !runtime && reservedScratchPath(relative) {
		return scratchRefusal("%s is reserved for runtime output", scratchReservedDir)
	}
	if strings.TrimSpace(relative) == "" {
		return scratchRefusal("path is required")
	}
	if !utf8.ValidString(content) {
		return scratchRefusal("content must be UTF-8 text")
	}
	if len(content) > maxScratchFileBytes {
		return scratchRefusal(
			"content is %d bytes, over the %d byte file limit",
			len(content), maxScratchFileBytes,
		)
	}
	target, err := s.resolve(relative)
	if err != nil {
		return scratchRefusal("%v", err)
	}
	if target == s.root {
		return scratchRefusal("path is required")
	}
	used, err := s.partitionBytes()
	if err != nil {
		return ToolResult{}, err
	}
	projected := used - scratchExistingSize(target) + int64(len(content))
	if projected > maxScratchPartitionBytes {
		return scratchRefusal(
			"writing %s would use %d bytes, over the %d byte scratchpad limit",
			scratchDisplayPath(relative), projected, maxScratchPartitionBytes,
		)
	}
	if err := os.MkdirAll(filepath.Dir(target), scratchPermissions); err != nil {
		return ToolResult{}, fmt.Errorf("prepare scratchpad directory: %w", err)
	}
	if err := os.WriteFile(target, []byte(content), scratchFilePermissions); err != nil {
		return ToolResult{}, fmt.Errorf("write scratchpad file: %w", err)
	}
	return ToolResult{
		Text: fmt.Sprintf("wrote %s (%d bytes)", scratchDisplayPath(relative), len(content)),
	}, nil
}

func (s *scratchSession) search(query, relative string) (ToolResult, error) {
	if strings.TrimSpace(query) == "" {
		return scratchRefusal("query is required")
	}
	target, err := s.resolve(relative)
	if err != nil {
		return scratchRefusal("%v", err)
	}
	needle := strings.ToLower(query)
	matches := make([]string, 0, 16)
	walkErr := filepath.WalkDir(target, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if len(matches) >= maxScratchMatches {
			return fs.SkipAll
		}
		if d.IsDir() {
			return nil
		}
		info, infoErr := d.Info()
		if infoErr != nil || info.Size() > maxScratchFileBytes {
			return nil
		}
		data, readErr := os.ReadFile(p)
		if readErr != nil || !utf8.Valid(data) {
			return nil
		}
		rel, relErr := filepath.Rel(s.root, p)
		if relErr != nil {
			return relErr
		}
		for i, line := range strings.Split(string(data), "\n") {
			if len(matches) >= maxScratchMatches {
				return fs.SkipAll
			}
			if strings.Contains(strings.ToLower(line), needle) {
				matches = append(matches, fmt.Sprintf(
					"%s:%d: %s", filepath.ToSlash(rel), i+1, strings.TrimSpace(line),
				))
			}
		}
		return nil
	})
	if errors.Is(walkErr, fs.ErrNotExist) {
		return scratchRefusal("no such directory: %s", scratchDisplayPath(relative))
	}
	if walkErr != nil {
		return ToolResult{}, fmt.Errorf("search scratchpad: %w", walkErr)
	}
	if len(matches) == 0 {
		return ToolResult{Text: "no matches"}, nil
	}
	return ToolResult{Text: strings.Join(matches, "\n")}, nil
}

func (s *scratchSession) partitionBytes() (int64, error) {
	var total int64
	err := filepath.WalkDir(s.root, func(_ string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		info, infoErr := d.Info()
		if infoErr != nil {
			return infoErr
		}
		total += info.Size()
		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("measure scratchpad: %w", err)
	}
	return total, nil
}

func scratchExistingSize(target string) int64 {
	info, err := os.Stat(target)
	if err != nil || info.IsDir() {
		return 0
	}
	return info.Size()
}

// scratchDisplayPath echoes the requested path without the partition prefix,
// which names the requester.
func scratchDisplayPath(relative string) string {
	trimmed := strings.TrimSpace(relative)
	if trimmed == "" {
		return "."
	}
	return filepath.ToSlash(path.Clean(trimmed))
}
