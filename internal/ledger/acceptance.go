package ledger

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"time"
)

var (
	ErrLaneCandidateNotFound       = errors.New("ledger: lane candidate not found")
	ErrAcceptanceReceiptNotFound   = errors.New("ledger: acceptance receipt not found")
	ErrImmutableAcceptanceEvidence = errors.New("ledger: acceptance evidence is immutable")
	ErrAcceptanceBindingMismatch   = errors.New("ledger: acceptance binding mismatch")
)

// LaneCandidate is the terminal dispatch identity frozen before acceptance.
type LaneCandidate struct {
	RunID, LaneID, PacketID, PacketDigest string
	PrimaryRoot, WorktreePath             string
	BaseCommit, BaseTree                  string
	CandidateCommit, CandidateTree        string
	AllowedPaths                          []string
	ResultPath                            string
	ResultJSON, ResultHash                string
	RecordedAt                            time.Time
}

// AcceptanceBinding is every identity value covered by a receipt hash.
type AcceptanceBinding struct {
	RunID, LaneID, PacketID, PacketDigest string
	BaseCommit, BaseTree                  string
	CandidateCommit, CandidateTree        string
	AllowedPathsHash, CheckPolicyHash     string
	EnvironmentHash                       string
}

// AcceptanceReceipt is immutable mechanical evidence for one exact binding.
type AcceptanceReceipt struct {
	ReceiptID, BindingHash, ResultHash, ChecksHash string
	Binding                                        AcceptanceBinding
	CreatedAt                                      time.Time
	Cleanup                                        string
}

// SetDoneCandidate atomically marks a lane done and freezes its candidate identity.
func (l *Ledger) SetDoneCandidate(ctx context.Context, candidate LaneCandidate) error {
	paths, err := json.Marshal(candidate.AllowedPaths)
	if err != nil {
		return fmt.Errorf("ledger: encode candidate allowed paths: %w", err)
	}
	tx, err := l.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("ledger: begin done candidate: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	res, err := tx.ExecContext(ctx, `UPDATE lanes SET status='done', ended_at=? WHERE run_id=? AND lane_id=?`,
		candidate.RecordedAt.UTC().Format(time.RFC3339Nano), candidate.RunID, candidate.LaneID)
	if err != nil {
		return fmt.Errorf("ledger: mark lane done: %w", err)
	}
	if n, _ := res.RowsAffected(); n != 1 {
		return ErrLaneUnknown
	}
	insert, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO lane_candidates
		(run_id,lane_id,packet_id,packet_digest,primary_root,worktree_path,base_commit,base_tree,candidate_commit,candidate_tree,allowed_paths,result_path,result_json,result_hash,recorded_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, candidate.RunID, candidate.LaneID, candidate.PacketID,
		candidate.PacketDigest, candidate.PrimaryRoot, candidate.WorktreePath, candidate.BaseCommit,
		candidate.BaseTree, candidate.CandidateCommit, candidate.CandidateTree, string(paths), candidate.ResultPath,
		candidate.ResultJSON, candidate.ResultHash,
		candidate.RecordedAt.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("ledger: insert done candidate: %w", err)
	}
	if inserted, rowsErr := insert.RowsAffected(); rowsErr != nil {
		return fmt.Errorf("ledger: inspect done candidate insert: %w", rowsErr)
	} else if inserted == 0 {
		got, getErr := getLaneCandidate(ctx, tx, candidate.RunID, candidate.LaneID)
		if getErr != nil {
			return getErr
		}
		if !reflect.DeepEqual(got, candidate) {
			return ErrImmutableAcceptanceEvidence
		}
	}
	return tx.Commit()
}

func (l *Ledger) GetLaneCandidate(ctx context.Context, runID, laneID string) (LaneCandidate, error) {
	return getLaneCandidate(ctx, l.db, runID, laneID)
}

type rowQueryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func getLaneCandidate(ctx context.Context, q rowQueryer, runID, laneID string) (LaneCandidate, error) {
	var c LaneCandidate
	var paths, recorded string
	err := q.QueryRowContext(ctx, `SELECT run_id,lane_id,packet_id,packet_digest,primary_root,worktree_path,
		base_commit,base_tree,candidate_commit,candidate_tree,allowed_paths,result_path,result_json,result_hash,recorded_at
		FROM lane_candidates WHERE run_id=? AND lane_id=?`, runID, laneID).Scan(
		&c.RunID, &c.LaneID, &c.PacketID, &c.PacketDigest, &c.PrimaryRoot, &c.WorktreePath,
		&c.BaseCommit, &c.BaseTree, &c.CandidateCommit, &c.CandidateTree, &paths, &c.ResultPath, &c.ResultJSON, &c.ResultHash, &recorded)
	if errors.Is(err, sql.ErrNoRows) {
		return LaneCandidate{}, ErrLaneCandidateNotFound
	}
	if err != nil {
		return LaneCandidate{}, fmt.Errorf("ledger: query lane candidate: %w", err)
	}
	if err := json.Unmarshal([]byte(paths), &c.AllowedPaths); err != nil {
		return LaneCandidate{}, fmt.Errorf("ledger: decode candidate allowed paths: %w", err)
	}
	c.RecordedAt, err = time.Parse(time.RFC3339Nano, recorded)
	return c, err
}

func (l *Ledger) FindAcceptanceReceipt(ctx context.Context, bindingHash string) (AcceptanceReceipt, error) {
	return findAcceptanceReceipt(ctx, l.db, bindingHash)
}

func findAcceptanceReceipt(ctx context.Context, q rowQueryer, hash string) (AcceptanceReceipt, error) {
	var r AcceptanceReceipt
	var created string
	err := q.QueryRowContext(ctx, `SELECT receipt_id,binding_hash,run_id,lane_id,packet_id,packet_digest,
		base_commit,base_tree,candidate_commit,candidate_tree,allowed_paths_hash,check_policy_hash,
		environment_hash,result_hash,checks_hash,cleanup,created_at FROM acceptance_receipts WHERE binding_hash=?`, hash).Scan(
		&r.ReceiptID, &r.BindingHash, &r.Binding.RunID, &r.Binding.LaneID, &r.Binding.PacketID, &r.Binding.PacketDigest,
		&r.Binding.BaseCommit, &r.Binding.BaseTree, &r.Binding.CandidateCommit, &r.Binding.CandidateTree,
		&r.Binding.AllowedPathsHash, &r.Binding.CheckPolicyHash, &r.Binding.EnvironmentHash,
		&r.ResultHash, &r.ChecksHash, &r.Cleanup, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return AcceptanceReceipt{}, ErrAcceptanceReceiptNotFound
	}
	if err != nil {
		return AcceptanceReceipt{}, fmt.Errorf("ledger: query acceptance receipt: %w", err)
	}
	r.CreatedAt, err = time.Parse(time.RFC3339Nano, created)
	return r, err
}

// InsertAcceptanceReceipt atomically inserts once or returns an exactly equal receipt.
func (l *Ledger) InsertAcceptanceReceipt(ctx context.Context, receipt AcceptanceReceipt) (AcceptanceReceipt, error) {
	tx, err := l.db.BeginTx(ctx, nil)
	if err != nil {
		return AcceptanceReceipt{}, err
	}
	defer tx.Rollback() //nolint:errcheck
	_, err = tx.ExecContext(ctx, `INSERT OR IGNORE INTO acceptance_receipts
		(receipt_id,binding_hash,run_id,lane_id,packet_id,packet_digest,base_commit,base_tree,candidate_commit,candidate_tree,
		allowed_paths_hash,check_policy_hash,environment_hash,result_hash,checks_hash,cleanup,created_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, receipt.ReceiptID, receipt.BindingHash,
		receipt.Binding.RunID, receipt.Binding.LaneID, receipt.Binding.PacketID, receipt.Binding.PacketDigest,
		receipt.Binding.BaseCommit, receipt.Binding.BaseTree, receipt.Binding.CandidateCommit, receipt.Binding.CandidateTree,
		receipt.Binding.AllowedPathsHash, receipt.Binding.CheckPolicyHash, receipt.Binding.EnvironmentHash,
		receipt.ResultHash, receipt.ChecksHash, receipt.Cleanup, receipt.CreatedAt.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return AcceptanceReceipt{}, fmt.Errorf("ledger: insert acceptance receipt: %w", err)
	}
	got, err := findAcceptanceReceipt(ctx, tx, receipt.BindingHash)
	if err != nil {
		return AcceptanceReceipt{}, ErrAcceptanceBindingMismatch
	}
	if got != receipt {
		return AcceptanceReceipt{}, ErrAcceptanceBindingMismatch
	}
	if err := tx.Commit(); err != nil {
		return AcceptanceReceipt{}, err
	}
	return got, nil
}
