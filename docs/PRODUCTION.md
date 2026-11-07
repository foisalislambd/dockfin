# Production

## Startup

`dockfin serve` refuses to start when:

- `DOCKFIN_DATABASE_URL` is missing
- `DOCKFIN_MASTER_KEY` is shorter than 32 characters, or it is the example key while `DOCKFIN_ENV` is not `development`
- `DOCKFIN_PUBLIC_URL` is not an absolute `http` or `https` URL (passkeys and cookies use it)

`serve` runs Goose `Up` before it listens. Down migrations are not applied. They drop data and are not part of an upgrade.

In production, set `DOCKFIN_METRICS_TOKEN`. Without it, `GET /metrics` returns 401 and the log says so. Development serves metrics without a token.

## Health

| Path | Meaning |
| --- | --- |
| `GET /health` | Process is up and Postgres answers. `503` when the database ping fails. Includes the running version. |
| `GET /ready` | Same database ping, for a load balancer readiness check. |
| `GET /metrics` | Prometheus. Production requires `Authorization: Bearer <DOCKFIN_METRICS_TOKEN>`. |

Shutdown on `SIGINT` or `SIGTERM` stops the HTTP server (15 seconds), the deploy workers, the scheduler, and the job workers. An SSH session that ignores cancel can still be in flight when the process exits.

## Upgrade

1. Back up the control-plane database (below) and keep the previous image tag.
2. Deploy the new image or binary. `serve` applies every new file in `migrations/` in order.
3. Index builds in those files take a lock. Run the upgrade when deploys can wait.
4. Confirm `GET /health` and `GET /ready`.

`VERSION.md` is the product version. A push to `main` with a higher `X.Y.Z` than the latest `vX.Y.Z` tag is what creates the GitHub Release and the image tags. Do not tag by hand unless that workflow is disabled.

The dashboard package version matches `VERSION.md`. OpenAPI `info.version` stays `dev` on purpose.

## Backups

**Control plane.** `POST /api/v1/settings/backup/run` queues an instance backup and returns `202` with a job. `dfin jobs <id>` or the Settings page follows it. Restore still asks for `RESTORE`.

**Application and database backups** stay on the scheduler. They are not the instance backup.

**Before an upgrade**, take an instance backup or a Postgres dump of the Dockfin database, and keep `DOCKFIN_MASTER_KEY`. Secrets cannot be decrypted with a different key.

## Jobs

On-demand image scans, Docker cleanup, and instance backups are rows in `background_jobs`. Deployments keep the deployment queue. Cancel a job only while it is `queued`:

```bash
dfin jobs
dfin jobs <job-uuid>
dfin jobs cancel <job-uuid>
```
