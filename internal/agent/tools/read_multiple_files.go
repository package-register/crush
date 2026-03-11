package tools

import (
	"context"
	_ "embed"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"unicode/utf8"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/filepathext"
	"github.com/charmbracelet/crush/internal/filetracker"
	"github.com/charmbracelet/crush/internal/fsext"
	"github.com/charmbracelet/crush/internal/permission"
)

//go:embed read_multiple_files.md
var readMultipleFilesDescription []byte

const ReadMultipleFilesToolName = "read_multiple_files"

// ReadMultipleFilesParams defines the parameters for reading multiple files.
type ReadMultipleFilesParams struct {
	Paths       []string `json:"paths" description:"List of file paths to read (supports glob patterns)"`
	MaxFileSize int64    `json:"max_file_size,omitempty" description:"Maximum size per file in bytes (default 5MB)"`
	Encoding    string   `json:"encoding,omitempty" description:"File encoding (default UTF-8)"`
}

// ReadMultipleFilesPermissionsParams defines the permissions parameters.
type ReadMultipleFilesPermissionsParams struct {
	Paths       []string `json:"paths"`
	MaxFileSize int64    `json:"max_file_size,omitempty"`
	Encoding    string   `json:"encoding,omitempty"`
}

// FileResult represents the result of reading a single file.
type FileResult struct {
	Path    string `json:"path"`
	Content string `json:"content,omitempty"`
	Error   string `json:"error,omitempty"`
	Size    int64  `json:"size"`
}

// ReadMultipleFilesResult represents the result of reading multiple files.
type ReadMultipleFilesResult struct {
	Files []FileResult `json:"files"`
}

// ReadMultipleFilesResponseMetadata contains metadata about the operation.
type ReadMultipleFilesResponseMetadata struct {
	TotalFiles   int `json:"total_files"`
	SuccessCount int `json:"success_count"`
	FailureCount int `json:"failure_count"`
	TotalSize    int `json:"total_size"`
}

const (
	// DefaultMaxFileSize is the default maximum file size (5MB).
	DefaultMaxFileSize = 5 * 1024 * 1024
	// MaxFilesPerRequest is the maximum number of files that can be read in one request.
	MaxFilesPerRequest = 100
)

// NewReadMultipleFilesTool creates a new tool for reading multiple files concurrently.
func NewReadMultipleFilesTool(
	permissions permission.Service,
	filetracker filetracker.Service,
	workingDir string,
) fantasy.AgentTool {
	return fantasy.NewParallelAgentTool(
		ReadMultipleFilesToolName,
		string(readMultipleFilesDescription),
		func(ctx context.Context, params ReadMultipleFilesParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if len(params.Paths) == 0 {
				return fantasy.NewTextErrorResponse("paths parameter is required and must not be empty"), nil
			}

			if len(params.Paths) > MaxFilesPerRequest {
				return fantasy.NewTextErrorResponse(
					fmt.Sprintf("too many paths: %d (maximum is %d). Please split into multiple requests.",
						len(params.Paths), MaxFilesPerRequest)), nil
			}

			// Set default max file size if not provided
			maxFileSize := params.MaxFileSize
			if maxFileSize <= 0 {
				maxFileSize = DefaultMaxFileSize
			}

			// Expand glob patterns in paths
			allPaths, expansionErrors := expandGlobPaths(params.Paths, workingDir)

			// Read all files concurrently
			results := readFilesConcurrently(ctx, allPaths, maxFileSize, workingDir, permissions, filetracker)

			// Add expansion errors to results
			for path, err := range expansionErrors {
				results = append(results, FileResult{
					Path:  path,
					Error: err,
					Size:  0,
				})
			}

			// Build response
			successCount := 0
			failureCount := 0
			totalSize := 0

			var output strings.Builder
			output.WriteString("<files>\n")

			for _, result := range results {
				if result.Error != "" {
					failureCount++
					fmt.Fprintf(&output, "<file_error>\n")
					fmt.Fprintf(&output, "  <path>%s</path>\n", result.Path)
					fmt.Fprintf(&output, "  <error>%s</error>\n", result.Error)
					fmt.Fprintf(&output, "</file_error>\n")
				} else {
					successCount++
					totalSize += int(result.Size)
					fmt.Fprintf(&output, "<file>\n")
					fmt.Fprintf(&output, "  <path>%s</path>\n", result.Path)
					fmt.Fprintf(&output, "  <size>%d</size>\n", result.Size)
					fmt.Fprintf(&output, "  <content>\n%s\n  </content>\n", result.Content)
					fmt.Fprintf(&output, "</file>\n")
				}
			}

			output.WriteString("</files>\n")

			// Add summary
			fmt.Fprintf(&output, "\n<summary>\n")
			fmt.Fprintf(&output, "  Total: %d files\n", len(results))
			fmt.Fprintf(&output, "  Success: %d files\n", successCount)
			fmt.Fprintf(&output, "  Failed: %d files\n", failureCount)
			fmt.Fprintf(&output, "  Total size: %d bytes\n", totalSize)
			fmt.Fprintf(&output, "</summary>\n")

			if failureCount > 0 {
				output.WriteString("\nNote: Some files failed to read. Check individual file_error entries for details.\n")
			}

			return fantasy.WithResponseMetadata(
				fantasy.NewTextResponse(output.String()),
				ReadMultipleFilesResponseMetadata{
					TotalFiles:   len(results),
					SuccessCount: successCount,
					FailureCount: failureCount,
					TotalSize:    totalSize,
				},
			), nil
		})
}

