package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/fmiskovic/softika/internal/analyzer"
	ghclient "github.com/fmiskovic/softika/internal/github"
	"github.com/fmiskovic/softika/internal/model"
)

func main() {
	if err := rootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}

func rootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "softika",
		Short: "Analyze GitHub repos for fake traction",
		Long:  "Softika detects fake stars, inflated metrics, and artificial popularity on public GitHub repositories.",
	}

	cmd.AddCommand(analyzeCmd())
	return cmd
}

func analyzeCmd() *cobra.Command {
	var token string
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "analyze <owner/repo>",
		Short: "Analyze a GitHub repository's traction authenticity",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			owner, repo, err := analyzer.ParseRepo(args[0])
			if err != nil {
				return err
			}

			if token == "" {
				token = os.Getenv("GITHUB_TOKEN")
			}
			if token == "" {
				fmt.Fprintln(os.Stderr, "Warning: no GITHUB_TOKEN set. Rate limits will be very low (60 req/hr).")
				fmt.Fprintln(os.Stderr, "Set GITHUB_TOKEN or use --token to increase to 5,000 req/hr.")
			}

			client := ghclient.NewClient(token)
			a := analyzer.NewRepoAnalyzer(client)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()

			progress := func(msg string) {
				if !jsonOutput {
					fmt.Fprintf(os.Stderr, "  %s\n", msg)
				}
			}

			if !jsonOutput {
				fmt.Fprintf(os.Stderr, "\nAnalyzing %s/%s...\n\n", owner, repo)
			}

			result, err := a.Analyze(ctx, owner, repo, progress)
			if err != nil {
				return fmt.Errorf("analysis failed: %w", err)
			}

			if jsonOutput {
				return outputJSON(result)
			}
			return outputHuman(result)
		},
	}

	cmd.Flags().StringVar(&token, "token", "", "GitHub personal access token")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output raw JSON")

	return cmd
}

func outputJSON(result model.AnalysisResult) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(result)
}

func outputHuman(result model.AnalysisResult) error {
	repo := result.Repo

	// Header
	fmt.Printf("\n╔══════════════════════════════════════════════════╗\n")
	fmt.Printf("║  SOFTIKA — Traction Trust Analysis               ║\n")
	fmt.Printf("╠══════════════════════════════════════════════════╣\n")
	fmt.Printf("║  Repo: %-41s ║\n", fmt.Sprintf("%s/%s", repo.Owner, repo.Name))
	fmt.Printf("╚══════════════════════════════════════════════════╝\n\n")

	// Quick stats
	fmt.Printf("  Stars: %d  |  Forks: %d  |  Watchers: %d  |  Issues: %d\n\n",
		repo.Stars, repo.Forks, repo.Watchers, repo.TotalIssues,
	)

	// Trust score
	scoreBar := renderBar(result.TrustScore)
	fmt.Printf("  TRUST SCORE: %.1f / 100  %s\n", result.TrustScore, scoreBar)
	fmt.Printf("  VERDICT:     %s\n\n", verdictDisplay(string(result.Verdict)))

	// Stargazer summary
	summary := result.StargazerSummary
	fmt.Printf("  Stargazer Sample: %d of %d stars (%.1f%%)\n",
		summary.SampleSize, summary.TotalStars,
		result.SampleRate*100,
	)

	ghostPct := safePctInt(summary.GhostCount, summary.SampleSize)
	zfPct := safePctInt(summary.ZeroFollowerCount, summary.SampleSize)
	fmt.Printf("  Ghost Accounts:       %d (%.1f%%)\n", summary.GhostCount, ghostPct)
	fmt.Printf("  Zero-Follower:        %d (%.1f%%)\n", summary.ZeroFollowerCount, zfPct)
	fmt.Printf("  Dormant Accounts:     %d\n", summary.DormantCount)
	fmt.Printf("  Organic Accounts:     %d\n\n", summary.OrganicCount)

	// Signal breakdown
	fmt.Printf("  %-30s %8s %8s %8s\n", "SIGNAL", "RAW", "SCORE", "WEIGHT")
	fmt.Printf("  %s\n", strings.Repeat("─", 55))

	for _, s := range result.Signals {
		fmt.Printf("  %-30s %8.4f %7.1f %7.0f%%\n",
			s.Name, s.RawValue, s.Score, s.Weight*100,
		)
	}

	fmt.Println()
	return nil
}

func safePctInt(num, denom int) float64 {
	if denom == 0 {
		return 0
	}
	return float64(num) / float64(denom) * 100
}

func renderBar(score float64) string {
	filled := int(score / 5)
	empty := 20 - filled
	if filled < 0 {
		filled = 0
	}
	if empty < 0 {
		empty = 0
	}
	return "[" + strings.Repeat("#", filled) + strings.Repeat("-", empty) + "]"
}

func verdictDisplay(v string) string {
	switch v {
	case "high_risk":
		return "HIGH RISK - Likely artificial traction"
	case "moderate":
		return "MODERATE - Some suspicious signals"
	case "likely_organic":
		return "LIKELY ORGANIC - Traction appears genuine"
	default:
		return v
	}
}
