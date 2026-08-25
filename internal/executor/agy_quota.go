package executor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// geminiFiveHourBucketID is the bucket id agy's `/usage` response uses for
// the rolling 5-hour Gemini-family quota window -- verified against the real
// CLI (agy 1.1.20). AgyQuota gates on this bucket alone, not the weekly one:
// the 5-hour window is what a burst of concurrent lane dispatches within one
// wave can actually exhaust, while the weekly window reflects a longer-term
// budget a single wave's rotation decision has no bearing on.
const geminiFiveHourBucketID = "gemini-5h"

// AgyQuota gates a wave dispatch on the active Antigravity account's
// remaining 5-hour Gemini quota, rotating to whichever pooled account (via
// the agy-pool script) has the most quota left when the active one falls
// below the caller's threshold. It exists because lucind-ai's agy executor
// bills against a shared Google account pool with no per-request quota
// signal -- the only place that number is exposed is `agy --print "/usage"
// --output-format json`, a free local slash command (it costs no model
// tokens: num_turns is always 0 for it).
type AgyQuota struct {
	// AgyBinary is the agy executable to run. Defaults to "agy" (resolved
	// via PATH) when empty, but tests override it to point at a stub script
	// instead of spending real quota against the real CLI.
	AgyBinary string
	// AgyPoolBinary is the agy-pool script to run. Defaults to "agy-pool"
	// (resolved via PATH) when empty, but tests override it to point at a
	// stub script.
	AgyPoolBinary string
}

// agyUsageResponse mirrors the subset of `agy --print "/usage"
// --output-format json`'s response this package reads.
type agyUsageResponse struct {
	Command struct {
		Data struct {
			Groups []struct {
				Buckets []struct {
					ID                string  `json:"id"`
					RemainingFraction float64 `json:"remaining_fraction"`
				} `json:"buckets"`
			} `json:"groups"`
		} `json:"data"`
	} `json:"command"`
}

// Ensure checks the active pooled account's remaining gemini-5h quota
// fraction before a wave dispatches. If it is at or above minFraction,
// Ensure returns nil and the active account is left untouched. If it is
// below minFraction, Ensure asks agy-pool for whichever pooled account
// currently has the most gemini-5h quota remaining -- agy-pool checks the
// active account live and every other saved account from its own cache, see
// scripts/agy-pool's `usage`/`best` subcommands -- and switches to it via
// `agy-pool use`. If no pooled account clears minFraction, Ensure returns an
// error and the caller must not dispatch the wave.
func (q AgyQuota) Ensure(ctx context.Context, minFraction float64) error {
	agyBin := q.AgyBinary
	if agyBin == "" {
		agyBin = "agy"
	}
	poolBin := q.AgyPoolBinary
	if poolBin == "" {
		poolBin = "agy-pool"
	}

	fraction, err := activeFiveHourFraction(ctx, agyBin)
	if err != nil {
		return fmt.Errorf("agy quota: read active account's usage: %w", err)
	}

	if fraction >= minFraction {
		return nil
	}

	best, err := bestPooledAccount(ctx, poolBin, minFraction)
	if err != nil {
		return fmt.Errorf("agy quota: active account is at %.0f%% gemini-5h quota (below the %.0f%% minimum) and no pooled account clears the minimum: %w", fraction*100, minFraction*100, err)
	}

	if err := useAccount(ctx, poolBin, best); err != nil {
		return fmt.Errorf("agy quota: switch to pooled account %q: %w", best, err)
	}

	return nil
}

func activeFiveHourFraction(ctx context.Context, agyBin string) (float64, error) {
	cmd := exec.CommandContext(ctx, agyBin, "--print", "/usage", "--output-format", "json")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return 0, fmt.Errorf("run %s --print /usage: %w (stderr: %s)", agyBin, err, strings.TrimSpace(stderr.String()))
	}
	return ParseGeminiFiveHourFraction(stdout.Bytes())
}

// ParseGeminiFiveHourFraction extracts the gemini-5h bucket's remaining
// fraction from a raw `agy --print "/usage" --output-format json` response.
func ParseGeminiFiveHourFraction(raw []byte) (float64, error) {
	var resp agyUsageResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return 0, fmt.Errorf("decode /usage response: %w", err)
	}
	for _, group := range resp.Command.Data.Groups {
		for _, bucket := range group.Buckets {
			if bucket.ID == geminiFiveHourBucketID {
				return bucket.RemainingFraction, nil
			}
		}
	}
	return 0, fmt.Errorf("no %q bucket in /usage response", geminiFiveHourBucketID)
}

func bestPooledAccount(ctx context.Context, poolBin string, minFraction float64) (string, error) {
	cmd := exec.CommandContext(ctx, poolBin, "best", strconv.FormatFloat(minFraction, 'g', -1, 64))
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%w (stderr: %s)", err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(stdout.String()), nil
}

func useAccount(ctx context.Context, poolBin, email string) error {
	cmd := exec.CommandContext(ctx, poolBin, "use", email)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%w (stderr: %s)", err, strings.TrimSpace(stderr.String()))
	}
	return nil
}
