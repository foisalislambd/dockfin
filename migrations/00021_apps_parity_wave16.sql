-- +goose Up
-- Coolify Application parity: build commands, networking, health, advanced settings, swarm.

ALTER TABLE applications
    ADD COLUMN IF NOT EXISTS ports_mappings TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS custom_network_aliases TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS install_command TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS build_command TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS start_command TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS publish_directory TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS custom_nginx_configuration TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS preview_url_template TEXT NOT NULL DEFAULT '{{pr_id}}.{{domain}}',
    ADD COLUMN IF NOT EXISTS health_check_host TEXT NOT NULL DEFAULT 'localhost',
    ADD COLUMN IF NOT EXISTS health_check_scheme TEXT NOT NULL DEFAULT 'http',
    ADD COLUMN IF NOT EXISTS health_check_response_text TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS health_check_start_period INTEGER NOT NULL DEFAULT 5,
    ADD COLUMN IF NOT EXISTS health_check_type TEXT NOT NULL DEFAULT 'http',
    ADD COLUMN IF NOT EXISTS health_check_command TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS swarm_replicas INTEGER NOT NULL DEFAULT 1,
    ADD COLUMN IF NOT EXISTS swarm_placement_constraints TEXT NOT NULL DEFAULT '';

ALTER TABLE applications
    DROP CONSTRAINT IF EXISTS applications_health_check_type_check;
ALTER TABLE applications
    ADD CONSTRAINT applications_health_check_type_check
    CHECK (health_check_type IN ('http', 'cmd'));

ALTER TABLE application_settings
    ADD COLUMN IF NOT EXISTS is_spa BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS inject_build_args_to_dockerfile BOOLEAN NOT NULL DEFAULT TRUE,
    ADD COLUMN IF NOT EXISTS use_build_secrets BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS include_source_commit_in_build BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS docker_images_to_keep INTEGER NOT NULL DEFAULT 5,
    ADD COLUMN IF NOT EXISTS is_consistent_container_name_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS custom_internal_name TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS is_gzip_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    ADD COLUMN IF NOT EXISTS is_stripprefix_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    ADD COLUMN IF NOT EXISTS is_log_drain_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS is_debug_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS is_env_sorting_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    ADD COLUMN IF NOT EXISTS is_pr_deployments_public_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS skip_rebuild_if_unchanged BOOLEAN NOT NULL DEFAULT TRUE,
    ADD COLUMN IF NOT EXISTS gpu_driver TEXT NOT NULL DEFAULT 'nvidia',
    ADD COLUMN IF NOT EXISTS gpu_device_ids TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS gpu_options TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS custom_docker_max_restart_count INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS pre_deployment_command_container TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS post_deployment_command_container TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS is_swarm_only_worker_nodes BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS is_include_timestamps BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS logs_line_limit INTEGER NOT NULL DEFAULT 1000;

-- Build secrets: mark env vars as Docker BuildKit secrets (Coolify use_build_secrets).
ALTER TABLE environment_variables
    ADD COLUMN IF NOT EXISTS is_build_secret BOOLEAN NOT NULL DEFAULT FALSE;

-- +goose Down
ALTER TABLE environment_variables DROP COLUMN IF EXISTS is_build_secret;

ALTER TABLE application_settings
    DROP COLUMN IF EXISTS is_spa,
    DROP COLUMN IF EXISTS inject_build_args_to_dockerfile,
    DROP COLUMN IF EXISTS use_build_secrets,
    DROP COLUMN IF EXISTS include_source_commit_in_build,
    DROP COLUMN IF EXISTS docker_images_to_keep,
    DROP COLUMN IF EXISTS is_consistent_container_name_enabled,
    DROP COLUMN IF EXISTS custom_internal_name,
    DROP COLUMN IF EXISTS is_gzip_enabled,
    DROP COLUMN IF EXISTS is_stripprefix_enabled,
    DROP COLUMN IF EXISTS is_log_drain_enabled,
    DROP COLUMN IF EXISTS is_debug_enabled,
    DROP COLUMN IF EXISTS is_env_sorting_enabled,
    DROP COLUMN IF EXISTS is_pr_deployments_public_enabled,
    DROP COLUMN IF EXISTS skip_rebuild_if_unchanged,
    DROP COLUMN IF EXISTS gpu_driver,
    DROP COLUMN IF EXISTS gpu_device_ids,
    DROP COLUMN IF EXISTS gpu_options,
    DROP COLUMN IF EXISTS custom_docker_max_restart_count,
    DROP COLUMN IF EXISTS pre_deployment_command_container,
    DROP COLUMN IF EXISTS post_deployment_command_container,
    DROP COLUMN IF EXISTS is_swarm_only_worker_nodes,
    DROP COLUMN IF EXISTS is_include_timestamps,
    DROP COLUMN IF EXISTS logs_line_limit;

ALTER TABLE applications DROP CONSTRAINT IF EXISTS applications_health_check_type_check;
ALTER TABLE applications
    DROP COLUMN IF EXISTS ports_mappings,
    DROP COLUMN IF EXISTS custom_network_aliases,
    DROP COLUMN IF EXISTS install_command,
    DROP COLUMN IF EXISTS build_command,
    DROP COLUMN IF EXISTS start_command,
    DROP COLUMN IF EXISTS publish_directory,
    DROP COLUMN IF EXISTS custom_nginx_configuration,
    DROP COLUMN IF EXISTS preview_url_template,
    DROP COLUMN IF EXISTS health_check_host,
    DROP COLUMN IF EXISTS health_check_scheme,
    DROP COLUMN IF EXISTS health_check_response_text,
    DROP COLUMN IF EXISTS health_check_start_period,
    DROP COLUMN IF EXISTS health_check_type,
    DROP COLUMN IF EXISTS health_check_command,
    DROP COLUMN IF EXISTS swarm_replicas,
    DROP COLUMN IF EXISTS swarm_placement_constraints;
