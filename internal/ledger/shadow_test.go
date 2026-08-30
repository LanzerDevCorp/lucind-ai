package ledger

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestPersistShadowAttemptAndReviewIsolatedInSQLite(t *testing.T) {
	l := openTestLedger(t)
	ctx := context.Background()
	attempt := validShadowAttempt("attempt-1", 1)
	if err := l.PersistShadowAttempt(ctx, attempt, &ShadowReview{AttemptID: attempt.ID, Reviewer: "operator", ReviewMS: 17, CreatedAt: attempt.CreatedAt}); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := l.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM packet_author_shadow_attempts WHERE id = ?`, attempt.ID).Scan(&count); err != nil || count != 1 {
		t.Fatalf("attempt count = %d, %v", count, err)
	}
	if err := l.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM packet_author_shadow_reviews WHERE attempt_id = ?`, attempt.ID).Scan(&count); err != nil || count != 1 {
		t.Fatalf("review count = %d, %v", count, err)
	}
	warnings := l.PersistShadowAttempts(ctx, []ShadowAttempt{validShadowAttempt("attempt-2", 2)})
	if len(warnings) != 0 {
		t.Fatalf("later attempt warnings = %+v", warnings)
	}
	failedThenLater := []ShadowAttempt{{ID: "bad", AttemptIndex: 4, LatencyMS: -1, CreatedAt: time.Now()}, validShadowAttempt("attempt-5", 5)}
	if warnings := l.PersistShadowAttempts(ctx, failedThenLater); len(warnings) != 1 || warnings[0].AttemptIndex != 4 {
		t.Fatalf("isolated warnings = %+v", warnings)
	}
	if err := l.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM packet_author_shadow_attempts WHERE id = 'attempt-5'`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("later attempt count = %d, %v", count, err)
	}
	sorted := l.PersistShadowAttempts(ctx, []ShadowAttempt{
		{ID: "bad-2", AttemptIndex: 2, LatencyMS: -1, CreatedAt: time.Now()},
		{ID: "bad-1", AttemptIndex: 1, LatencyMS: -1, CreatedAt: time.Now()},
	})
	if len(sorted) != 2 || sorted[0].AttemptIndex != 1 || sorted[1].AttemptIndex != 2 {
		t.Fatalf("warnings are not ordered by attempt: %+v", sorted)
	}
}

func TestPersistShadowAttemptReportsSanitizedBeginInsertAndCommitFailures(t *testing.T) {
	ctx := context.Background()

	beginLedger := openTestLedger(t)
	if err := beginLedger.Close(); err != nil {
		t.Fatal(err)
	}
	warning := beginLedger.PersistShadowAttempts(ctx, []ShadowAttempt{validShadowAttempt("begin", 1)})
	if len(warning) != 1 || warning[0].Stage != ShadowStageBegin || warning[0].Code != ShadowEvidencePersistFailed {
		t.Fatalf("begin warning = %+v", warning)
	}
	if strings.Contains(warning[0].Error(), "database") {
		t.Fatalf("warning leaked SQLite detail: %v", warning[0])
	}

	insertLedger := openTestLedger(t)
	insertWarnings := insertLedger.PersistShadowAttempts(ctx, []ShadowAttempt{{ID: "insert", AttemptIndex: 2, LatencyMS: -1, CreatedAt: time.Now()}})
	if len(insertWarnings) != 1 || insertWarnings[0].Stage != ShadowStageInsert {
		t.Fatalf("insert warning = %+v", insertWarnings)
	}

	commitLedger := openTestLedger(t)
	previousCommit := shadowCommitTx
	shadowCommitTx = func(*sql.Tx) error { return errors.New("synthetic commit failure") }
	t.Cleanup(func() { shadowCommitTx = previousCommit })
	commitWarnings := commitLedger.PersistShadowAttempts(ctx, []ShadowAttempt{validShadowAttempt("commit", 3)})
	if len(commitWarnings) != 1 || commitWarnings[0].Stage != ShadowStageCommit || commitWarnings[0].Code != ShadowEvidencePersistFailed {
		t.Fatalf("commit warning = %+v", commitWarnings)
	}
}

func validShadowAttempt(id string, index int) ShadowAttempt {
	return ShadowAttempt{ID: id, AttemptIndex: index, RunID: "run", LaneID: "lane", InputHash: "input", SpecialistIdentity: "lucind-packet-author", FailureClass: ShadowFailureNone, Valid: true, Equivalent: true, DiffJSON: "[]", ManualDigest: "manual", SpecialistDigest: "specialist", ReplayStable: true, LatencyMS: 3, CreatedAt: time.Now().UTC()}
}
