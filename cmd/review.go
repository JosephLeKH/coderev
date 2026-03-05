package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"

	"github.com/JosephLeKH/coderev/internal/bedrock"
	"github.com/JosephLeKH/coderev/internal/config"
	"github.com/JosephLeKH/coderev/internal/git"
	"github.com/JosephLeKH/coderev/internal/output"
	"github.com/JosephLeKH/coderev/internal/reviewer"
	"github.com/spf13/cobra"
)

var (
	flagTarget string
	flagJSON   bool
	flagMock   bool
	flagRegion string
)

var reviewCmd = &cobra.Command{
	Use:   "review",
	Short: "Review staged changes (or a target revision) via AWS Bedrock",
	RunE:  runReview,
}

func init() {
	reviewCmd.Flags().StringVar(&flagTarget, "target", "", "Git revision to diff against (e.g. HEAD~1, main)")
	reviewCmd.Flags().BoolVar(&flagJSON, "json", false, "Output results as JSON")
	reviewCmd.Flags().BoolVar(&flagMock, "mock", false, "Use mock Bedrock client (for testing without AWS credentials)")
	reviewCmd.Flags().StringVar(&flagRegion, "region", "", "AWS region override (e.g. us-west-2)")
	rootCmd.AddCommand(reviewCmd)
}

func runReview(cmd *cobra.Command, args []string) error {
	// Cancel in-flight requests on Ctrl-C.
	ctx, cancel := signal.NotifyContext(cmd.Context(), os.Interrupt)
	defer cancel()

	// Load config from repo root (current directory).
	cfg, err := config.LoadConfig(".")
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// Get diff.
	chunks, err := git.GetDiff(flagTarget)
	if err != nil {
		if errors.Is(err, git.ErrNotGitRepo) {
			fmt.Fprintln(os.Stderr, "Error: not a git repository. Run coderev from inside a git repo.")
			return nil
		}
		if errors.Is(err, git.ErrNothingToReview) {
			fmt.Fprintln(os.Stderr, "Nothing to review — stage changes with `git add` or pass --target.")
			return nil
		}
		return fmt.Errorf("getting diff: %w", err)
	}

	// Apply ignore patterns from config.
	chunks = git.FilterChunks(chunks, cfg.Ignore)
	if len(chunks) == 0 {
		fmt.Println("All changed files are ignored by .coderev.yaml — nothing to review.")
		return nil
	}

	// Truncate oversized chunks and warn.
	for i, chunk := range chunks {
		truncated, warning := git.TruncateChunk(chunk)
		chunks[i] = truncated
		if warning != "" {
			fmt.Fprintln(os.Stderr, warning)
		}
	}

	// Build the Bedrock client.
	var client bedrock.Client
	if flagMock {
		client = &bedrock.MockClient{Response: "[NITPICK] L1 Mock review: looks good"}
	} else {
		client, err = bedrock.NewRealClient(ctx, cfg.Model, flagRegion)
		if err != nil {
			return fmt.Errorf("initializing Bedrock client: %w", err)
		}
	}

	// Print progress header.
	fmt.Fprintf(os.Stderr, "Reviewing %d file(s) with model %s...\n", len(chunks), cfg.Model)

	// Run parallel review (tokens stream live to terminal as they arrive).
	results, err := reviewer.RunReview(ctx, chunks, cfg, client)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			fmt.Fprintln(os.Stderr, "\nReview cancelled.")
			return nil
		}
		return fmt.Errorf("review failed: %w", err)
	}

	// Print structured summary (or JSON).
	if flagJSON {
		out, err := output.FormatJSON(results)
		if err != nil {
			return err
		}
		fmt.Println(out)
	} else {
		fmt.Print(output.FormatTerminal(results))
	}

	return nil
}
