package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---- GetDiff integration tests ----

// initGitRepo creates a minimal git repo in dir with a single initial commit.
func initGitRepo(t *testing.T, dir string) {
	t.Helper()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "git %v: %s", args, out)
	}
	run("init")
	run("config", "user.email", "test@example.com")
	run("config", "user.name", "Test")
	// Create an initial commit so HEAD exists.
	initial := filepath.Join(dir, "README.md")
	require.NoError(t, os.WriteFile(initial, []byte("# repo\n"), 0o644))
	run("add", "README.md")
	run("commit", "-m", "init")
}

func TestGetDiff_HappyPath(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	// Stage a new file.
	f := filepath.Join(dir, "main.go")
	require.NoError(t, os.WriteFile(f, []byte("package main\n"), 0o644))
	cmd := exec.Command("git", "add", "main.go")
	cmd.Dir = dir
	require.NoError(t, cmd.Run())

	t.Chdir(dir)

	chunks, err := GetDiff("") // staged diff
	require.NoError(t, err)
	require.NotEmpty(t, chunks)
	assert.Equal(t, "main.go", chunks[0].Filename)
	assert.Contains(t, chunks[0].Content, "package main")
}

func TestGetDiff_NothingToReview(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	t.Chdir(dir)

	_, err := GetDiff("") // no staged changes
	assert.ErrorIs(t, err, ErrNothingToReview)
}

func TestGetDiff_NotGitRepo(t *testing.T) {
	dir := t.TempDir() // plain directory, no .git

	t.Chdir(dir)

	_, err := GetDiff("")
	assert.ErrorIs(t, err, ErrNotGitRepo)
}

func TestParseDiff_SingleFile(t *testing.T) {
	input := `diff --git a/main.go b/main.go
index abc123..def456 100644
--- a/main.go
+++ b/main.go
@@ -1,5 +1,7 @@
 package main

+import "fmt"
+
 func main() {
-	println("hello")
+	fmt.Println("hello, world")
 }`

	chunks, err := ParseDiff(input)
	require.NoError(t, err)
	require.Len(t, chunks, 1)
	assert.Equal(t, "main.go", chunks[0].Filename)
	assert.Contains(t, chunks[0].Content, `fmt.Println`)
}

func TestParseDiff_MultipleFiles(t *testing.T) {
	input := `diff --git a/main.go b/main.go
index abc123..def456 100644
--- a/main.go
+++ b/main.go
@@ -1,3 +1,3 @@
 package main
-func old() {}
+func new() {}
diff --git a/util.go b/util.go
index 111..222 100644
--- a/util.go
+++ b/util.go
@@ -1,3 +1,3 @@
 package main
-func helper() {}
+func helper2() {}`

	chunks, err := ParseDiff(input)
	require.NoError(t, err)
	require.Len(t, chunks, 2)
	assert.Equal(t, "main.go", chunks[0].Filename)
	assert.Equal(t, "util.go", chunks[1].Filename)
}

func TestParseDiff_SkipsBinaryFiles(t *testing.T) {
	input := `diff --git a/image.png b/image.png
index abc..def 100644
Binary files a/image.png and b/image.png differ`

	chunks, err := ParseDiff(input)
	require.NoError(t, err)
	assert.Empty(t, chunks)
}

func TestParseDiff_Empty(t *testing.T) {
	chunks, err := ParseDiff("")
	require.NoError(t, err)
	assert.Empty(t, chunks)
}

func TestParseDiff_CountsChangedLines(t *testing.T) {
	input := `diff --git a/main.go b/main.go
index abc..def 100644
--- a/main.go
+++ b/main.go
@@ -1,2 +1,2 @@
-old line
+new line`

	chunks, err := ParseDiff(input)
	require.NoError(t, err)
	require.Len(t, chunks, 1)
	assert.Equal(t, 2, chunks[0].NumLines)
}

func TestParseDiff_NestedPath(t *testing.T) {
	input := `diff --git a/internal/git/diff.go b/internal/git/diff.go
index abc..def 100644
--- a/internal/git/diff.go
+++ b/internal/git/diff.go
@@ -1 +1 @@
-old
+new`

	chunks, err := ParseDiff(input)
	require.NoError(t, err)
	require.Len(t, chunks, 1)
	assert.Equal(t, "internal/git/diff.go", chunks[0].Filename)
}

func TestFilterChunks_Basic(t *testing.T) {
	chunks := []FileChunk{
		{Filename: "main.go"},
		{Filename: "go.sum"},
		{Filename: "vendor/lib.go"},
		{Filename: "api/client.go"},
	}
	result := FilterChunks(chunks, []string{"*.sum", "vendor/**"})
	assert.Len(t, result, 2)
	assert.Equal(t, "main.go", result[0].Filename)
	assert.Equal(t, "api/client.go", result[1].Filename)
}

func TestFilterChunks_NoPatterns(t *testing.T) {
	chunks := []FileChunk{{Filename: "main.go"}, {Filename: "util.go"}}
	result := FilterChunks(chunks, nil)
	assert.Len(t, result, 2)
}

func TestFilterChunks_TestFiles(t *testing.T) {
	chunks := []FileChunk{
		{Filename: "main.go"},
		{Filename: "main_test.go"},
		{Filename: "internal/foo_test.go"},
	}
	result := FilterChunks(chunks, []string{"*_test.go"})
	assert.Len(t, result, 1)
	assert.Equal(t, "main.go", result[0].Filename)
}

func TestEstimatedTokens(t *testing.T) {
	chunk := FileChunk{Content: "abcd"} // 4 chars = 1 token
	assert.Equal(t, 1, chunk.EstimatedTokens())

	chunk2 := FileChunk{Content: "abcde"} // 5 chars = 2 tokens (ceiling)
	assert.Equal(t, 2, chunk2.EstimatedTokens())
}

func TestTruncateChunk_BelowLimit(t *testing.T) {
	chunk := FileChunk{Filename: "f.go", Content: "short content"}
	result, warn := TruncateChunk(chunk)
	assert.Equal(t, chunk.Content, result.Content)
	assert.Empty(t, warn)
}

func TestTruncateChunk_AboveLimit(t *testing.T) {
	// Create content just over 8000 tokens (32001 chars)
	bigContent := make([]byte, 32001)
	for i := range bigContent {
		bigContent[i] = 'x'
	}
	chunk := FileChunk{Filename: "big.go", Content: string(bigContent)}
	result, warn := TruncateChunk(chunk)
	assert.Less(t, len(result.Content), len(chunk.Content))
	assert.Contains(t, warn, "big.go")
	assert.Contains(t, warn, "truncated")
}
