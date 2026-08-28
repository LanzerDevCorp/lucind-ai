package ledger

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"time"
)

const (
	ShadowEvidencePersistFailed = "SHADOW_EVIDENCE_PERSIST_FAILED"
	ShadowFailureNone           = "none"
	ShadowStageBegin            = "begin"
	ShadowStageInsert           = "insert"
	ShadowStageCommit           = "commit"
)

// ShadowAttempt is one isolated observational specialist attempt. It is
// intentionally separate from lane candidates and acceptance receipts.
type ShadowAttempt struct {
	ID, RunID, LaneID, InputHash, SpecialistIdentity, FailureClass string
	AttemptIndex                                                   int
	Valid, Equivalent, ReplayStable                                bool
	DiffJSON, ManualDigest, SpecialistDigest                       string
	LatencyMS                                                      int64
	CreatedAt                                                      time.Time
}

type ShadowReview struct {
	AttemptID, Reviewer string
	ReviewMS            int64
	CreatedAt           time.Time
}

// ShadowPersistenceError exposes only the safe attempt and transaction
// stage. The underlying SQLite error is deliberately not included.
type ShadowPersistenceError struct {
	AttemptIndex int
	Stage        string
}

func (e *ShadowPersistenceError) Error() string {
	return fmt.Sprintf("%s: attempt %d stage %s", ShadowEvidencePersistFailed, e.AttemptIndex, e.Stage)
}

var shadowCommitTx = func(tx *sql.Tx) error { return tx.Commit() }

// PersistShadowAttempt uses one transaction per attempt. A failed attempt
// rolls back only its own row and never prevents a later attempt from being
// tried or persisted.
func (l *Ledger) PersistShadowAttempt(ctx context.Context, attempt ShadowAttempt, review *ShadowReview) error {
	tx, err := l.db.BeginTx(ctx, nil)
	if err != nil {
		return &ShadowPersistenceError{AttemptIndex: attempt.AttemptIndex, Stage: ShadowStageBegin}
	}
	defer tx.Rollback() //nolint:errcheck
	_, err = tx.ExecContext(ctx, `INSERT INTO packet_author_shadow_attempts
			(id,run_id,lane_id,input_hash,specialist_identity,failure_class,valid,equivalent,diff_json,manual_digest,specialist_digest,replay_stable,latency_ms,created_at)
			VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, attempt.ID, attempt.RunID, attempt.LaneID, attempt.InputHash,
		attempt.SpecialistIdentity, attempt.FailureClass, boolInt(attempt.Valid), boolInt(attempt.Equivalent), attempt.DiffJSON,
		attempt.ManualDigest, attempt.SpecialistDigest, boolInt(attempt.ReplayStable), attempt.LatencyMS,
		attempt.CreatedAt.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return &ShadowPersistenceError{AttemptIndex: attempt.AttemptIndex, Stage: ShadowStageInsert}
	}
	if review != nil {
		_, err = tx.ExecContext(ctx, `INSERT INTO packet_author_shadow_reviews (attempt_id,reviewer,review_ms,created_at) VALUES (?,?,?,?)`,
			review.AttemptID, review.Reviewer, review.ReviewMS, review.CreatedAt.UTC().Format(time.RFC3339Nano))
		if err != nil {
			return &ShadowPersistenceError{AttemptIndex: attempt.AttemptIndex, Stage: ShadowStageInsert}
		}
	}
	if err := shadowCommitTx(tx); err != nil {
		return &ShadowPersistenceError{AttemptIndex: attempt.AttemptIndex, Stage: ShadowStageCommit}
	}
	return nil
}

// PersistShadowAttempts continues after each failure and returns stable,
// sanitized warnings ordered by attempt then transaction stage.
func (l *Ledger) PersistShadowAttempts(ctx context.Context, attempts []ShadowAttempt) []ShadowPersistenceWarning {
	warnings := make([]ShadowPersistenceWarning, 0)
	for _, attempt := range attempts {
		if err := l.PersistShadowAttempt(ctx, attempt, nil); err != nil {
			var persistErr *ShadowPersistenceError
			if ok := asShadowPersistenceError(err, &persistErr); ok {
				warnings = append(warnings, ShadowPersistenceWarning{AttemptIndex: persistErr.AttemptIndex, Code: ShadowEvidencePersistFailed, Stage: persistErr.Stage})
			}
		}
	}
	sort.Slice(warnings, func(i, j int) bool {
		if warnings[i].AttemptIndex != warnings[j].AttemptIndex {
			return warnings[i].AttemptIndex < warnings[j].AttemptIndex
		}
		return shadowStageRank(warnings[i].Stage) < shadowStageRank(warnings[j].Stage)
	})
	return warnings
}

type ShadowPersistenceWarning struct {
	AttemptIndex int
	Code         string
	Stage        string
}

func (w ShadowPersistenceWarning) Error() string {
	return fmt.Sprintf("%s: attempt %d stage %s", w.Code, w.AttemptIndex, w.Stage)
}

func shadowStageRank(stage string) int {
	switch stage {
	case ShadowStageBegin:
		return 10
	case ShadowStageInsert:
		return 20
	case ShadowStageCommit:
		return 30
	default:
		return 99
	}
}

func asShadowPersistenceError(err error, target **ShadowPersistenceError) bool {
	if err == nil {
		return false
	}
	if value, ok := err.(*ShadowPersistenceError); ok {
		*target = value
		return true
	}
	return false
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
