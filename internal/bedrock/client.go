package bedrock

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"
)

// Client is the interface for reviewing a file diff via an LLM.
type Client interface {
	ReviewFile(ctx context.Context, prompt string, onToken func(string)) error
}

// ─── Real Bedrock Client ──────────────────────────────────────────────────────

// RealClient sends requests to AWS Bedrock using InvokeModelWithResponseStream.
type RealClient struct {
	client  *bedrockruntime.Client
	modelID string
}

// NewRealClient creates a RealClient using the AWS default credential chain.
// region overrides the configured region when non-empty.
// Returns a clear error if credentials are not configured.
func NewRealClient(ctx context.Context, modelID, region string) (*RealClient, error) {
	opts := []func(*awsconfig.LoadOptions) error{}
	if region != "" {
		opts = append(opts, awsconfig.WithRegion(region))
	}
	cfg, err := awsconfig.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf(
			"loading AWS config: %w. See: https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-files.html",
			err,
		)
	}
	return &RealClient{
		client:  bedrockruntime.NewFromConfig(cfg),
		modelID: modelID,
	}, nil
}

type claudeRequest struct {
	AnthropicVersion string    `json:"anthropic_version"`
	Messages         []message `json:"messages"`
	MaxTokens        int       `json:"max_tokens"`
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type streamDelta struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type streamEvent struct {
	Type  string       `json:"type"`
	Delta *streamDelta `json:"delta,omitempty"`
}

// ReviewFile sends prompt to Bedrock and calls onToken for each streamed token.
// Retries with exponential backoff on throttling errors (up to 3 retries).
func (c *RealClient) ReviewFile(ctx context.Context, prompt string, onToken func(string)) error {
	body, err := json.Marshal(claudeRequest{
		AnthropicVersion: "bedrock-2023-05-31",
		Messages:         []message{{Role: "user", Content: prompt}},
		MaxTokens:        4096,
	})
	if err != nil {
		return fmt.Errorf("marshaling request: %w", err)
	}

	const maxRetries = 3
	backoff := time.Second

	for attempt := 0; attempt <= maxRetries; attempt++ {
		err = c.invokeStream(ctx, body, onToken)
		if err == nil {
			return nil
		}
		if !isThrottlingError(err) || attempt == maxRetries {
			return err
		}
		fmt.Printf("\nRate limited by Bedrock — retrying in %s...\n", backoff)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}
		backoff *= 2
	}
	return err
}

func (c *RealClient) invokeStream(ctx context.Context, body []byte, onToken func(string)) error {
	output, err := c.client.InvokeModelWithResponseStream(ctx, &bedrockruntime.InvokeModelWithResponseStreamInput{
		ModelId:     aws.String(c.modelID),
		ContentType: aws.String("application/json"),
		Body:        body,
	})
	if err != nil {
		return fmt.Errorf("invoking model %s: %w", c.modelID, err)
	}

	stream := output.GetStream()
	defer stream.Close()

	for event := range stream.Events() {
		chunk, ok := event.(*types.ResponseStreamMemberChunk)
		if !ok {
			continue
		}
		var evt streamEvent
		if err := json.Unmarshal(chunk.Value.Bytes, &evt); err != nil {
			continue
		}
		if evt.Type == "content_block_delta" && evt.Delta != nil && evt.Delta.Type == "text_delta" {
			onToken(evt.Delta.Text)
		}
	}
	return stream.Err()
}

func isThrottlingError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "throttling") ||
		strings.Contains(msg, "rate") ||
		strings.Contains(msg, "too many requests")
}

// ─── Mock Client ──────────────────────────────────────────────────────────────

// MockClient returns canned responses without any AWS calls. For testing only.
type MockClient struct {
	// Response is the full text delivered token-by-token (word by word).
	Response string
	// Err is returned instead of streaming a response.
	Err error
}

// ReviewFile simulates streaming by calling onToken for each word in Response.
func (m *MockClient) ReviewFile(_ context.Context, _ string, onToken func(string)) error {
	if m.Err != nil {
		return m.Err
	}
	words := strings.Fields(m.Response)
	for i, word := range words {
		if i > 0 {
			onToken(" ")
		}
		onToken(word)
	}
	return nil
}
