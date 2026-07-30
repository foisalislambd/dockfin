# Coolify Projects System — Deep Audit & Goolify Gaps

Reference tree: `coolify/` (upstream Coolify). Goolify target: Go API + React under `apps/web`.

Last updated: 2026-07-30

---

## 1. Coolify architecture (Projects)

```text
Sidebar → Projects
   │
   ├─ Project Index (/projects)
   │     • List cards
   │     • + Add Resource → env/new (first env)
   │     • Settings → project/edit
   │     • Card click → navigateTo()
   │           • 1 environment → resource index
   │           • else → project show (env list)
   │
   ├─ Project Show (/project/{uuid})
   │     • Environment cards
   │     • + Add environment
   │
   ├─ Project Edit (/project/{uuid}/edit)
   │     • Name / description
   │     • Delete Project (disabled unless isEmpty)
   │
   ├─ Environment Resources (/project/.../environment/...)
   │     • Apps / DBs / Services cards
   │     • + New → New Resource select
   │     • Clone Environment
   │     • Environment Settings
   │
   ├─ Environment Edit
   │     • Name / description
   │     • Delete Environment (disabled unless isEmpty)
   │
   ├─ New Resource (select.blade.php)
   │     • Applications (Git / Docker)
   │     • Databases (engines)
   │     • Services (one-click catalog + search + category)
   │
   └─ Resource detail (Application / Database / Service)
         • Configuration tabs + Danger zone (delete resource)
```

Key Coolify helpers:

| Piece | Path / behavior |
|-------|-----------------|
| `sslip()` / `generateUrl()` | Magic domains are **http://** `{ip}.sslip.io` |
| `Project::navigateTo()` | 1 env → resources; else → env list |
| `Project::isEmpty()` | No apps / DBs / services → delete allowed |
| `DeleteProject` | Confirm by typing project name; refuse if not empty |
| `DeleteEnvironment` | Same pattern for environments |
| Navbar | **No** top-level Applications / Databases / Services |

---

## 2. Coolify Projects feature inventory

### Navigation & list
- [x] Projects list with search-ish layout
- [x] Create project → auto `production` environment
- [x] Single-env shortcut into resources
- [x] “+ Add Resource” from list
- [x] Project Settings link

### Project settings
- [x] Edit name / description (`Project/Edit`)
- [x] Delete project with typed confirmation
- [x] Delete **blocked** while any resource exists (`isEmpty()`)

### Environments
- [x] List environments under project
- [x] Create environment
- [x] Edit environment name/description
- [x] Delete environment (empty only)
- [x] Clone environment / project (`CloneMe`) — server + destination + optional volume clone
- [x] Environment shared variables

### Resources inside environment
- [x] Unified resource index (apps, DBs, services)
- [x] New Resource mega-select (apps + DBs + one-click services)
- [x] Destination picker before create (Coolify wizard)
- [x] Resource detail pages with many tabs (env, storage, backups, logs, terminal, danger, …)
- [x] Per-resource Danger zone delete

### Cross-cutting
- [x] Project-scoped shared env vars
- [x] Environment-scoped shared env vars
- [x] Tags on resources
- [x] Clone with volumes (async jobs)

---

## 3. What Goolify already has

| Area | Status |
|------|--------|
| Projects CRUD create/list/get | Yes — update/delete with Coolify empty-only rule |
| Auto `production` env on create | Yes |
| Environments list/create | Yes — update/delete with empty-only rule |
| Nested routes Project → Env → Resources | Yes |
| Coolify-style New Resource (apps/DBs/catalog) | Yes (frontend pass) |
| Sidebar without Apps/DBs/Services | Yes |
| Flat `/applications` redirect to `/projects` | Yes |
| Create project → land in env resources | Yes |
| Single-env redirect | Yes |
| App / DB / Service create + detail + deploy | Yes (subset of Coolify tabs) |
| App/Service/DB delete on resource Danger tab | Mostly yes |
| Magic domains (sslip http) + Traefik | Yes |
| Shared env vars API | Exists (`/shared-env-vars`) — UI thin |
| Project Danger / delete | Yes (`/projects/:id/edit`) |
| Environment Settings / delete | Yes (`…/environments/:id/edit`) |
| Clone environment | **Missing** |
| Destination wizard on New | **Missing** (auto-picks first destination) |

---

## 4. Gaps prioritized

### P0 — Project system parity (must)
1. [x] **PATCH/DELETE project** — Coolify edit + delete-if-empty + name confirmation
2. [x] **PATCH/DELETE environment** — edit + delete-if-empty
3. [x] **`is_empty` / resource counts** on project & environment for UI disable state
4. [x] **UI: Project Settings page** (`/projects/:id/edit`) with Danger zone
5. [x] **UI: Environment Settings** (`…/environments/:id/edit`) with Danger zone
6. [x] Wire list “Settings” links to real edit pages (today Settings → show which auto-redirects)

### P1 — Environment power features
1. Clone environment / project
2. Destination selection step on New Resource (like Coolify)
3. Project & environment shared variables UI tied to scope
4. Empty-state guidance when delete blocked (“delete resources first”)

### P2 — Resource depth (Coolify tabs)
1. Application: storage, backups, healthcheck, webhooks UI completeness, metrics, tags
2. Database: backups, public port, import
3. Service: stack unit management parity, terminal polish
4. Tags everywhere
5. Global search across projects/resources

---

## 5. Coolify Danger Zone — Project delete (detail)

**UI** (`delete-project.blade.php` + `edit.blade.php`):

- Button “Delete Project” sits next to Save on Project Edit.
- `:disabled="!$project->isEmpty()"` — button disabled when resources exist.
- Modal: type **exact project name** to confirm.
- Actions listed: delete project; all environments inside deleted too.

**Logic** (`DeleteProject.php`):

```php
if ($project->isEmpty()) {
    $project->delete(); // cascades environments
    return redirect to project.index;
}
return error: has resources, delete them first;
```

**`isEmpty()`** (`Project.php`): zero applications + all standalone DB types + services.

Goolify equivalent rule:

- Count apps + databases + services under all environments of the project.
- Allow DELETE only when count == 0.
- Cascade delete environments (SQL already `ON DELETE CASCADE` for envs; resources reference envs).

Same pattern for **environment** delete (count resources in that env only).

---

## 6. Frontend UX notes (Coolify)

- Everything hangs off **Projects**; no parallel resource nav.
- New Resource is one long page, not three separate hubs.
- Settings/Danger for containers live on the resource; Settings/Danger for project/env live on dedicated edit pages.
- Confirmation always requires typing the resource/project name for destructive deletes.

---

## 7. Implementation order (this repo)

1. [x] Backend: `isEmpty`, update/delete project & environment  
2. [x] Frontend: Project Edit + Environment Edit + Danger zones  
3. (Next) Clone, destination wizard, shared-var scopes UI  
4. (Later) Deep resource tabs  

Track progress by checking items in §4 as they land.
