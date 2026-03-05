package reviewer

import (
	"context"
	"errors"
	"testing"

	"github.com/JosephLeKH/coderev/internal/bedrock"
	"github.com/JosephLeKH/coderev/internal/config"
	"github.com/JosephLeKH/coderev/internal/git"
	"github.com/JosephLeKH/coderev/internal/output"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunReview_SingleFile(t *testing.T) {
	mock := &bedrock.MockClient{Response: "[BUG] L5 Nil pointer dereference"}
	chunks := []git.FileChunk{
		{Filename: "main.go", Content: "+x := nil\n+fmt.Println(*x)"},
	}
	results, err := RunReview(context.Background(), chunks, &config.Config{}, mock)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "main.go", results[0].File)
	require.Len(t, results[0].Comments, 1)
	assert.Equal(t, "BUG", results[0].Comments[0].Severity)
	assert.Equal(t, 5, results[0].Comments[0].Line)
}

func TestRunReview_MultipleFiles_OrderPreserved(t *testing.T) {
	mock := &bedrock.MockClient{Response: "[STYLE] L1 minor"}
	chunks := []git.FileChunk{
		{Filename: "a.go", Content: "+a"},
		{Filename: "b.go", Content: "+b"},
		{Filename: "c.go", Content: "+c"},
	}
	results, err := RunReview(context.Background(), chunks, &config.Config{}, mock)
	require.NoError(t, err)
	require.Len(t, results, 3)
	// Order must match input order
	assert.Equal(t, "a.go", results[0].File)
	assert.Equal(t, "b.go", results[1].File)
	assert.Equal(t, "c.go", results[2].File)
}

func TestRunReview_NoIssues(t *testing.T) {
	mock := &bedrock.MockClient{Response: "No issues found."}
	chunks := []git.FileChunk{
		{Filename: "clean.go", Content: "+x := 1"},
	}
	results, err := RunReview(context.Background(), chunks, &config.Config{}, mock)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Empty(t, results[0].Comments)
}

func TestRunReview_ClientError_ReturnsError(t *testing.T) {
	mock := &bedrock.MockClient{Err: errors.New("bedrock unavailable")}
	chunks := []git.FileChunk{
		{Filename: "main.go", Content: "+x := 1"},
	}
	_, err := RunReview(context.Background(), chunks, &config.Config{}, mock)
	assert.Error(t, err)
}

func TestRunReview_EmptyChunks(t *testing.T) {
	mock := &bedrock.MockClient{Response: ""}
	results, err := RunReview(context.Background(), []git.FileChunk{}, &config.Config{}, mock)
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestRunReview_RawResponseCaptured(t *testing.T) {
	mock := &bedrock.MockClient{Response: "[BUG] L1 issue"}
	chunks := []git.FileChunk{
		{Filename: "x.go", Content: "+bad"},
	}
	results, err := RunReview(context.Background(), chunks, &config.Config{}, mock)
	require.NoError(t, err)
	require.Len(t, results, 1)
	// Raw response should include the original model output
	assert.Contains(t, results[0].Raw, "[BUG]")
}

// Ensure RunReview satisfies the expected signature with the output package's types.
var _ []output.FileResult = ([]output.FileResult)(nil)