// expandGlobPaths expands glob patterns in the given paths.
func expandGlobPaths(paths []string, workingDir string) ([]string, map[string]string) {
	var expandedPaths []string
	errors := make(map[string]string)

	for _, path := range paths {
		// Handle relative paths
		absPath := filepathext.SmartJoin(workingDir, path)

		// Check if path contains glob patterns
		if containsGlobPattern(path) {
			matches, truncated, err := fsext.GlobGitignoreAware(path, workingDir, MaxFilesPerRequest)
			if err != nil {
				errors[path] = fmt.Sprintf("glob pattern error: %v", err)
				continue
			}

			if len(matches) == 0 {
				errors[path] = fmt.Sprintf("no files matched pattern: %s", path)
				continue
			}

			// Check if results were truncated
			if truncated {
				slog.Warn("Glob results truncated", "pattern", path, "limit", MaxFilesPerRequest)
			}

			expandedPaths = append(expandedPaths, matches...)
		} else {
			// Direct path
			expandedPaths = append(expandedPaths, absPath)
		}
	}

	return expandedPaths, errors
}

// containsGlobPattern checks if a path contains glob patterns.
func containsGlobPattern(path string) bool {
	return strings.ContainsAny(path, "*?[]")
}

// readFilesConcurrently reads multiple files concurrently.
func readFilesConcurrently(
	ctx context.Context,
	paths []string,
	maxFileSize int64,
	workingDir string,
	permissions permission.Service,
	filetracker filetracker.Service,
) []FileResult {
	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		results []FileResult
	)

	// Create a semaphore to limit concurrency
	sem := make(chan struct{}, 10) // Max 10 concurrent reads

	for _, path := range paths {
		wg.Add(1)
		sem <- struct{}{} // Acquire semaphore

		go func(filePath string) {
			defer wg.Done()
			defer func() { <-sem }() // Release semaphore

			result := readFile(ctx, filePath, maxFileSize, workingDir, permissions, filetracker)

			mu.Lock()
			results = append(results, result)
			mu.Unlock()
		}(path)
	}

	wg.Wait()
	return results
}

