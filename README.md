# Softika

Analyze public GitHub repositories for fake traction. Detects artificial stars, inflated metrics, and suspicious engagement patterns using signals derived from the [CMU ICSE 2026 StarScout study](https://awesomeagents.ai/news/github-fake-stars-investigation/).

## Install

```bash
go install github.com/fmiskovic/softika/cmd/softika@latest
```

Or build from source:

```bash
git clone https://github.com/fmiskovic/softika.git
cd softika
go build -o softika ./cmd/softika/
```

## Usage

```bash
# Set a GitHub token (recommended — unauthenticated is limited to 60 req/hr)
export GITHUB_TOKEN=ghp_xxx

# Analyze a repo
softika analyze owner/repo

# Full GitHub URLs work too
softika analyze https://github.com/spf13/cobra

# JSON output for scripting
softika analyze owner/repo --json

# Pass token inline
softika analyze owner/repo --token ghp_xxx
```

## Example Output

```
╔══════════════════════════════════════════════════╗
║  SOFTIKA — Traction Trust Analysis               ║
╠══════════════════════════════════════════════════╣
║  Repo: spf13/cobra                               ║
╚══════════════════════════════════════════════════╝

  Stars: 43746  |  Forks: 3125  |  Watchers: 360  |  Issues: 1242

  TRUST SCORE: 65.6 / 100  [#############-------]
  VERDICT:     MODERATE - Some suspicious signals

  Stargazer Sample: 54 of 43746 stars (1.1%)
  Ghost Accounts:       1 (1.9%)
  Zero-Follower:        4 (7.4%)
  Dormant Accounts:     0
  Organic Accounts:     52

  SIGNAL                              RAW    SCORE   WEIGHT
  ──────────────────────────────────────────────────────
  fork_to_star_ratio               0.0714    42.9      25%
  ghost_account_pct                0.0185    96.6      20%
  star_velocity                    0.0000    50.0      15%
  watcher_to_star_ratio            0.0082    82.6      10%
  median_account_age_days        4561.3903    97.9      10%
  zero_follower_pct                0.0741   100.0      10%
  issue_to_star_ratio              0.0284    80.0      10%
```

## Detection Signals

Softika computes 7 weighted signals to produce a trust score (0-100):

| Signal | Weight | Organic Range | Suspicious |
|--------|--------|---------------|------------|
| Fork-to-star ratio | 25% | 0.09 - 0.23 | < 0.05 |
| Ghost account % | 20% | 1 - 6% | > 20% |
| Star velocity spikes | 15% | gradual growth | step-function jumps |
| Watcher-to-star ratio | 10% | 0.005 - 0.03 | < 0.001 |
| Median account age | 10% | > 2500 days | < 1000 days |
| Zero-follower % | 10% | 10 - 20% | > 50% |
| Issue-to-star ratio | 10% | > 0.01 | < 0.002 |

### Verdicts

- **LIKELY ORGANIC** (70-100): Traction appears genuine
- **MODERATE** (40-70): Some suspicious signals, warrants closer inspection
- **HIGH RISK** (0-40): Likely artificial traction

### Stargazer Classification

Each sampled stargazer is classified into one of four categories:

- **Ghost**: Zero public repos and zero followers
- **Dormant**: Account older than 1 year with no meaningful activity
- **Low-signal**: Fewer than 3 repos and fewer than 5 followers
- **Organic**: Meaningful activity history

## How It Works

1. Fetches repository metadata (stars, forks, watchers, issues)
2. Samples stargazers with timestamps (full scan for repos under 5K stars, stratified random sampling for larger repos)
3. Profiles each sampled stargazer (account age, repos, followers)
4. Classifies stargazers and computes aggregate statistics
5. Calculates 7 detection signals against known organic/suspicious baselines
6. Produces a weighted composite trust score and verdict

## GitHub Token

A personal access token significantly improves the experience:

- **Without token**: 60 requests/hour (enough to analyze small repos)
- **With token**: 5,000 requests/hour (handles large repos with thousands of stargazers)

Create a token at https://github.com/settings/tokens — no special scopes needed for public repo analysis.

## Project Structure

```
softika/
├── cmd/softika/main.go              # CLI (Cobra)
├── internal/
│   ├── model/types.go               # Domain types
│   ├── github/client.go             # GitHub API client wrapper
│   └── analyzer/
│       ├── stargazer.go             # Profiler and classifier
│       ├── signals.go               # Signal calculators
│       ├── scorer.go                # Composite scoring
│       └── repo.go                  # Analysis orchestrator
└── go.mod
```

## Limitations

- Unauthenticated API calls are severely rate-limited — always use a token
- Star velocity detection requires starred_at timestamps (available via the GitHub API's `application/vnd.github.star+json` media type)
- For repos with 100K+ stars, only a statistical sample of stargazers is profiled
- Viral events (HN front page, trending on Twitter) can produce legitimate star spikes that look similar to bought stars — always consider context
- This tool provides probabilistic analysis, not definitive proof of manipulation

## License

AGPL-3.0 — see [LICENSE](LICENSE)
