package store

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// CloneEnvironmentResult summarizes a Coolify-style environment clone (config only).
type CloneEnvironmentResult struct {
	Environment  *Environment `json:"environment"`
	Applications int          `json:"applications"`
	Databases    int          `json:"databases"`
	Services     int          `json:"services"`
}

// CloneEnvironment duplicates all resources in srcEnv into a new environment.
// Copies configuration + env vars + scheduled tasks. Does not copy runtime data,
// volumes, webhook secrets, or FQDNs (avoids domain conflicts).
// On any failure after the destination environment is created, the partial clone
// is deleted so callers never keep an orphan half-copied environment.
func (s *Store) CloneEnvironment(ctx context.Context, teamID, projectID, srcEnvID uuid.UUID, name, desc string) (*CloneEnvironmentResult, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("name required")
	}
	src, err := s.GetEnvironment(ctx, teamID, srcEnvID)
	if err != nil {
		return nil, err
	}
	if src.ProjectID != projectID {
		return nil, ErrNotFound
	}
	dst, err := s.CreateEnvironment(ctx, teamID, projectID, name, desc)
	if err != nil {
		return nil, err
	}

	ok := false
	defer func() {
		if !ok {
			_ = s.purgeClonedEnvironment(context.Background(), teamID, projectID, dst.ID)
		}
	}()

	result := &CloneEnvironmentResult{Environment: dst}

	apps, err := s.ListApplications(ctx, teamID, &src.ID)
	if err != nil {
		return nil, err
	}
	for _, app := range apps {
		clone := app
		clone.ID = uuid.Nil
		clone.EnvironmentID = dst.ID
		clone.FQDN = ""
		clone.Status = "exited"
		clone.Name = uniqueCloneName(app.Name, name)
		created, err := s.CreateApplication(ctx, &clone)
		if err != nil {
			return nil, fmt.Errorf("clone application %s: %w", app.Name, err)
		}
		if err := s.copyResourceExtras(ctx, teamID, "application", app.ID, created.ID); err != nil {
			return nil, err
		}
		result.Applications++
	}

	dbs, err := s.ListDatabases(ctx, teamID, &src.ID)
	if err != nil {
		return nil, err
	}
	for _, db := range dbs {
		creds, err := s.GetDatabaseCredentials(ctx, teamID, db.ID)
		if err != nil {
			creds = ""
		}
		clone := db
		clone.ID = uuid.Nil
		clone.EnvironmentID = dst.ID
		clone.Status = "exited"
		clone.IsPublic = false
		clone.PublicPort = nil
		clone.Name = uniqueCloneName(db.Name, name)
		created, err := s.CreateDatabase(ctx, &clone, creds)
		if err != nil {
			return nil, fmt.Errorf("clone database %s: %w", db.Name, err)
		}
		if err := s.copyResourceExtras(ctx, teamID, "database", db.ID, created.ID); err != nil {
			return nil, err
		}
		result.Databases++
	}

	svcs, err := s.ListServices(ctx, teamID, &src.ID)
	if err != nil {
		return nil, err
	}
	for _, svc := range svcs {
		clone := svc
		clone.ID = uuid.New()
		clone.EnvironmentID = dst.ID
		clone.FQDN = ""
		clone.Status = "exited"
		// Keep raw compose; leave prepared blank so next deploy re-bakes Traefik/domains.
		clone.DockerCompose = ""
		clone.Name = uniqueCloneName(svc.Name, name)
		created, err := s.CreateService(ctx, &clone)
		if err != nil {
			return nil, fmt.Errorf("clone service %s: %w", svc.Name, err)
		}
		if err := s.copyResourceExtras(ctx, teamID, "service", svc.ID, created.ID); err != nil {
			return nil, err
		}
		result.Services++
	}

	shared, err := s.ListSharedEnv(ctx, teamID, "environment", &src.ID, true)
	if err == nil {
		for _, v := range shared {
			_, _ = s.UpsertSharedEnv(ctx, teamID, "environment", &dst.ID, v.Key, v.Value, v.IsLiteral)
		}
	}

	if refreshed, err := s.GetEnvironment(ctx, teamID, dst.ID); err == nil {
		result.Environment = refreshed
	}

	ok = true
	return result, nil
}

// purgeClonedEnvironment removes a partially cloned environment and its resources.
func (s *Store) purgeClonedEnvironment(ctx context.Context, teamID, projectID, envID uuid.UUID) error {
	apps, _ := s.ListApplications(ctx, teamID, &envID)
	for _, app := range apps {
		_ = s.DeleteApplication(ctx, teamID, app.ID)
	}
	dbs, _ := s.ListDatabases(ctx, teamID, &envID)
	for _, db := range dbs {
		_ = s.DeleteDatabase(ctx, teamID, db.ID)
	}
	svcs, _ := s.ListServices(ctx, teamID, &envID)
	for _, svc := range svcs {
		_ = s.DeleteService(ctx, teamID, svc.ID)
	}
	_, _ = s.Pool.Exec(ctx, `
		DELETE FROM shared_environment_variables
		WHERE team_id=$1 AND scope_type='environment' AND scope_id=$2
	`, teamID, envID)
	return s.DeleteEnvironment(ctx, teamID, projectID, envID)
}

func uniqueCloneName(base, envName string) string {
	base = strings.TrimSpace(base)
	suffix := strings.TrimSpace(envName)
	if suffix == "" {
		return base + "-clone"
	}
	return base + "-" + suffix
}

func (s *Store) copyResourceExtras(ctx context.Context, teamID uuid.UUID, resourceType string, fromID, toID uuid.UUID) error {
	vars, err := s.ListEnvVars(ctx, teamID, resourceType, fromID, true)
	if err != nil {
		return err
	}
	for _, v := range vars {
		if _, err := s.UpsertEnvVar(ctx, teamID, resourceType, toID, UpsertEnvVarInput{
			Key:           v.Key,
			Value:         v.Value,
			Runtime:       v.IsRuntime,
			Buildtime:     v.IsBuildtime,
			Literal:       v.IsLiteral,
			Multiline:     v.IsMultiline,
			Locked:        v.IsLocked,
			IsBuildSecret: v.IsBuildSecret,
			Comment:       v.Comment,
			BypassLock:    true,
		}); err != nil {
			return err
		}
	}

	tasks, err := s.ListScheduledTasks(ctx, teamID, resourceType, &fromID)
	if err != nil {
		return err
	}
	for _, t := range tasks {
		created, err := s.CreateScheduledTask(ctx, teamID, resourceType, toID, t.Name, t.Command, t.Frequency, t.Container)
		if err != nil {
			return err
		}
		if !t.Enabled {
			enabled := false
			_, _ = s.UpdateScheduledTask(ctx, teamID, created.ID, UpdateScheduledTaskInput{Enabled: &enabled})
		}
	}

	tags, err := s.ListResourceTags(ctx, teamID, resourceType, fromID)
	if err != nil {
		return err
	}
	for _, tag := range tags {
		_ = s.AttachTag(ctx, teamID, tag.ID, resourceType, toID)
	}

	if resourceType == "application" || resourceType == "database" || resourceType == "service" {
		vols, err := s.ListVolumes(ctx, teamID, resourceType, fromID)
		if err != nil {
			return err
		}
		for _, v := range vols {
			host := v.HostPath
			// Avoid sharing the source app's host path; leave blank so deploy recreates under new id.
			if resourceType == "application" {
				host = ""
			}
			if _, err := s.UpsertVolume(ctx, teamID, resourceType, toID, v.Name, v.MountPath, host, v.IsFile); err != nil {
				return err
			}
		}
	}
	return nil
}