// readFile reads a single file with all necessary checks.
func readFile(
	ctx context.Context,
	filePath string,
	maxFileSize int64,
	workingDir string,
	permissions permission.Service,
	filetracker filetracker.Service,
) FileResult {
	// Check if file exists
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			// Try to offer suggestions for similarly named files
			suggestions := findSimilarFiles(filePath)
			if len(suggestions) > 0 {
				return FileResult{
					Path: filePath,
					Error: fmt.Sprintf("file not found: %s\n\nDid you mean one of these?\n%s",
						filePath, strings.Join(suggestions, "\n")),
					Size: 0,
				}
			}
			return FileResult{
				Path:  filePath,
				Error: fmt.Sprintf("file not found: %s", filePath),
				Size:  0,
			}
		}
		return FileResult{
			Path:  filePath,
			Error: fmt.Sprintf("error accessing file: %v", err),
			Size:  0,
		}
	}

	// Check if it's a directory
	if fileInfo.IsDir() {
		return FileResult{
			Path:  filePath,
			Error: fmt.Sprintf("path is a directory, not a file: %s", filePath),
			Size:  0,
		}
	}

	// Check file size
	if fileInfo.Size() > maxFileSize {
		return FileResult{
			Path: filePath,
			Error: fmt.Sprintf("file is too large (%d bytes). Maximum size is %d bytes",
				fileInfo.Size(), maxFileSize),
			Size: fileInfo.Size(),
		}
	}

	// Check if file is outside working directory and request permission if needed
	absWorkingDir, err := filepath.Abs(workingDir)
	if err != nil {
		return FileResult{
			Path:  filePath,
			Error: fmt.Sprintf("error resolving working directory: %v", err),
			Size:  0,
		}
	}

	absFilePath, err := filepath.Abs(filePath)
	if err != nil {
		return FileResult{
			Path:  filePath,
			Error: fmt.Sprintf("error resolving file path: %v", err),
			Size:  0,
		}
	}

	relPath, err := filepath.Rel(absWorkingDir, absFilePath)
	isOutsideWorkDir := err != nil || strings.HasPrefix(relPath, "..")

	sessionID := GetSessionFromContext(ctx)
	if isOutsideWorkDir && sessionID != "" {
		granted, permErr := permissions.Request(ctx,
			permission.CreatePermissionRequest{
				SessionID:   sessionID,
				Path:        absFilePath,
				ToolName:    ReadMultipleFilesToolName,
				Action:      "read",
				Description: fmt.Sprintf("Read file outside working directory: %s", absFilePath),
				Params: ReadMultipleFilesPermissionsParams{
					Paths:       []string{filePath},
					MaxFileSize: maxFileSize,
				},
			},
		)
		if permErr != nil {
			return FileResult{
				Path:  filePath,
				Error: fmt.Sprintf("permission request error: %v", permErr),
				Size:  0,
			}
		}
		if !granted {
			return FileResult{
				Path:  filePath,
				Error: "permission denied",
				Size:  0,
			}
		}
	}

	// Read the file content
	content, err := os.ReadFile(filePath)
	if err != nil {
		return FileResult{
			Path:  filePath,
			Error: fmt.Sprintf("error reading file: %v", err),
			Size:  0,
		}
	}

	// Check if content is valid UTF-8
	if !utf8.Valid(content) {
		return FileResult{
			Path:  filePath,
			Error: "file content is not valid UTF-8 (binary file?)",
			Size:  fileInfo.Size(),
		}
	}

	// Record file read in tracker
	if sessionID != "" {
		filetracker.RecordRead(ctx, sessionID, filePath)
	}

	return FileResult{
		Path:    filePath,
		Content: string(content),
		Size:    fileInfo.Size(),
	}
}

// findSimilarFiles finds files with similar names to the given path.
func findSimilarFiles(filePath string) []string {
	dir := filepath.Dir(filePath)
	base := filepath.Base(filePath)

	dirEntries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var suggestions []string
	for _, entry := range dirEntries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.Contains(strings.ToLower(name), strings.ToLower(base)) ||
			strings.Contains(strings.ToLower(base), strings.ToLower(name)) {
			suggestions = append(suggestions, filepath.Join(dir, name))
			if len(suggestions) >= 3 {
				break
			}
		}
	}

	return suggestions
}
