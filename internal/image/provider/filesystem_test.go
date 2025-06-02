package provider

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

func TestFilesystem_Provide(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "ims-filesystem-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test files
	testFile := filepath.Join(tmpDir, "test.txt")
	testContent := "test content"
	if err := os.WriteFile(testFile, []byte(testContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create subdirectory with file
	subDir := filepath.Join(tmpDir, "subdir")
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatalf("Failed to create subdir: %v", err)
	}
	subFile := filepath.Join(subDir, "subfile.txt")
	if err := os.WriteFile(subFile, []byte("sub content"), 0644); err != nil {
		t.Fatalf("Failed to create sub file: %v", err)
	}

	fs := &Filesystem{Dir: http.Dir(tmpDir)}
	ctx := context.Background()

	tests := []struct {
		name        string
		filename    string
		expectError error
		expectData  string
	}{
		{
			name:        "existing file",
			filename:    "test.txt",
			expectError: nil,
			expectData:  testContent,
		},
		{
			name:        "file in subdirectory",
			filename:    "subdir/subfile.txt",
			expectError: nil,
			expectData:  "sub content",
		},
		{
			name:        "non-existent file",
			filename:    "nonexistent.txt",
			expectError: ErrNotFound,
		},
		{
			name:        "directory traversal attempt",
			filename:    "../../../etc/passwd",
			expectError: ErrNotFound, // http.Dir should block this
		},
		{
			name:        "absolute path attempt",
			filename:    "/etc/passwd",
			expectError: ErrNotFound, // http.Dir should block this
		},
		{
			name:        "empty filename",
			filename:    "",
			expectError: nil, // http.Dir.Open("") actually works, opens directory
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader, err := fs.Provide(ctx, tt.filename)

			if tt.expectError != nil {
				if err == nil {
					t.Errorf("Expected error %v, got nil", tt.expectError)
					if reader != nil {
						reader.Close()
					}
					return
				}
				if !errors.Is(err, tt.expectError) {
					t.Errorf("Expected error %v, got %v", tt.expectError, err)
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if reader == nil {
				t.Error("Expected reader, got nil")
				return
			}
			defer reader.Close()

			// Read the content only if we expect specific data
			if tt.expectData != "" {
				buf := make([]byte, len(tt.expectData))
				n, err := reader.Read(buf)
				if err != nil && err.Error() != "EOF" {
					t.Errorf("Error reading content: %v", err)
					return
				}

				if string(buf[:n]) != tt.expectData {
					t.Errorf("Expected content %q, got %q", tt.expectData, string(buf[:n]))
				}
			}
		})
	}
}

func TestFilesystem_ProvideContextCancellation(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "ims-filesystem-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	fs := &Filesystem{Dir: http.Dir(tmpDir)}

	// Create a context that's already cancelled
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// The filesystem provider doesn't actually check context cancellation
	// but this tests that it doesn't panic with a cancelled context
	reader, err := fs.Provide(ctx, "test.txt")
	if err != nil {
		t.Errorf("Unexpected error with cancelled context: %v", err)
		return
	}
	if reader != nil {
		reader.Close()
	}
}