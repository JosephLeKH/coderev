package reviewer

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/JosephLeKH/coderev/internal/bedrock"
	"github.com/JosephLeKH/coderev/internal/config"
	"github.com/JosephLeKH/coderev/internal/git"
	"github.com/JosephLeKH/coderev/internal/output"
	"github.com/JosephLeKH/coderev/internal/prompt"
)

// RunReview fans out one goroutine per file chunk, collects results in input order.
// All Bedrock calls run in parallel; terminal output is serialized via a mutex so
// tokens from different files don't interleave.
func RunReview(ctx context.Context, chunks []git.FileChunk, cfg *config.Config, client bedrock.Client) ([]output.FileResult, error) {
	if len(chunks) == 0 {
		return nil, nil
	}

	type result struct {
		fileResult output.FileResult
		err        error
	}

	results := make([]result, len(chunks))
	var wg sync.WaitGroup

	// printMu serializes all terminal writes across goroutines.
	var printMu sync.Mutex

	for i, chunk := range chunks {
		wg.Add(1)
		go func(idx int, fc git.FileChunk) {
			defer wg.Done()

			p := prompt.BuildPrompt(fc, cfg)
			var rawBuilder strings.Builder

			// Print the file header, then stream tokens live as they arrive.
			printMu.Lock()
			fmt.Fprintf(os.Stderr, "\n\033[1m▶ %s\033[0m\n", fc.Filename)
			printMu.Unlock()

			err := client.ReviewFile(ctx, p, func(token string) {
				rawBuilder.WriteString(token)
				printMu.Lock()
				fmt.Fprint(os.Stderr, token)
				printMu.Unlock()
			})
			// Ensure we end on a newline after streaming.
			printMu.Lock()
			fmt.Println()
			printMu.Unlock()

			if err != nil {
				results[idx] = result{err: fmt.Errorf("reviewing %s: %w", fc.Filename, err)}
				return
			}

			raw := rawBuilder.String()
			results[idx] = result{
				fileResult: output.FileResult{
					File:     fc.Filename,
					Comments: output.ParseComments(fc.Filename, raw),
					Raw:      raw,
				},
			}
		}(i, chunk)
	}

	wg.Wait()

	// Collect in order; return first error encountered.
	fileResults := make([]output.FileResult, 0, len(chunks))
	for _, r := range results {
		if r.err != nil {
			return nil, r.err
		}
		fileResults = append(fileResults, r.fileResult)
	}
	return fileResults, nil
}
