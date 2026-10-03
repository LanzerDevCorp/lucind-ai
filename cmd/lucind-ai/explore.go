package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/LanzerDevCorp/lucind-ai/internal/explorefan"
	"github.com/LanzerDevCorp/lucind-ai/internal/lane"
	lucindrun "github.com/LanzerDevCorp/lucind-ai/internal/run"
)

type scopeList []string

func (s *scopeList) String() string {
	if s == nil {
		return ""
	}
	return fmt.Sprint([]string(*s))
}

func (s *scopeList) Set(value string) error {
	*s = append(*s, value)
	return nil
}

func exploreRunDir(id string) (string, error) {
	if xdg := os.Getenv("XDG_STATE_HOME"); xdg != "" {
		return filepath.Join(xdg, "lucind-ai", "explore", id), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve user home directory: %w", err)
	}
	return filepath.Join(home, ".local", "state", "lucind-ai", "explore", id), nil
}

func exploreDispatch(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("explore", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprintln(stderr, usage)
		fs.PrintDefaults()
	}

	objective := fs.String("objective", "", "exploration objective (required)")
	var scopeFlags scopeList
	fs.Var(&scopeFlags, "scope", "scope path (repeatable)")
	idFlag := fs.String("id", "", "run prefix / identifier (default: explore-<UTC timestamp>)")
	timeout := fs.Duration("timeout", defaultTimeout, "wall clock budget granted to each lane")

	if err := fs.Parse(args); err != nil {
		return 1
	}

	prefix := *idFlag
	if prefix == "" {
		prefix = fmt.Sprintf("explore-%s", time.Now().UTC().Format("20060102150405"))
	}

	spec := explorefan.Spec{
		Prefix:    prefix,
		Objective: *objective,
		Scope:     scopeFlags,
	}

	if err := explorefan.Validate(spec); err != nil {
		fmt.Fprintf(stderr, "lucind-ai: %v\n", err)
		return 1
	}

	runDir, err := exploreRunDir(spec.Prefix)
	if err != nil {
		fmt.Fprintf(stderr, "lucind-ai: %v\n", err)
		return 1
	}
	if err := os.MkdirAll(runDir, 0o700); err != nil {
		fmt.Fprintf(stderr, "lucind-ai: create explore directory: %v\n", err)
		return 1
	}

	lensPacketPaths := make([]string, len(explorefan.Lenses))
	for i, l := range explorefan.Lenses {
		packetID, md := explorefan.LensPacket(spec, l)
		packetPath := filepath.Join(runDir, packetID+".md")
		if err := os.WriteFile(packetPath, md, 0o600); err != nil {
			fmt.Fprintf(stderr, "lucind-ai: write packet %q: %v\n", packetPath, err)
			return 1
		}
		lensPacketPaths[i] = packetPath
	}

	primaryRoot, err := resolvePrimaryRoot(ctx)
	if err != nil {
		fmt.Fprintf(stderr, "lucind-ai: resolve primary repository root: %v\n", err)
		return 1
	}
	headSHA, err := resolveAdmissionRefSHA(ctx, primaryRoot, "HEAD")
	if err != nil {
		fmt.Fprintf(stderr, "lucind-ai: resolve HEAD commit: %v\n", err)
		return 1
	}

	lensBatch, _ := executePacketBatch(ctx, dispatchBatchConfig{
		packetPaths:       lensPacketPaths,
		legacyMain:        true,
		expectedParentSHA: headSHA,
		minQuota:          defaultMinQuota,
		maxParallel:       lucindrun.DefaultMaxParallelLanes,
		timeout:           *timeout,
	}, stdout, stderr)

	reportByLane := make(map[string]lucindrun.Report, len(lensBatch.Lanes))
	for _, r := range lensBatch.Lanes {
		reportByLane[r.LaneID] = r
	}

	outputs := make([]explorefan.LensOutput, len(explorefan.Lenses))
	allFailed := true
	for i, l := range explorefan.Lenses {
		laneID := fmt.Sprintf("%s-%s", spec.Prefix, string(l))
		r, ok := reportByLane[laneID]
		if ok && r.Status == lane.Done {
			allFailed = false
			outputs[i] = explorefan.LensOutput{
				Lens:    l,
				OK:      true,
				Summary: r.Envelope.Summary,
			}
		} else {
			diag := r.Diagnosis
			if diag == "" {
				diag = fmt.Sprintf("lane %s ended with status %s", laneID, r.Status)
			}
			outputs[i] = explorefan.LensOutput{
				Lens:    l,
				OK:      false,
				Failure: diag,
			}
		}
	}

	if allFailed {
		fmt.Fprintf(stderr, "lucind-ai: explore %s: all lens lanes failed:\n", spec.Prefix)
		for _, out := range outputs {
			fmt.Fprintf(stderr, "  %s: %s\n", out.Lens, out.Failure)
		}
		return 1
	}

	synthID, synthMD := explorefan.SynthesisPacket(spec, outputs)
	synthPath := filepath.Join(runDir, synthID+".md")
	if err := os.WriteFile(synthPath, synthMD, 0o600); err != nil {
		fmt.Fprintf(stderr, "lucind-ai: write synthesis packet: %v\n", err)
		return 1
	}

	synthBatch, _ := executePacketBatch(ctx, dispatchBatchConfig{
		packetPaths:       []string{synthPath},
		legacyMain:        true,
		expectedParentSHA: headSHA,
		minQuota:          defaultMinQuota,
		maxParallel:       1,
		timeout:           *timeout,
	}, stdout, stderr)

	var synthReport *lucindrun.Report
	for i := range synthBatch.Lanes {
		if synthBatch.Lanes[i].LaneID == synthID {
			synthReport = &synthBatch.Lanes[i]
			break
		}
	}

	if synthReport == nil || synthReport.Status != lane.Done {
		diag := "unknown error"
		if synthReport != nil && synthReport.Diagnosis != "" {
			diag = synthReport.Diagnosis
		} else if synthReport != nil {
			diag = fmt.Sprintf("lane ended with status %s", synthReport.Status)
		}
		fmt.Fprintf(stderr, "lucind-ai: explore %s: synthesis lane %s failed: %s\n", spec.Prefix, synthID, diag)
		return 1
	}

	handoff := synthReport.Envelope.Summary
	fmt.Fprintf(stdout, "explore %s: handoff from synthesis lane %s\n", spec.Prefix, synthID)
	fmt.Fprintln(stdout, handoff)

	handoffPath := filepath.Join(runDir, "handoff.md")
	if err := os.WriteFile(handoffPath, []byte(handoff+"\n"), 0o600); err != nil {
		fmt.Fprintf(stderr, "lucind-ai: write handoff.md: %v\n", err)
	}

	return 0
}
