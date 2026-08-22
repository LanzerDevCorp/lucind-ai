package serve

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/LanzerDevCorp/lucind-ai/internal/worktree"
)

// WorktreeStatus is the read-only JSON payload for one lane worktree.
type WorktreeStatus struct {
	Path      string `json:"path"`
	Branch    string `json:"branch"`
	LaneID    string `json:"lane_id"`
	DiskBytes int64  `json:"disk_bytes"`
	Stale     bool   `json:"stale"`
}

// ListWorktrees reports worktrees recorded by the ledger without running Git
// or changing either repository or worktree state.
func (m *Model) ListWorktrees(ctx context.Context) ([]WorktreeStatus, error) {
	runs, err := m.ledger.ListRuns(ctx)
	if err != nil {
		return nil, fmt.Errorf("serve: list worktree runs: %w", err)
	}

	var statuses []WorktreeStatus
	seen := make(map[string]struct{})
	for _, run := range runs {
		lanes, err := m.ledger.Lanes(ctx, run.RunID)
		if err != nil {
			return nil, fmt.Errorf("serve: list worktree lanes for run %q: %w", run.RunID, err)
		}
		for _, lane := range lanes {
			if lane.WorktreePath == "" {
				continue
			}
			if _, ok := seen[lane.WorktreePath]; ok {
				continue
			}

			bytes, err := worktreeDiskBytes(ctx, lane.WorktreePath)
			if err != nil {
				return nil, err
			}
			statuses = append(statuses, WorktreeStatus{
				Path:      lane.WorktreePath,
				Branch:    worktree.BranchFor(lane.LaneID),
				LaneID:    lane.LaneID,
				DiskBytes: bytes,
				Stale:     !worktree.IsLinkedWorktree(lane.WorktreePath),
			})
			seen[lane.WorktreePath] = struct{}{}
		}
	}
	return statuses, nil
}

func worktreeDiskBytes(ctx context.Context, root string) (int64, error) {
	var total int64
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) && path == root {
				return nil
			}
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		total += info.Size()
		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("serve: measure worktree %q: %w", root, err)
	}
	return total, nil
}
