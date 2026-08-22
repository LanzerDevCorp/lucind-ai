package ledger

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

const laneMetadataAuditPrefix = "lane_metadata:v1:"

// LaneMetadata is the dispatch context attached to an existing lane. RunID
// and LaneID identify the lane but are never updated by UpdateLaneMetadata.
// Schema-v6's model, agent, and feature columns remain directly queryable;
// the complete snapshot is retained in the append-only event log so changes
// to the remaining packet and SDD context are auditable without widening the
// approved v6 schema.
type LaneMetadata struct {
	RunID        string   `json:"run_id"`
	LaneID       string   `json:"lane_id"`
	Model        string   `json:"model"`
	Agent        string   `json:"agent"`
	SDDPhase     string   `json:"sdd_phase"`
	FanoutGroup  string   `json:"fanout_group"`
	Change       string   `json:"change"`
	Feature      string   `json:"feature"`
	AllowedPaths []string `json:"allowed_paths"`
	Dependencies []string `json:"dependencies"`
	BodyDigest   string   `json:"body_digest"`
}

// UpdateLaneMetadata updates schema-v6's lane metadata columns and appends a
// complete metadata snapshot to the event log in one transaction. It never
// replaces the lane row, preserving its composite identity and lifecycle
// fields. An unknown (run_id, lane_id) pair returns ErrLaneUnknown and writes
// no audit event.
func (l *Ledger) UpdateLaneMetadata(ctx context.Context, metadata LaneMetadata, at time.Time) error {
	detail, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("ledger: encode lane metadata: %w", err)
	}

	tx, err := l.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("ledger: begin lane metadata update: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // no-op once Commit succeeds

	result, err := tx.ExecContext(ctx, `
		UPDATE lanes SET model = ?, agent = ?, feature = ?
		WHERE run_id = ? AND lane_id = ?`,
		metadata.Model, metadata.Agent, metadata.Feature, metadata.RunID, metadata.LaneID,
	)
	if err != nil {
		return fmt.Errorf("ledger: update lane metadata: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("ledger: read lane metadata rows affected: %w", err)
	}
	if affected == 0 {
		return ErrLaneUnknown
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO events (run_id, lane_id, type, detail, at)
		VALUES (?, ?, ?, ?, ?)`,
		metadata.RunID,
		metadata.LaneID,
		EventLaneNote,
		laneMetadataAuditPrefix+string(detail),
		at.UTC().Format(time.RFC3339),
	); err != nil {
		return fmt.Errorf("ledger: append lane metadata audit: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("ledger: commit lane metadata update: %w", err)
	}
	return nil
}

// GetLaneMetadata returns the latest audited metadata snapshot for an
// existing lane. The schema-v6 model, agent, and feature columns are the
// authoritative values for those fields; lanes that predate an audit record
// return those columns with zero values for the extended metadata.
func (l *Ledger) GetLaneMetadata(ctx context.Context, runID, laneID string) (LaneMetadata, error) {
	metadata := LaneMetadata{RunID: runID, LaneID: laneID}
	var model, agent, feature sql.NullString
	if err := l.db.QueryRowContext(ctx, `
		SELECT model, agent, feature FROM lanes
		WHERE run_id = ? AND lane_id = ?`, runID, laneID).
		Scan(&model, &agent, &feature); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return LaneMetadata{}, ErrLaneUnknown
		}
		return LaneMetadata{}, fmt.Errorf("ledger: query lane metadata: %w", err)
	}

	var detail string
	err := l.db.QueryRowContext(ctx, `
		SELECT detail FROM events
		WHERE run_id = ? AND lane_id = ? AND type = ?
		  AND substr(detail, 1, ?) = ?
		ORDER BY id DESC LIMIT 1`,
		runID, laneID, EventLaneNote, len(laneMetadataAuditPrefix), laneMetadataAuditPrefix,
	).Scan(&detail)
	switch {
	case err == nil:
		if err := json.Unmarshal([]byte(detail[len(laneMetadataAuditPrefix):]), &metadata); err != nil {
			return LaneMetadata{}, fmt.Errorf("ledger: decode lane metadata audit: %w", err)
		}
	case errors.Is(err, sql.ErrNoRows):
		// Migrated v5 rows and lanes written before this API have no audit
		// snapshot. Their v6 columns still form a valid partial result.
	default:
		return LaneMetadata{}, fmt.Errorf("ledger: query lane metadata audit: %w", err)
	}

	metadata.RunID = runID
	metadata.LaneID = laneID
	metadata.Model = model.String
	metadata.Agent = agent.String
	metadata.Feature = feature.String
	return metadata, nil
}
