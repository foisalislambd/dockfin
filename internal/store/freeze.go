package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// ErrDeployFrozen is returned when an environment refuses new deployments.
var ErrDeployFrozen = errors.New("deployments are frozen for this environment")

// ErrFreezeReasonTooLong rejects operator notes that will not fit the column.
var ErrFreezeReasonTooLong = errors.New("freeze reason must be 240 characters or fewer")

const maxFreezeReasonRunes = 240

// NormalizeFreezeReason trims a freeze note and rejects control characters
// and anything longer than the stored limit. An empty reason is allowed.
func NormalizeFreezeReason(reason string) (string, error) {
	reason = strings.TrimSpace(reason)
	reason = strings.Map(func(r rune) rune {
		switch r {
		case '\n', '\r', '\t':
			return ' '
		}
		if r < 32 || r == 127 {
			return -1
		}
		return r
	}, reason)
	reason = strings.Join(strings.Fields(reason), " ")
	if utf8.RuneCountInString(reason) > maxFreezeReasonRunes {
		return "", ErrFreezeReasonTooLong
	}
	return reason, nil
}

// FrozenDeployError wraps ErrDeployFrozen with the operator's reason when set.
func FrozenDeployError(reason string) error {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return fmt.Errorf("%w", ErrDeployFrozen)
	}
	return fmt.Errorf("%w: %s", ErrDeployFrozen, reason)
}

// SetEnvironmentDeployFreeze pauses or resumes deploys for one environment.
// Unfreezing clears the reason, timestamp, and actor.
func (s *Store) SetEnvironmentDeployFreeze(ctx context.Context, teamID, projectID, envID uuid.UUID, userID *uuid.UUID, frozen bool, reason string) (*Environment, error) {
	if !frozen {
		reason = ""
		userID = nil
	}
	row := s.Pool.QueryRow(ctx, `
		UPDATE environments SET
			deploy_frozen=$4,
			deploy_freeze_reason=$5,
			deploy_frozen_at=CASE WHEN $4 THEN NOW() ELSE NULL END,
			deploy_frozen_by=CASE WHEN $4 THEN $6 ELSE NULL END,
			updated_at=NOW()
		WHERE id=$1 AND project_id=$2 AND team_id=$3
		RETURNING `+environmentSelect, envID, projectID, teamID, frozen, reason, userID)
	e, err := scanEnvironment(row.Scan)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	empty, err := s.EnvironmentIsEmpty(ctx, teamID, envID)
	if err != nil {
		return nil, err
	}
	e.IsEmpty = empty
	return e, nil
}
