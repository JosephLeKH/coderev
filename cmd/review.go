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
	"github.com/JosephLeKH/coderev/internal/github"
	"github.com/JosephLeKH/coderev/internal/output"
	"github.com/JosephLeKH/coderev/internal/reviewer"
	"github.com/spf13/cobra"
)

var (
	flagTarget string
	flagJSON   bool
	flagMock   bool
	flagRegion string
	flagPost   bool
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
	reviewCmd.Flags().BoolVar(&flagPost, "post", false, "Post review comments to the open GitHub PR (requires GITHUB_TOKEN)")
	rootCmd.AddCommand(reviewCmd)
}

func runReview(cmd *cobra.Command, args []string) error {
	// Cancel in-flight Bedrock requests on Ctrl-C.
	ctx, cancel := signal.NotifyContext(cmd.Context(), os.Interrupt)
	defer cancel()

	cfg, err := config.LoadConfig(".")
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

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

	chunks = git.FilterChunks(chunks, cfg.Ignore)
	if len(chunks) == 0 {
		fmt.Println("All changed files are ignored by .coderev.yaml — nothing to review.")
		return nil
	}

	for i, chunk := range chunks {
		truncated, warning := git.TruncateChunk(chunk)
		chunks[i] = truncated
		if warning != "" {
			fmt.Fprintln(os.Stderr, warning)
		}
	}

	var client bedrock.Client
	if flagMock {
		client = &bedrock.MockClient{Response: "[NITPICK] L1 Mock review: looks good"}
	} else {
		client, err = bedrock.NewRealClient(ctx, cfg.Model, flagRegion)
		if err != nil {
			return fmt.Errorf("initializing Bedrock client: %w", err)
		}
	}

	fmt.Fprintf(os.Stderr, "Reviewing %d file(s) with model %s...\n", len(chunks), cfg.Model)

	// Tokens stream live to stderr as each file is reviewed in parallel.
	results, err := reviewer.RunReview(ctx, chunks, cfg, client)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			fmt.Fprintln(os.Stderr, "\nReview cancelled.")
			return nil
		}
		return fmt.Errorf("review failed: %w", err)
	}

	if flagJSON {
		out, err := output.FormatJSON(results)
		if err != nil {
			return err
		}
		fmt.Println(out)
	} else {
		fmt.Print(output.FormatTerminal(results))
	}

	if flagPost {
		fmt.Fprintln(os.Stderr, "Posting review to GitHub PR...")
		if err := github.PostReview(ctx, results, chunks); err != nil {
			if errors.Is(err, github.ErrNoPR) {
				fmt.Fprintln(os.Stderr, "No open PR found for current branch — skipping GitHub post.")
			} else {
				return fmt.Errorf("posting GitHub review: %w", err)
			}
		} else {
			fmt.Fprintln(os.Stderr, "Review posted successfully.")
		}
	}

	// Exit 1 if any issues were found — useful for git hooks and CI pipelines.
	for _, r := range results {
		if len(r.Comments) > 0 {
			os.Exit(1)
		}
	}

	return nil
}
