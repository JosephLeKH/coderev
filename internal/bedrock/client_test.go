package bedrock

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMockClient_StreamsTokens(t *testing.T) {
	mock := &MockClient{Response: "[BUG] L5 Nil pointer dereference"}
	var collected strings.Builder
	err := mock.ReviewFile(context.Background(), "test prompt", func(token string) {
		collected.WriteString(token)
	})
	require.NoError(t, err)
	assert.Equal(t, "[BUG] L5 Nil pointer dereference", collected.String())
}

func TestMockClient_ReturnsError(t *testing.T) {
	mock := &MockClient{Err: errors.New("mock error")}
	err := mock.ReviewFile(context.Background(), "prompt", func(string) {})
	assert.EqualError(t, err, "mock error")
}

func TestMockClient_EmptyResponse(t *testing.T) {
	mock := &MockClient{Response: ""}
	var called bool
	err := mock.ReviewFile(context.Background(), "prompt", func(string) { called = true })
	require.NoError(t, err)
	assert.False(t, called)
}

func TestMockClient_MultilineResponse(t *testing.T) {
	mock := &MockClient{Response: "[BUG] L1 First\n[SECURITY] L2 Second"}
	var collected strings.Builder
	err := mock.ReviewFile(context.Background(), "prompt", func(token string) {
		collected.WriteString(token)
	})
	require.NoError(t, err)
	result := collected.String()
	assert.Contains(t, result, "[BUG]")
	assert.Contains(t, result, "[SECURITY]")
}

func TestIsThrottlingError(t *testing.T) {
	cases := []struct {
		err      error
		expected bool
	}{
		{errors.New("ThrottlingException: rate exceeded"), true},
		{errors.New("rate limit exceeded"), true},
		{errors.New("too many requests"), true},
		{errors.New("model not found"), false},
		{errors.New("validation error"), false},
		{nil, false},
	}
	for _, tc := range cases {
		assert.Equal(t, tc.expected, isThrottlingError(tc.err), "err: %v", tc.err)
	}
}

// MockClient satisfies the Client interface.
var _ Client = (*MockClient)(nil)
