package store

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// TransferResult is what moved with a server, and what the target team must set up again.
type TransferResult struct {
	ServerID     uuid.UUID `json:"server_id"`
	Applications int       `json:"applications"`
	Services     int       `json:"services"`
	Databases    int       `json:"databases"`
	Projects     int       `json:"projects"`
	Split        int       `json:"projects_split"`
	Warnings     []string  `json:"warnings"`
}

type movedResource struct {
	id    uuid.UUID
	envID uuid.UUID
	gitID *uuid.UUID
	regID *uuid.UUID
	keyID *uuid.UUID
}

func (s *Store) transferServer(ctx context.Context, sourceTeam, targetTeam, serverID uuid.UUID) (*TransferResult, error) {
	if sourceTeam == targetTeam {
		return nil, fmt.Errorf("server is already on that team")
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var keyID *uuid.UUID
	err = tx.QueryRow(ctx, `SELECT private_key_id FROM servers WHERE id=$1 AND team_id=$2 FOR UPDATE`, serverID, sourceTeam).Scan(&keyID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	var extra int
	if err := tx.QueryRow(ctx, `
		SELECT COUNT(*) FROM additional_destinations ad
		JOIN destinations d ON d.id = ad.destination_id
		WHERE d.server_id <> $1
		  AND ad.application_id IN (
		    SELECT a.id FROM applications a JOIN destinations dest ON dest.id = a.destination_id WHERE dest.server_id = $1
		  )
	`, serverID).Scan(&extra); err != nil {
		return nil, err
	}
	if extra > 0 {
		return nil, fmt.Errorf("%d application also runs on another server; remove that extra destination first", extra)
	}

	apps, err := loadMoved(tx, ctx, `
		SELECT a.id, a.environment_id, a.git_source_id, a.docker_registry_id, a.private_key_id
		FROM applications a JOIN destinations d ON d.id = a.destination_id
		WHERE d.server_id=$1 AND a.team_id=$2
	`, serverID, sourceTeam)
	if err != nil {
		return nil, err
	}
	dbs, err := loadMoved(tx, ctx, `
		SELECT db.id, db.environment_id, NULL::uuid, NULL::uuid, NULL::uuid
		FROM databases db JOIN destinations d ON d.id = db.destination_id
		WHERE d.server_id=$1 AND db.team_id=$2
	`, serverID, sourceTeam)
	if err != nil {
		return nil, err
	}
	svcs, err := loadMoved(tx, ctx, `
		SELECT sv.id, sv.environment_id, NULL::uuid, NULL::uuid, NULL::uuid
		FROM services sv
		WHERE sv.deleted_at IS NULL AND sv.team_id=$2
		  AND (sv.server_id=$1 OR sv.destination_id IN (SELECT id FROM destinations WHERE server_id=$1))
	`, serverID, sourceTeam)
	if err != nil {
		return nil, err
	}

	res := &TransferResult{
		ServerID: serverID, Applications: len(apps), Databases: len(dbs), Services: len(svcs), Warnings: []string{},
	}
	envResources := map[uuid.UUID]struct{}{}
	for _, r := range append(append(apps, dbs...), svcs...) {
		envResources[r.envID] = struct{}{}
	}

	envProject := map[uuid.UUID]uuid.UUID{}
	projectEnvs := map[uuid.UUID][]uuid.UUID{}
	for envID := range envResources {
		var projectID uuid.UUID
		if err := tx.QueryRow(ctx, `SELECT project_id FROM environments WHERE id=$1 AND team_id=$2`, envID, sourceTeam).Scan(&projectID); err != nil {
			return nil, err
		}
		envProject[envID] = projectID
		projectEnvs[projectID] = append(projectEnvs[projectID], envID)
	}

	newEnv := map[uuid.UUID]uuid.UUID{}
	for projectID, envs := range projectEnvs {
		var name, desc string
		if err := tx.QueryRow(ctx, `SELECT name, description FROM projects WHERE id=$1`, projectID).Scan(&name, &desc); err != nil {
			return nil, err
		}
		var others int
		if err := tx.QueryRow(ctx, `
			SELECT
			  (SELECT COUNT(*) FROM applications a JOIN environments e ON e.id=a.environment_id
			    WHERE e.project_id=$1 AND a.id <> ALL($2::uuid[]))
			+ (SELECT COUNT(*) FROM databases d JOIN environments e ON e.id=d.environment_id
			    WHERE e.project_id=$1 AND d.id <> ALL($3::uuid[]))
			+ (SELECT COUNT(*) FROM services sv JOIN environments e ON e.id=sv.environment_id
			    WHERE e.project_id=$1 AND sv.deleted_at IS NULL AND sv.id <> ALL($4::uuid[]))
		`, projectID, idsOf(apps), idsOf(dbs), idsOf(svcs)).Scan(&others); err != nil {
			return nil, err
		}
		if others == 0 {
			if err := moveProject(ctx, tx, projectID, targetTeam, name); err != nil {
				return nil, err
			}
			if _, err := tx.Exec(ctx, `UPDATE environments SET team_id=$2 WHERE project_id=$1`, projectID, targetTeam); err != nil {
				return nil, err
			}
			for _, id := range envs {
				newEnv[id] = id
			}
			res.Projects++
			continue
		}
		newName := name + " (moved)"
		var newProject uuid.UUID
		err := tx.QueryRow(ctx, `
			INSERT INTO projects (team_id, name, description) VALUES ($1,$2,$3) RETURNING id
		`, targetTeam, newName, desc).Scan(&newProject)
		if err != nil && strings.Contains(err.Error(), "duplicate") {
			err = tx.QueryRow(ctx, `
				INSERT INTO projects (team_id, name, description) VALUES ($1,$2,$3) RETURNING id
			`, targetTeam, newName+" "+serverID.String()[:8], desc).Scan(&newProject)
		}
		if err != nil {
			return nil, err
		}
		for _, envID := range envs {
			var ename, edesc string
			if err := tx.QueryRow(ctx, `SELECT name, description FROM environments WHERE id=$1`, envID).Scan(&ename, &edesc); err != nil {
				return nil, err
			}
			var created uuid.UUID
			if err := tx.QueryRow(ctx, `
				INSERT INTO environments (team_id, project_id, name, description) VALUES ($1,$2,$3,$4) RETURNING id
			`, targetTeam, newProject, ename, edesc).Scan(&created); err != nil {
				return nil, err
			}
			newEnv[envID] = created
		}
		res.Split++
		res.Warnings = append(res.Warnings, name+" also has resources on another server, so only this server's resources moved into a new project")
	}

	if err := retargetEnv(ctx, tx, "applications", apps, newEnv, targetTeam); err != nil {
		return nil, err
	}
	if err := retargetEnv(ctx, tx, "databases", dbs, newEnv, targetTeam); err != nil {
		return nil, err
	}
	if err := retargetEnv(ctx, tx, "services", svcs, newEnv, targetTeam); err != nil {
		return nil, err
	}

	if detached := countSet(apps, func(r movedResource) bool { return r.gitID != nil }); detached > 0 {
		if _, err := tx.Exec(ctx, `UPDATE applications SET git_source_id=NULL WHERE id = ANY($1::uuid[]) AND git_source_id IS NOT NULL`, idsOf(apps)); err != nil {
			return nil, err
		}
		res.Warnings = append(res.Warnings, "Git sources stay on the original team. Reconnect them on the applications that moved.")
	}
	if countSet(apps, func(r movedResource) bool { return r.regID != nil }) > 0 {
		if _, err := tx.Exec(ctx, `UPDATE applications SET docker_registry_id=NULL WHERE id = ANY($1::uuid[]) AND docker_registry_id IS NOT NULL`, idsOf(apps)); err != nil {
			return nil, err
		}
		res.Warnings = append(res.Warnings, "Docker registries stay on the original team. Select them again on the applications that moved.")
	}

	all := append(append(idsOf(apps), idsOf(dbs)...), idsOf(svcs)...)
	for _, table := range []string{"environment_variables", "volumes", "scheduled_backups", "backup_executions", "scheduled_tasks", "image_scans", "resource_controls"} {
		if _, err := tx.Exec(ctx, `UPDATE `+table+` SET team_id=$2 WHERE resource_id = ANY($1::uuid[])`, all, targetTeam); err != nil {
			return nil, err
		}
	}
	if _, err := tx.Exec(ctx, `UPDATE application_previews SET team_id=$2 WHERE application_id = ANY($1::uuid[])`, idsOf(apps), targetTeam); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `UPDATE service_domains SET team_id=$2 WHERE service_id = ANY($1::uuid[])`, idsOf(svcs), targetTeam); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE deployments SET team_id=$2
		WHERE application_id = ANY($1::uuid[]) OR service_id = ANY($3::uuid[])
	`, idsOf(apps), targetTeam, idsOf(svcs)); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `UPDATE service_components SET team_id=$2 WHERE service_id = ANY($1::uuid[])`, idsOf(svcs), targetTeam); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE scheduled_task_executions SET team_id=$2
		WHERE scheduled_task_id IN (SELECT id FROM scheduled_tasks WHERE resource_id = ANY($1::uuid[]))
	`, all, targetTeam); err != nil {
		return nil, err
	}
	for _, table := range []string{"docker_cleanup_executions", "server_proxy_configurations", "ssl_certificates"} {
		if _, err := tx.Exec(ctx, `UPDATE `+table+` SET team_id=$2 WHERE server_id=$1`, serverID, targetTeam); err != nil {
			return nil, err
		}
	}
	var runners int
	if err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM github_runner_configs WHERE server_id=$1`, serverID).Scan(&runners); err != nil {
		return nil, err
	}
	if runners > 0 {
		if _, err := tx.Exec(ctx, `DELETE FROM github_runner_configs WHERE server_id=$1`, serverID); err != nil {
			return nil, err
		}
		res.Warnings = append(res.Warnings, "The GitHub runner configuration was removed. It used a Git source that stays on the original team.")
	}
	copied := map[uuid.UUID]uuid.UUID{}
	for _, app := range apps {
		if app.keyID == nil {
			continue
		}
		newKey, ok := copied[*app.keyID]
		if !ok {
			err = tx.QueryRow(ctx, `
				INSERT INTO private_keys (team_id, name, description, public_key, private_key_enc, fingerprint)
				SELECT $2, name || ' (transferred)', description, public_key, private_key_enc, fingerprint
				FROM private_keys WHERE id=$1
				RETURNING id
			`, *app.keyID, targetTeam).Scan(&newKey)
			if err != nil {
				return nil, err
			}
			copied[*app.keyID] = newKey
		}
		if _, err := tx.Exec(ctx, `UPDATE applications SET private_key_id=$2 WHERE id=$1`, app.id, newKey); err != nil {
			return nil, err
		}
	}
	cleared, err := tx.Exec(ctx, `
		UPDATE scheduled_backups SET s3_storage_id=NULL
		WHERE resource_id = ANY($1::uuid[]) AND s3_storage_id IS NOT NULL
	`, all)
	if err != nil {
		return nil, err
	}
	replicas, err := tx.Exec(ctx, `
		DELETE FROM scheduled_backup_s3_storages
		WHERE scheduled_backup_id IN (SELECT id FROM scheduled_backups WHERE resource_id = ANY($1::uuid[]))
	`, all)
	if err != nil {
		return nil, err
	}
	if cleared.RowsAffected() > 0 || replicas.RowsAffected() > 0 {
		res.Warnings = append(res.Warnings, "Scheduled backups moved, but their S3 storage stays on the original team and was disconnected.")
	}
	if _, err := tx.Exec(ctx, `UPDATE shared_environment_variables SET team_id=$2 WHERE scope_type='server' AND scope_id=$1`, serverID, targetTeam); err != nil {
		return nil, err
	}

	if keyID != nil {
		var newKey uuid.UUID
		err = tx.QueryRow(ctx, `
			INSERT INTO private_keys (team_id, name, description, public_key, private_key_enc, fingerprint)
			SELECT $2, name || ' (transferred)', description, public_key, private_key_enc, fingerprint
			FROM private_keys WHERE id=$1
			RETURNING id
		`, *keyID, targetTeam).Scan(&newKey)
		if err != nil {
			return nil, err
		}
		keyID = &newKey
	}
	if _, err := tx.Exec(ctx, `UPDATE servers SET team_id=$2, private_key_id=$3, updated_at=NOW() WHERE id=$1`, serverID, targetTeam, keyID); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `UPDATE destinations SET team_id=$2, updated_at=NOW() WHERE server_id=$1`, serverID, targetTeam); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `UPDATE server_health SET team_id=$2 WHERE server_id=$1`, serverID, targetTeam); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `UPDATE server_metrics SET team_id=$2 WHERE server_id=$1`, serverID, targetTeam); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	if res.Warnings == nil {
		res.Warnings = []string{}
	}
	return res, nil
}

func loadMoved(tx pgx.Tx, ctx context.Context, q string, serverID, teamID uuid.UUID) ([]movedResource, error) {
	rows, err := tx.Query(ctx, q, serverID, teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []movedResource
	for rows.Next() {
		var r movedResource
		if err := rows.Scan(&r.id, &r.envID, &r.gitID, &r.regID, &r.keyID); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func idsOf(in []movedResource) []uuid.UUID {
	out := make([]uuid.UUID, len(in))
	for i := range in {
		out[i] = in[i].id
	}
	if out == nil {
		out = []uuid.UUID{}
	}
	return out
}

func countSet(in []movedResource, ok func(movedResource) bool) int {
	n := 0
	for _, r := range in {
		if ok(r) {
			n++
		}
	}
	return n
}

func moveProject(ctx context.Context, tx pgx.Tx, id, team uuid.UUID, name string) error {
	_, err := tx.Exec(ctx, `UPDATE projects SET team_id=$2, name=$3, updated_at=NOW() WHERE id=$1`, id, team, name)
	if err != nil && strings.Contains(err.Error(), "duplicate") {
		_, err = tx.Exec(ctx, `UPDATE projects SET team_id=$2, name=$3, updated_at=NOW() WHERE id=$1`, id, team, name+" (moved)")
	}
	return err
}

func retargetEnv(ctx context.Context, tx pgx.Tx, table string, rows []movedResource, newEnv map[uuid.UUID]uuid.UUID, team uuid.UUID) error {
	for _, r := range rows {
		env := newEnv[r.envID]
		if env == uuid.Nil {
			env = r.envID
		}
		if _, err := tx.Exec(ctx, `UPDATE `+table+` SET team_id=$2, environment_id=$3, updated_at=NOW() WHERE id=$1`, r.id, team, env); err != nil {
			return err
		}
	}
	return nil
}
