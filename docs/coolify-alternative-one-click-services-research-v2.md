# Coolify Alternative: One-Click Docker Services Research

> **Expanded Edition v2:** This file contains the complete original research plus a second deep-research section covering services 43–85.

> **Research snapshot:** 31 July 2026  
> **Purpose:** A curated backlog of services that can be added to a Coolify-alternative platform as Docker or Docker Compose based one-click deployments.  
> **Scope:** This document focuses on services that were missing or strategically valuable compared with the existing catalog. It does not repeat every application already present in the catalog.

---

## Table of Contents

1. [Executive Summary](#executive-summary)
2. [Deployment Classes](#deployment-classes)
3. [Priority 0: Core Platform Services](#priority-0-core-platform-services)
4. [Priority 1: High-Value Application Services](#priority-1-high-value-application-services)
5. [Priority 2: Larger or More Complex Services](#priority-2-larger-or-more-complex-services)
6. [Advanced Infrastructure Services](#advanced-infrastructure-services)
7. [Mail Server Services](#mail-server-services)
8. [Recommended Initial Release Batch](#recommended-initial-release-batch)
9. [Catalog Manifest Requirements](#catalog-manifest-requirements)
10. [Example Manifest](#example-manifest)
11. [Security and Operational Rules](#security-and-operational-rules)
12. [Catalog Architecture Recommendations](#catalog-architecture-recommendations)
13. [Implementation Checklist](#implementation-checklist)
14. [Official Links Index](#official-links-index)
15. [Deep Research Expansion: Services 43–85](#deep-research-expansion-services-4385)

---

## Executive Summary

The existing catalog already contains many CMS, productivity, media, database, AI, monitoring, Git, and automation applications. The largest strategic gap is not the number of applications; it is the lack of several **core DevOps and infrastructure primitives**.

The first additions should therefore focus on:

- Metrics collection and observability
- Identity and directory services
- Private networking
- Package and container registries
- Database change management
- Backup management
- CI/CD and infrastructure automation
- Secure certificate and secret infrastructure
- Clear handling of privileged or host-integrated workloads

### Recommended first additions

1. Prometheus
2. VictoriaMetrics
3. OpenTelemetry Collector
4. Authelia
5. LLDAP
6. Headscale
7. Node-RED
8. Verdaccio
9. Bytebase
10. Ansible Semaphore
11. Backrest
12. Baserow
13. Matomo
14. PLANKA
15. Ghostfolio

---

## Deployment Classes

Every catalog template should declare a deployment class. This prevents privileged infrastructure services from being treated like ordinary web applications.

### `standard`

Suitable for normal multi-tenant application deployment.

Typical properties:

- Runs on a Docker bridge network
- One public HTTP or HTTPS domain
- No Docker socket access
- No host networking
- No privileged mode
- Uses named volumes or controlled bind mounts
- Internal databases are not publicly exposed

Examples:

- Baserow
- PLANKA
- Ghostfolio
- Etherpad
- Komga
- Kavita

### `advanced`

Requires elevated host access, device access, special mounts, extra Linux capabilities, or a Docker socket.

Examples:

- Woodpecker Agent
- Backrest
- Netdata
- cAdvisor
- Dockge

### `infrastructure`

Overlaps with the platform's networking, certificate, identity, security, storage, or reverse-proxy layer.

Examples:

- AdGuard Home
- BunkerWeb
- CrowdSec
- OpenBao
- step-ca
- Headscale

### `dedicated-host`

Should normally be deployed on a dedicated server or isolated node because of port conflicts, deliverability requirements, high resource usage, or broad host control.

Examples:

- Mailu
- Stalwart
- AzuraCast
- Large GPU-based AI stacks
- Large RAGFlow deployments

---

# Priority 0: Core Platform Services

These services add the most strategic value to a Coolify alternative.

## 1. Prometheus

- **Category:** Monitoring
- **Priority:** P0
- **Deployment class:** Standard
- **Description:** Time-series metrics collection, storage, alert evaluation, and service scraping.
- **Why add it:** Grafana can visualize metrics, but it does not replace a metrics collector and time-series database.
- **Typical internal port:** `9090`
- **Persistent data:** `/prometheus`
- **Configuration:** `/etc/prometheus/prometheus.yml`
- **Recommended template:** Single container with generated configuration file and persistent data volume.
- **Important notes:**
  - Provide a generated starter scrape configuration.
  - Allow users to add static scrape targets.
  - Add optional platform-native service discovery later.
  - Pin an explicit image version.
- **Official documentation:** https://prometheus.io/docs/prometheus/latest/installation/

## 2. VictoriaMetrics

- **Category:** Monitoring / Time-series database
- **Priority:** P0
- **Deployment class:** Standard
- **Description:** A Prometheus-compatible time-series database designed for efficient metrics storage and querying.
- **Why add it:** Gives users a resource-efficient alternative to a standard Prometheus-only storage architecture.
- **Typical single-node port:** `8428`
- **Persistent data:** `/victoria-metrics-data`
- **Recommended templates:**
  - VictoriaMetrics Single
  - VictoriaMetrics Cluster
- **Important notes:**
  - Start with the single-node edition.
  - Treat the cluster edition as an advanced multi-container template.
  - Expose Prometheus-compatible ingestion and query endpoints.
- **Official documentation:** https://docs.victoriametrics.com/victoriametrics/quick-start/

## 3. OpenTelemetry Collector

- **Category:** Observability
- **Priority:** P0
- **Deployment class:** Standard or Infrastructure
- **Description:** Vendor-neutral telemetry pipeline for receiving, processing, and exporting metrics, logs, and traces.
- **Why add it:** Creates a standard ingestion layer between applications and observability backends.
- **Common ports:**
  - OTLP gRPC: `4317`
  - OTLP HTTP: `4318`
- **Persistent data:** Usually not required for a basic gateway, depending on configured extensions.
- **Configuration:** Collector YAML file
- **Recommended templates:**
  - Core Collector
  - Contrib Collector
- **Important notes:**
  - Generate a safe starter configuration.
  - Do not expose OTLP ports publicly by default.
  - Support internal service-to-service ingestion.
- **Official documentation:** https://opentelemetry.io/docs/collector/install/docker/

## 4. Authelia

- **Category:** Authentication / Security
- **Priority:** P0
- **Deployment class:** Infrastructure
- **Description:** Authentication and authorization server commonly used with reverse proxies for SSO, access policies, and multi-factor authentication.
- **Why add it:** Provides a relatively lightweight identity gateway for protected applications.
- **Typical internal port:** `9091`
- **Dependencies:** Storage backend; Redis is optional for some deployment modes.
- **Persistent data:** Configuration and database storage
- **Important notes:**
  - Requires reverse-proxy integration.
  - Generate all JWT, session, storage, and encryption secrets.
  - Provide SQLite and PostgreSQL variants.
  - Do not publish a template with default secrets.
- **Official documentation:** https://www.authelia.com/integration/deployment/docker/

## 5. LLDAP

- **Category:** Authentication / Directory
- **Priority:** P0
- **Deployment class:** Infrastructure
- **Description:** Lightweight LDAP directory with a web interface for managing users and groups.
- **Why add it:** Useful as a central directory for Authelia, Jellyfin, Grafana, and other LDAP-capable services.
- **Common ports:**
  - Web UI: `17170`
  - LDAP: `3890`
- **Persistent data:** Application database and configuration
- **Important notes:**
  - Keep LDAP internal by default.
  - Generate JWT and admin credentials.
  - Offer SQLite and PostgreSQL-backed variants if supported by the selected release.
- **Official project:** https://github.com/lldap/lldap

## 6. Headscale

- **Category:** VPN / Networking
- **Priority:** P0
- **Deployment class:** Infrastructure
- **Description:** Self-hosted coordination server compatible with Tailscale clients.
- **Why add it:** Allows users to create private WireGuard-based device networks without relying on the hosted Tailscale control plane.
- **Persistent data:** Database and Headscale configuration
- **Important notes:**
  - Requires a stable public domain.
  - WebSocket and HTTPS proxy behavior must be tested.
  - Provide SQLite and PostgreSQL options where appropriate.
  - The client application is still required on user devices.
- **Official documentation:** https://headscale.net/stable/setup/install/container/

## 7. step-ca

- **Category:** Security / PKI
- **Priority:** P0
- **Deployment class:** Infrastructure
- **Description:** Private certificate authority for internal TLS, SSH certificates, and ACME-based certificate issuance.
- **Why add it:** Enables internal certificate automation and private PKI for self-hosted infrastructure.
- **Persistent data:** CA configuration, keys, certificates, and database
- **Important notes:**
  - Root and intermediate keys require special protection.
  - Initialization must be separated from normal startup.
  - Support backup and disaster-recovery documentation.
  - Never print generated CA passwords in normal logs.
- **Official documentation:** https://smallstep.com/docs/tutorials/docker-tls-certificate-authority/

## 8. Verdaccio

- **Category:** Package Registry
- **Priority:** P0
- **Deployment class:** Standard
- **Description:** Lightweight private npm-compatible package registry.
- **Why add it:** Adds private JavaScript package hosting, which is a major developer-platform feature.
- **Typical internal port:** `4873`
- **Persistent data:** Storage, configuration, and plugins
- **Important notes:**
  - Persist package storage and configuration.
  - Include uplink configuration for npmjs.
  - Support optional authentication and scoped packages.
  - Do not run as root.
- **Official documentation:** https://verdaccio.org/docs/docker/

## 9. Bytebase

- **Category:** Database / DevTools
- **Priority:** P0
- **Deployment class:** Standard
- **Description:** Database schema change, migration, review, approval, and database DevOps platform.
- **Why add it:** Complements hosted databases by adding controlled schema-management workflows.
- **Persistent data:** Application data or external PostgreSQL
- **Important notes:**
  - Provide an external PostgreSQL production variant.
  - Pin image versions or digests.
  - Keep managed database credentials encrypted and protected.
  - Add backup guidance before platform upgrades.
- **Official documentation:** https://docs.bytebase.com/get-started/step-by-step/deploy-with-docker

## 10. Ansible Semaphore

- **Category:** DevOps / Automation
- **Priority:** P0
- **Deployment class:** Standard or Advanced
- **Description:** Web interface and task runner for Ansible, Terraform, OpenTofu, and infrastructure automation workflows.
- **Why add it:** Gives users an infrastructure automation layer without requiring Jenkins.
- **Dependencies:** Database
- **Persistent data:** Database and project data
- **Important notes:**
  - SSH keys and vault passwords must use encrypted secret storage.
  - Outbound SSH access may be required.
  - Provide PostgreSQL and MariaDB variants only when fully maintained.
- **Official documentation:** https://docs.semaphoreui.com/administration-guide/installation

---

# Priority 1: High-Value Application Services

These are comparatively straightforward one-click deployments and broaden the catalog.

## 11. Node-RED

- **Category:** Automation / IoT
- **Priority:** P1
- **Deployment class:** Standard
- **Description:** Flow-based event automation and IoT integration platform.
- **Why add it:** Complements n8n with device, MQTT, and event-driven use cases.
- **Typical internal port:** `1880`
- **Persistent data:** `/data`
- **Important notes:**
  - Persist the complete `/data` directory.
  - Generate a credential secret.
  - Make project mode optional.
  - Support MQTT and internal service connections.
- **Official documentation:** https://nodered.org/docs/getting-started/docker

## 12. Backrest

- **Category:** Backup
- **Priority:** P1
- **Deployment class:** Advanced
- **Description:** Web-based backup orchestration and monitoring interface built around Restic.
- **Why add it:** Gives users a practical UI for scheduled encrypted backups.
- **Persistent data:** Configuration, repository credentials, and task metadata
- **Important notes:**
  - Host paths must be mounted explicitly.
  - Avoid unrestricted access to the whole host filesystem.
  - Support S3-compatible, SFTP, and local repositories.
  - Store repository passwords as platform secrets.
- **Official project:** https://github.com/garethgeorge/backrest

## 13. Baserow

- **Category:** Productivity / Database
- **Priority:** P1
- **Deployment class:** Standard
- **Description:** Open-source no-code database and Airtable-style collaboration platform.
- **Why add it:** Provides another strong no-code database option alongside NocoDB and Teable.
- **Dependencies:** PostgreSQL; optional Redis and worker services depending on deployment
- **Important notes:**
  - Use PostgreSQL for the production template.
  - Generate the public URL correctly.
  - Separate background workers where required.
  - Add SMTP as an optional integration.
- **Official documentation:** https://baserow.io/docs/installation%2Finstall-with-docker

## 14. Matomo

- **Category:** Analytics
- **Priority:** P1
- **Deployment class:** Standard
- **Description:** Full-featured web analytics platform and Google Analytics alternative.
- **Why add it:** Complements lightweight analytics tools with a more comprehensive analytics suite.
- **Dependencies:** MariaDB or MySQL
- **Important notes:**
  - Include a scheduled archive or cron service.
  - Keep the database private.
  - Persist application configuration and database data.
  - Recommend object storage or external archival strategy for large installations.
- **Official documentation:** https://matomo.org/faq/how-to-install/install-matomo-with-docker/

## 15. PLANKA

- **Category:** Productivity / Project Management
- **Priority:** P1
- **Deployment class:** Standard
- **Description:** Open-source Kanban project-management application.
- **Why add it:** Provides a lightweight Trello-style board.
- **Dependencies:** PostgreSQL
- **Important notes:**
  - Generate application secrets.
  - Persist user uploads.
  - Set the public base URL.
  - Include optional SMTP configuration.
- **Official documentation:** https://docs.planka.cloud/docs/installation/docker/production-version/

## 16. Ghostfolio

- **Category:** Finance
- **Priority:** P1
- **Deployment class:** Standard
- **Description:** Open-source wealth and investment portfolio management application.
- **Why add it:** Adds a polished finance application for portfolio tracking.
- **Dependencies:** PostgreSQL and Redis
- **Important notes:**
  - Generate access and JWT secrets.
  - Keep financial data backups enabled.
  - External market-data integrations may require API credentials.
- **Official project:** https://github.com/ghostfolio/ghostfolio

## 17. Etherpad

- **Category:** Productivity / Collaboration
- **Priority:** P1
- **Deployment class:** Standard
- **Description:** Real-time collaborative text and document editor.
- **Why add it:** Lightweight collaborative editing with a mature plugin ecosystem.
- **Dependencies:** PostgreSQL, MariaDB, or another supported database for production
- **Important notes:**
  - Avoid ephemeral in-memory storage.
  - Persist database data and plugin configuration.
  - Generate session and API keys.
- **Official documentation:** https://docs.etherpad.org/docker.html

## 18. Docspell

- **Category:** Document Management
- **Priority:** P1
- **Deployment class:** Standard
- **Description:** Document organizer with tagging, search, text extraction, and automated document processing.
- **Why add it:** Provides an alternative to Paperless-ngx.
- **Dependencies:** Multi-container stack depending on deployment mode
- **Important notes:**
  - Persist database, file, and search data.
  - Document OCR language availability.
  - Consider a larger default memory recommendation.
- **Official documentation:** https://docspell.org/docs/install/docker/

## 19. Komga

- **Category:** Media
- **Priority:** P1
- **Deployment class:** Standard
- **Description:** Media server for comics, manga, magazines, and ebooks.
- **Why add it:** Covers a focused library-management use case not served by general media servers.
- **Typical internal port:** `25600`
- **Persistent data:** Configuration and media library mounts
- **Important notes:**
  - Media should be read-only by default.
  - Configuration must be writable and backed up.
  - Support large library scan timeouts.
- **Official documentation:** https://komga.org/docs/installation/docker/

## 20. Kavita

- **Category:** Media
- **Priority:** P1
- **Deployment class:** Standard
- **Description:** Self-hosted reading server for manga, comics, and ebooks.
- **Why add it:** Popular alternative to Komga with a different reading and metadata experience.
- **Persistent data:** Configuration and library directories
- **Important notes:**
  - Use read-only media mounts where possible.
  - Persist configuration separately.
  - Document supported file formats.
- **Official documentation:** https://wiki.kavitareader.com/installation/docker/

---

# Priority 2: Larger or More Complex Services

These services are valuable but need more resources, more containers, or special lifecycle handling.

## 21. Harbor

- **Category:** Container Registry / Security
- **Priority:** P2
- **Deployment class:** Infrastructure
- **Description:** Enterprise-grade OCI container registry with projects, RBAC, replication, scanning integrations, and a web interface.
- **Why add it:** A major upgrade over a basic Docker Registry.
- **Important notes:**
  - It is a large multi-container stack.
  - Requires significant CPU, memory, and disk compared with a basic registry.
  - Needs a stable domain and TLS.
  - Registry storage backup and garbage collection must be documented.
- **Official documentation:** https://goharbor.io/docs/main/install-config/installation-prereqs/

## 22. Woodpecker CI

- **Category:** CI/CD
- **Priority:** P2
- **Deployment class:** Advanced
- **Description:** Lightweight, container-native continuous integration platform.
- **Why add it:** Provides a smaller alternative to Jenkins and GitLab CI.
- **Components:** Server and one or more agents
- **Important notes:**
  - Docker-based agents commonly require Docker socket access.
  - The agent must be clearly marked as host-privileged.
  - Support GitHub, Gitea, and Forgejo OAuth configuration.
  - Keep server and agent templates separate.
- **Official documentation:** https://woodpecker-ci.org/docs/administration/installation/docker-compose

## 23. Dockge

- **Category:** Docker Management
- **Priority:** P2
- **Deployment class:** Advanced
- **Description:** Web interface for managing Docker Compose stacks.
- **Why add it:** Useful for administrators who want direct Compose management.
- **Important notes:**
  - Requires Docker socket access.
  - Can overlap with the platform's own application-management functionality.
  - Better suited as an internal administration application than a public catalog item.
- **Official project:** https://github.com/louislam/dockge

## 24. LocalAI

- **Category:** AI
- **Priority:** P2
- **Deployment class:** Standard, Advanced, or Dedicated Host
- **Description:** Local OpenAI-compatible inference API supporting multiple model backends.
- **Why add it:** Lets users run AI models without a hosted model provider.
- **Important notes:**
  - Create separate CPU, NVIDIA GPU, and AMD GPU templates.
  - Model storage can be very large.
  - GPU templates require device and runtime configuration.
  - Do not download huge models automatically without user confirmation.
- **Official documentation:** https://localai.io/installation/containers/index.html

## 25. Tabby

- **Category:** AI / Developer Tools
- **Priority:** P2
- **Deployment class:** Advanced or Dedicated Host
- **Description:** Self-hosted AI coding assistant and code-completion server.
- **Why add it:** Adds a developer-focused AI workload.
- **Important notes:**
  - GPU is strongly recommended for useful performance.
  - Model storage requires persistent disk.
  - Provide explicit NVIDIA and CPU variants.
  - Add model-size and VRAM guidance.
- **Official documentation:** https://tabby.tabbyml.com/docs/quick-start/installation/docker/

## 26. RAGFlow

- **Category:** AI / RAG
- **Priority:** P2
- **Deployment class:** Dedicated Host
- **Description:** Retrieval-augmented generation platform with document ingestion, search, knowledge bases, and AI workflows.
- **Why add it:** Provides an integrated RAG application stack.
- **Important notes:**
  - It is a large multi-container deployment.
  - Dependencies can include MySQL, object storage, Redis, and a search engine.
  - Publish a high memory and disk recommendation.
  - Validate architecture support before enabling ARM.
- **Official documentation:** https://ragflow.io/docs/dev/

## 27. ONLYOFFICE Docs

- **Category:** Productivity / Office
- **Priority:** P2
- **Deployment class:** Standard or Dedicated Host
- **Description:** Browser-based document, spreadsheet, and presentation editing server.
- **Why add it:** Useful with Nextcloud, ownCloud, and other document-management systems.
- **Important notes:**
  - Requires substantial memory compared with small web apps.
  - JWT must be enabled with a generated secret.
  - Persistent document and log paths are required.
  - Integration with another application is usually necessary.
- **Official documentation:** https://helpcenter.onlyoffice.com/docs/installation/docs-community-install-docker.aspx

## 28. CryptPad

- **Category:** Productivity / Privacy
- **Priority:** P2
- **Deployment class:** Infrastructure
- **Description:** Privacy-focused collaborative office and document platform using client-side encryption.
- **Why add it:** Strong privacy-oriented collaboration option.
- **Important notes:**
  - Common deployments require both a main domain and a sandbox domain.
  - The platform must support multiple domains per service.
  - Security headers and reverse-proxy behavior require careful validation.
- **Official documentation:** https://docs.cryptpad.org/en/admin_guide/installation.html

## 29. OpenProject

- **Category:** Productivity / Project Management
- **Priority:** P2
- **Deployment class:** Standard
- **Description:** Mature project-management platform with work packages, planning, roadmaps, and team collaboration.
- **Why add it:** Strong enterprise-oriented project-management option.
- **Important notes:**
  - Prefer the documented multi-process Compose deployment for production.
  - Do not treat a development all-in-one image as a production template.
  - Include database, cache, worker, and cron lifecycle handling.
- **Official documentation:** https://www.openproject.org/docs/installation-and-operations/installation/docker/

## 30. PhotoPrism

- **Category:** Media / Photos
- **Priority:** P2
- **Deployment class:** Standard
- **Description:** Photo library, indexing, search, and media-management platform.
- **Why add it:** Alternative to Immich with different indexing and library-management behavior.
- **Important notes:**
  - Indexing can be CPU- and memory-intensive.
  - Avoid overly restrictive memory limits.
  - Mount original media read-only where possible.
  - Persist storage, database, and sidecar data.
- **Official documentation:** https://docs.photoprism.app/getting-started/docker-compose/

## 31. Owncast

- **Category:** Media / Streaming
- **Priority:** P2
- **Deployment class:** Infrastructure
- **Description:** Self-hosted live video-streaming and chat server.
- **Why add it:** Adds independent live-streaming capability.
- **Common ports:**
  - Web interface and playback: `8080`
  - RTMP ingest: `1935`
- **Important notes:**
  - Requires both HTTP and raw TCP routing.
  - Bandwidth and storage usage can be high.
  - RTMP must not be routed through an HTTP-only proxy.
- **Official documentation:** https://owncast.online/quickstart/container/

## 32. AzuraCast

- **Category:** Media / Radio
- **Priority:** P2
- **Deployment class:** Dedicated Host
- **Description:** Complete self-hosted web-radio management and broadcasting platform.
- **Why add it:** Adds a specialized streaming and station-management workload.
- **Important notes:**
  - Uses multiple ports and broad host networking assumptions.
  - Better suited to a clean dedicated server.
  - Storage and bandwidth requirements vary heavily by station.
- **Official documentation:** https://www.azuracast.com/docs/getting-started/installation/docker/

---

# Advanced Infrastructure Services

These templates require explicit warnings and stricter installation controls.

## 33. Netdata

- **Category:** Monitoring
- **Deployment class:** Advanced
- **Description:** Real-time host, container, system, and application monitoring.
- **Risk reason:** Complete monitoring may require host PID, host network, system mounts, extra capabilities, and Docker socket access.
- **Template requirement:** Show every requested mount and capability before deployment.
- **Official documentation:** https://learn.netdata.cloud/docs/netdata-agent/installation/docker

## 34. cAdvisor

- **Category:** Monitoring
- **Deployment class:** Advanced
- **Description:** Container resource usage and performance collector.
- **Risk reason:** Requires access to host control groups, system paths, Docker data paths, and runtime information.
- **Template requirement:** Use read-only mounts where possible and avoid public exposure.
- **Official project:** https://github.com/google/cadvisor

## 35. CrowdSec

- **Category:** Security
- **Deployment class:** Infrastructure
- **Description:** Collaborative security engine that analyzes logs and identifies malicious behavior.
- **Risk reason:** The engine alone does not block traffic; it needs correct log sources and a remediation component or bouncer.
- **Template requirement:** Clearly separate detection and remediation components.
- **Official documentation:** https://docs.crowdsec.net/u/getting_started/installation/docker/

## 36. AdGuard Home

- **Category:** DNS / Networking
- **Deployment class:** Infrastructure
- **Description:** Network-wide DNS filtering, privacy, parental controls, and local DNS management.
- **Risk reason:** Requires DNS ports, UDP support, real client IP visibility, and sometimes host networking.
- **Template requirement:** Detect port `53` conflicts before deployment.
- **Official documentation:** https://github.com/AdguardTeam/AdGuardHome/wiki/Docker

## 37. BunkerWeb

- **Category:** Security / Reverse Proxy / WAF
- **Deployment class:** Infrastructure
- **Description:** Web application firewall and reverse-proxy security platform.
- **Risk reason:** Can conflict directly with the platform's own reverse proxy and ports `80` and `443`.
- **Template requirement:** Deploy only on dedicated proxy nodes or in an explicitly supported integration mode.
- **Official documentation:** https://docs.bunkerweb.io/latest/quickstart-guide/

## 38. Loki

- **Category:** Logging
- **Deployment class:** Infrastructure
- **Description:** Log aggregation system designed to integrate with Grafana.
- **Risk reason:** A production deployment needs deliberate storage, retention, authentication, and ingestion design.
- **Template requirement:** Start with a clearly labeled single-node template; add distributed mode separately.
- **Official documentation:** https://grafana.com/docs/loki/latest/setup/install/

## 39. OpenBao

- **Category:** Security / Secrets
- **Deployment class:** Infrastructure
- **Description:** Open-source secrets management, encryption, identity, and PKI platform derived from the Vault ecosystem.
- **Risk reason:** Initialization, seal state, unseal or recovery keys, storage backend, and disaster recovery require special lifecycle handling.
- **Template requirement:** Implement a post-deployment initialization workflow rather than pretending the service is ready after container start.
- **Official documentation:** https://openbao.org/docs/commands/server/

## 40. Jaeger

- **Category:** Observability / Tracing
- **Deployment class:** Standard or Infrastructure
- **Description:** Distributed tracing backend and trace exploration interface.
- **Risk reason:** The all-in-one image is convenient but often uses non-production storage defaults.
- **Template requirement:** Provide separate development and production variants with external storage.
- **Official documentation:** https://www.jaegertracing.io/docs/latest/deployment/

---

# Mail Server Services

Mail servers need a dedicated template category because Docker deployment alone is not enough. DNS, IP reputation, reverse DNS, TLS, deliverability, anti-spam, and multiple host ports are core requirements.

## 41. Stalwart

- **Category:** Email / Groupware
- **Deployment class:** Dedicated Host
- **Description:** Modern mail and collaboration server supporting SMTP, IMAP, JMAP, CalDAV, CardDAV, and related protocols.
- **Why add it:** A relatively integrated modern mail-server option.
- **Important notes:**
  - Requires public mail ports.
  - Requires MX, SPF, DKIM, DMARC, and reverse DNS guidance.
  - Use pinned release versions.
  - Store configuration, queues, indexes, and mail data persistently.
  - Add preflight checks for blocked SMTP ports.
- **Official documentation:** https://stalw.art/docs/install/platform/docker/

## 42. Mailu

- **Category:** Email
- **Deployment class:** Dedicated Host
- **Description:** Complete containerized mail-server suite with SMTP, IMAP, webmail, filtering, and administration components.
- **Why add it:** Mature multi-container mail stack.
- **Important notes:**
  - Requires many containers and host ports.
  - Antivirus and filtering increase memory usage.
  - Requires correct DNS before production use.
  - Must not share conflicting mail ports with another mail stack.
- **Official documentation:** https://mailu.io/master/compose/setup.html

---

# Recommended Initial Release Batch

The following sequence balances user value, Docker compatibility, and implementation complexity.

## Batch A: Core observability and platform tooling

1. Prometheus
2. VictoriaMetrics Single
3. OpenTelemetry Collector
4. Verdaccio
5. Bytebase

## Batch B: Identity and private infrastructure

6. Authelia
7. LLDAP
8. Headscale
9. step-ca

## Batch C: Automation, backup, and productivity

10. Node-RED
11. Ansible Semaphore
12. Backrest
13. Baserow
14. PLANKA
15. Matomo
16. Ghostfolio

## Batch D: Advanced operator tooling

17. Harbor
18. Woodpecker CI Server
19. Woodpecker CI Agent
20. OpenBao
21. Loki
22. Jaeger
23. Netdata
24. cAdvisor

## Batch E: Specialized workloads

25. LocalAI
26. Tabby
27. RAGFlow
28. ONLYOFFICE Docs
29. CryptPad
30. PhotoPrism
31. Owncast
32. Stalwart
33. Mailu

---

# Catalog Manifest Requirements

A one-click catalog schema should include enough metadata to validate a deployment before creating containers.

## Identity

```yaml
id:
name:
slug:
description:
category:
tags:
logo:
website:
documentation:
sourceRepository:
license:
```

## Image policy

```yaml
image:
  repository:
  tag:
  digest:
  official:
  versionStrategy:
  allowLatest:
  architectures:
```

Recommended rules:

- `allowLatest` should default to `false`.
- A release tag or digest should be pinned.
- Supported architectures must be declared.
- Community images must be clearly marked.
- Image provenance should be recorded.

## Deployment requirements

```yaml
deployment:
  class:
  minimumResources:
    cpu:
    memory:
    storage:
  recommendedResources:
    cpu:
    memory:
    storage:
  requiresDedicatedHost:
  requiresGpu:
  requiresDockerSocket:
  requiresHostNetwork:
  requiresHostPid:
  requiresPrivileged:
  requiredCapabilities:
  requiredDevices:
  requiredSysctls:
```

## Networking

```yaml
network:
  domains:
  internalPorts:
  fixedHostPorts:
  protocols:
  tcpPorts:
  udpPorts:
  websocket:
  publicByDefault:
```

The schema should support:

- Multiple domains
- HTTP, HTTPS, TCP, and UDP
- Fixed host-port conflict detection
- Internal-only ports
- WebSocket upgrades
- Custom healthcheck ports

## Storage

```yaml
storage:
  volumes:
    - name:
      containerPath:
      mode:
      backup:
      minimumSize:
  bindMounts:
    - hostPath:
      containerPath:
      mode:
      risk:
```

Recommended rules:

- Stateful services cannot be deployed without persistence.
- Host bind mounts require a warning.
- Sensitive directories should not be mounted broadly.
- Read-only mounts should be preferred for media and log sources.

## Secrets

```yaml
secrets:
  generated:
    - name:
      length:
      encoding:
  required:
    - name:
      description:
  rotationSupported:
```

The platform should:

- Generate cryptographically secure values.
- Never put secrets in image arguments.
- Avoid printing secrets to normal logs.
- Distinguish secrets from ordinary environment variables.
- Support one-time display or secret export where necessary.

## Dependencies

```yaml
dependencies:
  services:
  optionalServices:
  externalServices:
  initJobs:
  migrations:
```

The catalog should support:

- Databases
- Redis
- Object storage
- Search engines
- Workers
- Cron containers
- Initialization jobs
- Database migrations

## Health and lifecycle

```yaml
healthcheck:
  type:
  port:
  path:
  interval:
  timeout:
  retries:
  startPeriod:

lifecycle:
  preDeploy:
  postDeploy:
  preUpdate:
  postUpdate:
  preBackup:
  postRestore:
  migrationCommand:
  updateStrategy:
  rollbackSupported:
```

## Backup metadata

```yaml
backup:
  enabledByDefault:
  paths:
  databases:
  preBackupCommands:
  restoreCommands:
  consistencyNotes:
```

---

# Example Manifest

```yaml
id: prometheus
name: Prometheus
slug: prometheus
description: Open-source metrics monitoring and time-series database.
category: monitoring
tags:
  - metrics
  - observability
  - alerting

website: https://prometheus.io/
documentation: https://prometheus.io/docs/prometheus/latest/installation/
sourceRepository: https://github.com/prometheus/prometheus
license: Apache-2.0

image:
  repository: prom/prometheus
  tag: "<PINNED_VERSION>"
  official: true
  versionStrategy: pinned
  allowLatest: false
  architectures:
    - amd64
    - arm64

deployment:
  class: standard
  minimumResources:
    cpu: "0.5"
    memory: 512Mi
    storage: 10Gi
  recommendedResources:
    cpu: "1"
    memory: 1Gi
    storage: 50Gi
  requiresDedicatedHost: false
  requiresGpu: false
  requiresDockerSocket: false
  requiresHostNetwork: false
  requiresHostPid: false
  requiresPrivileged: false
  requiredCapabilities: []
  requiredDevices: []
  requiredSysctls: []

network:
  domains:
    min: 0
    max: 1
  internalPorts:
    - port: 9090
      protocol: http
      publicByDefault: true
  fixedHostPorts: []
  websocket: false

storage:
  volumes:
    - name: prometheus-data
      containerPath: /prometheus
      mode: rw
      backup: true
      minimumSize: 10Gi

configuration:
  generatedFiles:
    - source: templates/prometheus.yml
      destination: /etc/prometheus/prometheus.yml

secrets:
  generated: []
  required: []
  rotationSupported: false

healthcheck:
  type: http
  port: 9090
  path: /-/healthy
  interval: 30s
  timeout: 5s
  retries: 5
  startPeriod: 20s

backup:
  enabledByDefault: true
  paths:
    - /prometheus
  databases: []
  consistencyNotes: >
    Coordinate snapshot or shutdown behavior for large production instances.

lifecycle:
  preDeploy: []
  postDeploy: []
  preUpdate:
    - create-backup
  postUpdate:
    - verify-healthcheck
  migrationCommand: null
  updateStrategy: recreate
  rollbackSupported: true
```

---

# Security and Operational Rules

## 1. Do not use `latest` by default

Use a pinned semantic version or immutable image digest.

Bad:

```yaml
image: example/service:latest
```

Better:

```yaml
image: example/service:1.2.3
```

Best for reproducibility:

```yaml
image: example/service@sha256:<DIGEST>
```

## 2. Treat Docker socket access as host-level access

Any service using:

```text
/var/run/docker.sock
```

must be marked as advanced and high risk.

The installation page should display:

- Why socket access is needed
- Whether read-only mode is enough
- What the service can control
- Which host is affected

## 3. Never publish databases by default

PostgreSQL, MySQL, MariaDB, Redis, search engines, and internal queues should stay on private Docker networks unless the user explicitly enables external access.

## 4. Generate secure secrets

Never ship default values such as:

```text
admin
password
changeme
secret
```

Use platform-generated values for:

- JWT secrets
- Session secrets
- Encryption keys
- Database passwords
- API tokens
- OAuth state secrets
- Initial admin passwords

## 5. Require persistence for stateful services

Block deployment when a stateful template has no persistent volume.

Examples:

- Databases
- Registries
- Monitoring storage
- User-upload applications
- Password managers
- Identity services
- Mail servers

## 6. Validate fixed port conflicts

Before deployment, detect conflicts for:

- `53/tcp` and `53/udp`
- `80/tcp`
- `443/tcp`
- `25/tcp`
- `465/tcp`
- `587/tcp`
- `993/tcp`
- `1935/tcp`
- VPN ports
- Game-server ports

## 7. Add automatic pre-update backups

For stateful applications:

1. Run the service-specific pre-backup command.
2. Back up volumes and databases.
3. Record the currently deployed image.
4. Perform the update.
5. Wait for healthchecks.
6. Roll back when supported.

## 8. Separate initialization from startup

Some applications require one-time initialization.

Examples:

- OpenBao initialization and unseal
- step-ca initialization
- Initial admin creation
- Database migrations
- Mail-server domain setup
- Object-storage bucket creation

A running container does not always mean a ready application.

## 9. Protect internal administrative interfaces

Administrative dashboards should support:

- Private-network-only mode
- Authentication middleware
- IP allowlists
- SSO integration
- Optional public domain
- Explicit public-exposure warning

## 10. Include resource recommendations

Each template should declare:

- Minimum CPU
- Recommended CPU
- Minimum RAM
- Recommended RAM
- Minimum storage
- Expected storage growth
- GPU and VRAM requirements
- Architecture support

---

# Catalog Architecture Recommendations

## Use variants instead of duplicate service entries

Do not create a separate top-level catalog item for every database combination.

Instead of:

```text
Forgejo with MariaDB
Forgejo with MySQL
Forgejo with PostgreSQL
Forgejo with Runner
Forgejo with Runner and MariaDB
Forgejo with Runner and MySQL
Forgejo with Runner and PostgreSQL
```

Use:

```text
Forgejo
├── Database
│   ├── PostgreSQL
│   ├── MySQL
│   └── MariaDB
├── Runner
│   ├── Disabled
│   └── Enabled
└── Storage
    ├── Local
    └── S3-compatible
```

The same model should be used for:

- WordPress database variants
- Directus database variants
- Nextcloud database variants
- Uptime Kuma database variants
- Keycloak database variants
- n8n worker and database variants
- Grafana database variants
- Pocket ID database variants

Benefits:

- Less catalog duplication
- Easier version maintenance
- Fewer outdated templates
- Cleaner user experience
- Shared upgrade and backup logic
- Easier testing

## Normalize category names

Use lowercase canonical identifiers:

```yaml
categories:
  - ai
  - analytics
  - auth
  - automation
  - backend
  - backup
  - ci-cd
  - cms
  - database
  - developer-tools
  - documentation
  - email
  - finance
  - games
  - git
  - helpdesk
  - media
  - messaging
  - monitoring
  - networking
  - productivity
  - proxy
  - search
  - security
  - storage
  - vpn
```

Avoid inconsistent values such as:

```text
Networking
networking
RSS
rss
database,observability,developer-tools
```

Use a primary category plus tags:

```yaml
category: database
tags:
  - observability
  - developer-tools
```

## Keep application and infrastructure catalogs visually separate

Suggested top-level navigation:

```text
Applications
Developer Tools
Databases
AI
Monitoring
Storage
Networking
Security
Infrastructure
Dedicated Host
```

Services marked `advanced`, `infrastructure`, or `dedicated-host` should display a visible badge.

## Add compatibility metadata

Each service variant should state:

- `amd64`
- `arm64`
- Other architectures
- Rootless compatibility
- Docker Compose compatibility
- Docker Swarm compatibility
- GPU requirements
- IPv6 support
- Multiple-domain requirements
- Raw TCP or UDP requirements

## Add quality levels

Suggested template statuses:

```text
experimental
community
verified
official
deprecated
```

Definitions:

- **experimental:** Not fully tested.
- **community:** Maintained but not officially verified by the platform.
- **verified:** Tested by the platform against a pinned release.
- **official:** Maintained by the platform or upstream integration team.
- **deprecated:** Kept for migration only; new installations disabled.

---

# Implementation Checklist

Use this checklist for every new service.

## Research

- [ ] Confirm official project website.
- [ ] Confirm official source repository.
- [ ] Confirm official or trusted container image.
- [ ] Confirm license.
- [ ] Confirm supported architectures.
- [ ] Identify stable release channel.
- [ ] Identify all required services.
- [ ] Identify all ports and protocols.
- [ ] Identify persistent paths.
- [ ] Identify generated secrets.
- [ ] Identify initialization steps.
- [ ] Identify migration steps.
- [ ] Identify backup and restore process.
- [ ] Identify resource requirements.
- [ ] Identify privileged requirements.
- [ ] Identify reverse-proxy requirements.
- [ ] Identify multi-domain requirements.

## Template implementation

- [ ] Pin an explicit image version.
- [ ] Add internal Docker network.
- [ ] Keep databases private.
- [ ] Add named volumes.
- [ ] Add generated secrets.
- [ ] Add configuration generation.
- [ ] Add healthcheck.
- [ ] Add startup grace period.
- [ ] Add pre-update backup.
- [ ] Add rollback metadata.
- [ ] Add architecture constraints.
- [ ] Add port-conflict checks.
- [ ] Add host-access warnings.
- [ ] Add minimum resource validation.

## Testing

- [ ] Fresh installation succeeds.
- [ ] Application is reachable through HTTPS.
- [ ] Restart preserves data.
- [ ] Server reboot preserves data.
- [ ] Backup succeeds.
- [ ] Restore succeeds.
- [ ] Minor update succeeds.
- [ ] Major update behavior is documented.
- [ ] Invalid credentials fail safely.
- [ ] Default secrets are not present.
- [ ] Database is not publicly exposed.
- [ ] ARM image works when advertised.
- [ ] Healthcheck reports real readiness.
- [ ] Removal does not delete data without confirmation.

---

# Official Links Index

| Service | Official documentation or project |
|---|---|
| Prometheus | https://prometheus.io/docs/prometheus/latest/installation/ |
| VictoriaMetrics | https://docs.victoriametrics.com/victoriametrics/quick-start/ |
| OpenTelemetry Collector | https://opentelemetry.io/docs/collector/install/docker/ |
| Authelia | https://www.authelia.com/integration/deployment/docker/ |
| LLDAP | https://github.com/lldap/lldap |
| Headscale | https://headscale.net/stable/setup/install/container/ |
| step-ca | https://smallstep.com/docs/tutorials/docker-tls-certificate-authority/ |
| Node-RED | https://nodered.org/docs/getting-started/docker |
| Verdaccio | https://verdaccio.org/docs/docker/ |
| Bytebase | https://docs.bytebase.com/get-started/step-by-step/deploy-with-docker |
| Ansible Semaphore | https://docs.semaphoreui.com/administration-guide/installation |
| Backrest | https://github.com/garethgeorge/backrest |
| Baserow | https://baserow.io/docs/installation%2Finstall-with-docker |
| Matomo | https://matomo.org/faq/how-to-install/install-matomo-with-docker/ |
| PLANKA | https://docs.planka.cloud/docs/installation/docker/production-version/ |
| Ghostfolio | https://github.com/ghostfolio/ghostfolio |
| Etherpad | https://docs.etherpad.org/docker.html |
| Docspell | https://docspell.org/docs/install/docker/ |
| Komga | https://komga.org/docs/installation/docker/ |
| Kavita | https://wiki.kavitareader.com/installation/docker/ |
| Harbor | https://goharbor.io/docs/main/install-config/installation-prereqs/ |
| Woodpecker CI | https://woodpecker-ci.org/docs/administration/installation/docker-compose |
| Dockge | https://github.com/louislam/dockge |
| LocalAI | https://localai.io/installation/containers/index.html |
| Tabby | https://tabby.tabbyml.com/docs/quick-start/installation/docker/ |
| RAGFlow | https://ragflow.io/docs/dev/ |
| ONLYOFFICE Docs | https://helpcenter.onlyoffice.com/docs/installation/docs-community-install-docker.aspx |
| CryptPad | https://docs.cryptpad.org/en/admin_guide/installation.html |
| OpenProject | https://www.openproject.org/docs/installation-and-operations/installation/docker/ |
| PhotoPrism | https://docs.photoprism.app/getting-started/docker-compose/ |
| Owncast | https://owncast.online/quickstart/container/ |
| AzuraCast | https://www.azuracast.com/docs/getting-started/installation/docker/ |
| Netdata | https://learn.netdata.cloud/docs/netdata-agent/installation/docker |
| cAdvisor | https://github.com/google/cadvisor |
| CrowdSec | https://docs.crowdsec.net/u/getting_started/installation/docker/ |
| AdGuard Home | https://github.com/AdguardTeam/AdGuardHome/wiki/Docker |
| BunkerWeb | https://docs.bunkerweb.io/latest/quickstart-guide/ |
| Loki | https://grafana.com/docs/loki/latest/setup/install/ |
| OpenBao | https://openbao.org/docs/commands/server/ |
| Jaeger | https://www.jaegertracing.io/docs/latest/deployment/ |
| Stalwart | https://stalw.art/docs/install/platform/docker/ |
| Mailu | https://mailu.io/master/compose/setup.html |

---

## Final Recommendation

Do not measure catalog quality only by the number of applications.

A high-quality Coolify alternative should provide:

- Reproducible pinned deployments
- Reliable healthchecks
- Safe secret generation
- Correct persistent storage
- Backup and restore support
- Upgrade and rollback workflows
- Architecture compatibility
- Clear host-access warnings
- TCP and UDP routing
- Multiple-domain support
- Template variants instead of duplicated catalog entries
- A strict distinction between applications and infrastructure

The strongest first milestone is a verified batch of 10–15 services with complete lifecycle support rather than dozens of partially working Compose files.

---

# Deep Research Expansion: Services 43–85

> **Expanded research date:** 31 July 2026  
> **New services in this edition:** 43  
> **Total researched candidates across both sections:** 85  
> **Selection rule:** Each candidate was checked against the supplied catalog and the first research edition to reduce duplicate recommendations. Preference was given to official container images, official Docker/Compose documentation, and services that add a missing platform capability.

## What This Expansion Adds

The first edition concentrated on foundational monitoring, identity, networking, backup, collaboration, and developer tooling. This expansion goes deeper into:

- Complete observability pipelines
- Analytical and time-series databases
- Search and vector infrastructure
- Messaging and durable workflow engines
- API gateways and backend primitives
- Software supply-chain security
- SIEM, vulnerability, and privileged-access systems
- General-purpose backup and web archiving
- Enterprise document management
- Hardware-connected home automation
- GPU model serving and enterprise AI search

## Important Upstream and Version Traps

### Temporal Compose repository change

The older `temporalio/docker-compose` repository was archived in January 2026. New catalog work should use the maintained `temporalio/samples-server` repository as the current reference. Do not silently keep copying an archived Compose file.

### InfluxDB moving `latest` tag

InfluxData documented an upcoming change for **15 September 2026**, when the Docker `latest` tag is scheduled to point to InfluxDB 3 Core. The catalog must use explicit major and minor versions and should represent InfluxDB 2 and InfluxDB 3 as different variants.

### Tempo distributed-mode dependency

Grafana Tempo's newer distributed architecture requires a Kafka-compatible queue. A single-binary local template and a distributed production template are materially different products from an operational perspective.

### Milvus stable-versus-beta documentation

Some current documentation paths can display beta-version examples. The platform should pin a stable Milvus release and must not generate templates directly from a moving documentation branch.

### Host-connected applications

Zigbee2MQTT, Z-Wave JS UI, Frigate, Homebridge, Wazuh agents, and similar workloads cannot be considered “perfect generic Docker apps.” They need explicit device, network, host, or agent-enrollment support.

---

## Expansion Summary Table

| # | Service | Category | Deployment class | Priority | Recommendation |
|---:|---|---|---|---|---|
| 43 | **Grafana Alloy** | Observability / Telemetry Collector | `standard` | **P0** | Add now |
| 44 | **Grafana Tempo** | Observability / Distributed Tracing | `standard for monolithic; infrastructure for distributed` | **P1** | Add monolithic first |
| 45 | **Grafana Mimir** | Observability / Metrics Storage | `standard for monolithic; infrastructure for distributed` | **P1** | Add monolithic first |
| 46 | **Prometheus Alertmanager** | Monitoring / Alerting | `standard` | **P0** | Add now |
| 47 | **Gatus** | Monitoring / Status | `standard` | **P0** | Add now |
| 48 | **Zabbix** | Monitoring / Infrastructure | `infrastructure` | **P2** | Add after core monitoring |
| 49 | **Checkmk Raw Edition** | Monitoring / Infrastructure | `infrastructure` | **P2** | Good advanced candidate |
| 50 | **ClickHouse** | Database / Analytics | `standard` | **P0** | Add now |
| 51 | **TimescaleDB** | Database / Time Series | `standard` | **P0** | Add now with production warning |
| 52 | **InfluxDB** | Database / Time Series | `standard` | **P1** | Add only with explicit version selection |
| 53 | **QuestDB** | Database / Time Series | `standard` | **P1** | Add after TimescaleDB |
| 54 | **OpenSearch and OpenSearch Dashboards** | Search / Analytics | `infrastructure` | **P1** | Add as a multi-container stack |
| 55 | **Apache Solr** | Search | `standard or infrastructure` | **P1** | Add standalone first |
| 56 | **NATS with JetStream** | Messaging | `standard` | **P0** | Add now |
| 57 | **Redpanda** | Messaging / Streaming | `infrastructure` | **P2** | Add a development single-node template first |
| 58 | **Temporal** | Workflow / Durable Execution | `infrastructure` | **P2** | Add only after lifecycle support |
| 59 | **Kestra** | Workflow / Automation | `standard` | **P1** | Add PostgreSQL variant |
| 60 | **Apache APISIX** | API Gateway | `infrastructure` | **P1** | Add after TCP routing and secret controls |
| 61 | **Hasura GraphQL Engine** | Backend / API | `standard` | **P1** | Add the open-source engine as a pinned variant |
| 62 | **Milvus** | Database / Vector Search | `infrastructure` | **P2** | Add stable standalone mode first |
| 63 | **MLflow** | AI / MLOps | `standard` | **P1** | Add a secure tracking-server stack |
| 64 | **ZITADEL** | Authentication / Identity | `infrastructure` | **P0** | Strong identity candidate |
| 65 | **Kanidm** | Authentication / Directory | `infrastructure` | **P1** | Add after LLDAP |
| 66 | **Casdoor** | Authentication / Identity | `standard or infrastructure` | **P1** | Good additional identity option |
| 67 | **NetBird Self-Hosted** | VPN / Networking | `infrastructure` | **P2** | Add only as a coordinated stack |
| 68 | **OWASP Dependency-Track** | Security / Software Supply Chain | `standard` | **P0** | Add now |
| 69 | **OWASP DefectDojo** | Security / Vulnerability Management | `standard or infrastructure` | **P1** | Add a reviewed production Compose variant |
| 70 | **Wazuh** | Security / SIEM / XDR | `dedicated-host` | **P2** | Advanced verified stack only |
| 71 | **Teleport Community Edition** | Security / Access | `infrastructure` | **P2** | Add only after multi-port ingress support |
| 72 | **Kopia** | Backup | `advanced` | **P0** | Add now with restricted mounts |
| 73 | **BorgWarehouse** | Backup | `advanced` | **P1** | Good Borg-focused candidate |
| 74 | **ArchiveBox** | Archiving / Knowledge | `standard` | **P0** | Add now |
| 75 | **Linkwarden** | Bookmarks / Archiving | `standard` | **P0** | Add now |
| 76 | **Mayan EDMS** | Document Management | `standard` | **P1** | Add as a complete Compose stack |
| 77 | **Papermerge DMS** | Document Management | `standard` | **P1** | Add the full OCR/search stack |
| 78 | **Taiga** | Project Management | `standard` | **P1** | Add after multi-service health support |
| 79 | **Tandoor Recipes** | Productivity / Recipes | `standard` | **P1** | Add now |
| 80 | **Zigbee2MQTT** | Home Automation / IoT | `advanced` | **P2** | Add only with device mapping support |
| 81 | **Z-Wave JS UI** | Home Automation / IoT | `advanced` | **P2** | Add after USB-device templates |
| 82 | **Frigate** | Video / Home Automation / AI | `dedicated-host` | **P2** | Add as hardware-specific variants |
| 83 | **Homebridge** | Home Automation | `advanced` | **P2** | Add with LAN-networking warning |
| 84 | **vLLM** | AI / Model Serving | `dedicated-host` | **P1** | Add for GPU nodes |
| 85 | **Onyx** | AI / Enterprise Search | `infrastructure or dedicated-host` | **P2** | Add as a reviewed multi-container stack |

---

# Detailed Expansion Catalog

## 43. Grafana Alloy

  - **Category:** Observability / Telemetry Collector
  - **Priority:** P0
  - **Deployment class:** `standard`
  - **Verdict:** **Add now**
  - **Description:** An OpenTelemetry-compatible collector and telemetry pipeline for metrics, logs, profiles, and traces.
  - **Why add it:** It can become the platform-native collection agent feeding Prometheus-compatible, Loki, Tempo, and OTLP backends.
  - **Ports:** `12345/tcp` for the optional UI and health endpoint; OTLP ports depend on configuration.
  - **Persistent storage:** Configuration is mandatory; local buffering or file positions may require persistent storage depending on enabled components.
  - **Dependencies:** None for a basic collector; requires one or more telemetry destinations.
  - **Production and template notes:**
    - Provide a generated starter configuration rather than an empty container.
- Do not expose telemetry receivers publicly by default.
- Offer agent, gateway, and host-monitoring variants separately.
- Host-monitoring variants may need additional read-only mounts and should be marked advanced.
  - **Official documentation/project:** https://grafana.com/docs/alloy/latest/set-up/install/docker/
## 44. Grafana Tempo

  - **Category:** Observability / Distributed Tracing
  - **Priority:** P1
  - **Deployment class:** `standard for monolithic; infrastructure for distributed`
  - **Verdict:** **Add monolithic first**
  - **Description:** A distributed tracing backend designed for high-volume trace storage and Grafana integration.
  - **Why add it:** Completes a metrics, logs, and traces observability stack alongside Prometheus-compatible storage and Loki.
  - **Ports:** Common receivers include OTLP `4317/4318`; query and HTTP ports depend on the selected mode.
  - **Persistent storage:** Persistent local storage for single-binary mode, or S3-compatible object storage for production architectures.
  - **Dependencies:** Optional object storage; distributed Tempo 3 deployments require a Kafka-compatible queue.
  - **Production and template notes:**
    - Publish a small single-binary template before a distributed template.
- Tempo does not provide built-in authentication; place it behind platform authentication or a private network.
- Do not market the local Docker Compose example as a high-availability production architecture.
- Distributed mode needs more lifecycle and dependency management than a normal one-click application.
  - **Official documentation/project:** https://grafana.com/docs/tempo/latest/set-up-for-tracing/setup-tempo/deploy/locally/docker-compose/
## 45. Grafana Mimir

  - **Category:** Observability / Metrics Storage
  - **Priority:** P1
  - **Deployment class:** `standard for monolithic; infrastructure for distributed`
  - **Verdict:** **Add monolithic first**
  - **Description:** Horizontally scalable, long-term storage for Prometheus metrics.
  - **Why add it:** Provides a multi-tenant long-term metrics backend beyond a single Prometheus instance.
  - **Ports:** HTTP and gRPC ports depend on deployment mode and configuration.
  - **Persistent storage:** Local disk for basic testing; object storage is preferred for durable production deployments.
  - **Dependencies:** S3-compatible object storage and additional services for distributed architectures.
  - **Production and template notes:**
    - Use the monolithic mode for the first verified template.
- Keep ingestion endpoints internal unless authenticated.
- Add distributed mode only after the platform supports service groups, object storage, and rolling upgrades.
- Document retention and compaction requirements.
  - **Official documentation/project:** https://grafana.com/docs/mimir/latest/get-started/
## 46. Prometheus Alertmanager

  - **Category:** Monitoring / Alerting
  - **Priority:** P0
  - **Deployment class:** `standard`
  - **Verdict:** **Add now**
  - **Description:** Receives alerts from Prometheus-compatible systems, groups them, suppresses duplicates, and routes notifications.
  - **Why add it:** Prometheus without Alertmanager leaves alert delivery and silencing incomplete.
  - **Ports:** `9093/tcp` for the web UI and API; cluster communication uses an additional peer port when clustering is enabled.
  - **Persistent storage:** Configuration and optional persistent notification or silence data.
  - **Dependencies:** Prometheus or another compatible alert sender; notification provider credentials.
  - **Production and template notes:**
    - Generate a minimal valid configuration.
- Store email, Slack, webhook, and other notification credentials as secrets.
- Start with single-node mode; add clustered Alertmanager as a separate variant.
- Expose the UI only when authentication is enabled.
  - **Official documentation/project:** https://prometheus.io/docs/alerting/latest/alertmanager/
## 47. Gatus

  - **Category:** Monitoring / Status
  - **Priority:** P0
  - **Deployment class:** `standard`
  - **Verdict:** **Add now**
  - **Description:** Automated service-health dashboard supporting HTTP, TCP, ICMP, DNS, and other endpoint checks.
  - **Why add it:** A lightweight alternative to heavier uptime platforms and a strong fit for a deployment dashboard.
  - **Ports:** `8080/tcp` by default.
  - **Persistent storage:** Configuration file; SQLite or an external database can persist results.
  - **Dependencies:** None for a basic deployment.
  - **Production and template notes:**
    - Generate a starter endpoint configuration.
- Allow private-network targets without exposing the Gatus UI publicly.
- Provide SQLite and PostgreSQL variants.
- Treat ICMP checks as an advanced capability if extra Linux permissions are required.
  - **Official documentation/project:** https://github.com/TwiN/gatus
## 48. Zabbix

  - **Category:** Monitoring / Infrastructure
  - **Priority:** P2
  - **Deployment class:** `infrastructure`
  - **Verdict:** **Add after core monitoring**
  - **Description:** Full infrastructure monitoring suite with server, web frontend, agents, proxies, discovery, and alerting.
  - **Why add it:** Covers enterprise-style infrastructure monitoring beyond lightweight uptime checks.
  - **Ports:** Server commonly uses `10051/tcp`; agent and SNMP-related ports depend on the selected topology.
  - **Persistent storage:** Database, server configuration, alert scripts, modules, and optional agent/proxy data.
  - **Dependencies:** PostgreSQL or MySQL/MariaDB; multiple containers for a complete stack.
  - **Production and template notes:**
    - Publish a PostgreSQL-based verified variant first.
- Separate Zabbix server, frontend, database, and agent templates.
- Do not expose agent or server ports globally without access controls.
- Resource recommendations should increase with monitored-host count and history retention.
  - **Official documentation/project:** https://www.zabbix.com/container_images
## 49. Checkmk Raw Edition

  - **Category:** Monitoring / Infrastructure
  - **Priority:** P2
  - **Deployment class:** `infrastructure`
  - **Verdict:** **Good advanced candidate**
  - **Description:** Infrastructure and application monitoring platform with host discovery, agents, checks, dashboards, and notifications.
  - **Why add it:** Offers a broad monitoring model with a polished operations interface.
  - **Ports:** Web and agent-related ports depend on the site configuration.
  - **Persistent storage:** Persist the Checkmk site directory, commonly under `/omd/sites`.
  - **Dependencies:** No external database is required for a basic Raw Edition deployment.
  - **Production and template notes:**
    - Use the Raw Edition image for an open-source catalog entry.
- Persist the complete site directory.
- Agent registration and monitored-host connectivity must be documented.
- Upgrades should be tested across Checkmk site-version changes.
  - **Official documentation/project:** https://docs.checkmk.com/latest/en/introduction_docker.html
## 50. ClickHouse

  - **Category:** Database / Analytics
  - **Priority:** P0
  - **Deployment class:** `standard`
  - **Verdict:** **Add now**
  - **Description:** Column-oriented analytical database for high-throughput ingestion and low-latency OLAP queries.
  - **Why add it:** Unlocks analytics, event, log, and product-telemetry workloads not well served by traditional relational databases.
  - **Ports:** `8123/tcp` for HTTP and `9000/tcp` for the native protocol.
  - **Persistent storage:** Persist `/var/lib/clickhouse`; configuration and user definitions may also be mounted.
  - **Dependencies:** None for single-node mode; coordination services may be needed for replicated clusters.
  - **Production and template notes:**
    - Set an appropriate `nofile` ulimit.
- Create an initial non-default user and password.
- Do not expose the native or HTTP database ports publicly by default.
- Start with single-node; add replicated clusters only after supporting topology-aware storage.
  - **Official documentation/project:** https://hub.docker.com/_/clickhouse
## 51. TimescaleDB

  - **Category:** Database / Time Series
  - **Priority:** P0
  - **Deployment class:** `standard`
  - **Verdict:** **Add now with production warning**
  - **Description:** PostgreSQL extension and distribution optimized for time-series, event, and analytical data.
  - **Why add it:** Provides familiar PostgreSQL semantics with time-series features and is useful for IoT, monitoring, and event data.
  - **Ports:** `5432/tcp` internally.
  - **Persistent storage:** Persistent PostgreSQL data directory and backup storage.
  - **Dependencies:** None for a single-node container.
  - **Production and template notes:**
    - Keep the database private by default.
- Pin a compatible PostgreSQL and TimescaleDB version.
- Official basic Docker examples are suitable for development and evaluation; add backups, PITR, and HA guidance for production.
- Test extension upgrades before automatic major-version updates.
  - **Official documentation/project:** https://docs.timescale.com/self-hosted/latest/install/installation-docker/
## 52. InfluxDB

  - **Category:** Database / Time Series
  - **Priority:** P1
  - **Deployment class:** `standard`
  - **Verdict:** **Add only with explicit version selection**
  - **Description:** Time-series database and telemetry platform for metrics, events, IoT, and operational data.
  - **Why add it:** A common self-hosted time-series choice with its own query and ingestion ecosystem.
  - **Ports:** `8086/tcp` for the HTTP API and UI in InfluxDB 2.x.
  - **Persistent storage:** Persist data and configuration directories; back up organization, bucket, and token configuration.
  - **Dependencies:** None for a basic single-node deployment.
  - **Production and template notes:**
    - Never deploy the moving `latest` image tag.
- Upstream documented that on September 15, 2026, `latest` is scheduled to point to InfluxDB 3 Core.
- Treat InfluxDB 2 and InfluxDB 3 as separate catalog variants.
- Generate the initial admin token and store it as a secret.
  - **Official documentation/project:** https://docs.influxdata.com/influxdb/v2/install/use-docker-compose/
## 53. QuestDB

  - **Category:** Database / Time Series
  - **Priority:** P1
  - **Deployment class:** `standard`
  - **Verdict:** **Add after TimescaleDB**
  - **Description:** High-performance time-series database with SQL, PostgreSQL wire protocol, and high-throughput ingestion.
  - **Why add it:** Useful for market data, telemetry, and high-ingestion time-series workloads.
  - **Ports:** Common ports include `9000`, `9009`, `8812`, and `9003`; expose only those required.
  - **Persistent storage:** Persist `/var/lib/questdb`.
  - **Dependencies:** None for single-node deployment.
  - **Production and template notes:**
    - Publish only the HTTP/UI port by default.
- Keep PostgreSQL wire and ingestion ports internal unless explicitly enabled.
- Pin a stable version.
- Include memory and storage recommendations for large ingestion workloads.
  - **Official documentation/project:** https://questdb.com/docs/get-started/docker/
## 54. OpenSearch and OpenSearch Dashboards

  - **Category:** Search / Analytics
  - **Priority:** P1
  - **Deployment class:** `infrastructure`
  - **Verdict:** **Add as a multi-container stack**
  - **Description:** Distributed search and analytics engine with a browser-based dashboard.
  - **Why add it:** Supports application search, log analytics, security analytics, and large-scale indexing.
  - **Ports:** OpenSearch commonly uses `9200/9600`; Dashboards commonly uses `5601`.
  - **Persistent storage:** Persistent index data, snapshots, and configuration.
  - **Dependencies:** Dashboards depends on OpenSearch; production clusters require multiple nodes and coordinated storage.
  - **Production and template notes:**
    - Validate and apply the required `vm.max_map_count` host setting.
- Generate a strong initial admin password.
- Provide single-node and cluster variants separately.
- Document JVM heap sizing and minimum Docker memory.
  - **Official documentation/project:** https://docs.opensearch.org/latest/install-and-configure/install-opensearch/docker/
## 55. Apache Solr

  - **Category:** Search
  - **Priority:** P1
  - **Deployment class:** `standard or infrastructure`
  - **Verdict:** **Add standalone first**
  - **Description:** Mature full-text search platform built on Apache Lucene.
  - **Why add it:** Useful for enterprise search, faceted search, document indexing, and applications already built around Solr.
  - **Ports:** `8983/tcp` for the HTTP API and administration UI.
  - **Persistent storage:** Persist `/var/solr` and back up collections or snapshots.
  - **Dependencies:** None for standalone; ZooKeeper is commonly used for SolrCloud.
  - **Production and template notes:**
    - Offer standalone and SolrCloud as different variants.
- Do not publicly expose the admin UI without authentication.
- Use a pre-create collection or core lifecycle step when requested.
- SolrCloud requires more than a single app container.
  - **Official documentation/project:** https://solr.apache.org/guide/solr/latest/deployment-guide/solr-in-docker.html
## 56. NATS with JetStream

  - **Category:** Messaging
  - **Priority:** P0
  - **Deployment class:** `standard`
  - **Verdict:** **Add now**
  - **Description:** High-performance messaging system supporting pub/sub, request/reply, queues, and persistent JetStream streams.
  - **Why add it:** Adds a lightweight messaging primitive for event-driven applications and platform internals.
  - **Ports:** `4222/tcp` clients, `8222/tcp` monitoring, and `6222/tcp` clustering.
  - **Persistent storage:** JetStream storage must be persistent when enabled.
  - **Dependencies:** None for single-node; multiple nodes for a resilient cluster.
  - **Production and template notes:**
    - Do not expose the monitoring endpoint publicly.
- Generate credentials or operator/account configuration.
- Provide Core NATS and NATS with JetStream variants.
- Add a separate clustered template only after verifying quorum behavior.
  - **Official documentation/project:** https://docs.nats.io/running-a-nats-service/nats_docker
## 57. Redpanda

  - **Category:** Messaging / Streaming
  - **Priority:** P2
  - **Deployment class:** `infrastructure`
  - **Verdict:** **Add a development single-node template first**
  - **Description:** Kafka-compatible event-streaming platform designed without a JVM.
  - **Why add it:** Provides Kafka API compatibility for streaming workloads with a simpler deployment model.
  - **Ports:** Kafka, admin, proxy, schema registry, and console ports depend on enabled components.
  - **Persistent storage:** Persistent broker data and optional console configuration.
  - **Dependencies:** Redpanda Console is optional; production clusters require multiple brokers.
  - **Production and template notes:**
    - The official quickstart is not a substitute for a production cluster design.
- Require a meaningful memory allocation; upstream quickstarts commonly expect several gigabytes free.
- Expose Kafka listeners carefully because advertised addresses must match client access paths.
- Publish single-node and multi-node variants separately.
  - **Official documentation/project:** https://docs.redpanda.com/current/get-started/quick-start/
## 58. Temporal

  - **Category:** Workflow / Durable Execution
  - **Priority:** P2
  - **Deployment class:** `infrastructure`
  - **Verdict:** **Add only after lifecycle support**
  - **Description:** Durable execution platform for long-running workflows, retries, timers, state, and distributed application orchestration.
  - **Why add it:** Useful for developers building reliable background jobs and business workflows.
  - **Ports:** Frontend gRPC commonly uses `7233`; UI and metrics ports depend on the selected stack.
  - **Persistent storage:** Persistent database and optional visibility/search backend.
  - **Dependencies:** PostgreSQL, MySQL, or Cassandra; optional Elasticsearch or OpenSearch for advanced visibility.
  - **Production and template notes:**
    - Use the current `temporalio/samples-server` Compose examples as references.
- The older `temporalio/docker-compose` repository was archived in January 2026.
- Do not use the `auto-setup` image as a general production recommendation.
- Schema initialization and upgrades require explicit lifecycle jobs.
  - **Official documentation/project:** https://github.com/temporalio/samples-server
## 59. Kestra

  - **Category:** Workflow / Automation
  - **Priority:** P1
  - **Deployment class:** `standard`
  - **Verdict:** **Add PostgreSQL variant**
  - **Description:** Event-driven orchestration platform for data pipelines, scheduled workflows, scripts, and infrastructure tasks.
  - **Why add it:** Adds code-oriented workflow orchestration between n8n-style automation and full data platforms.
  - **Ports:** Web UI and API commonly use `8080/tcp`.
  - **Persistent storage:** Database, internal storage, logs, and workflow files.
  - **Dependencies:** PostgreSQL is recommended for persistent deployments.
  - **Production and template notes:**
    - Use PostgreSQL instead of an ephemeral local database.
- Persist internal storage and execution logs.
- Worker isolation and task-runner permissions need review.
- Treat Docker-socket task execution as an advanced optional variant.
  - **Official documentation/project:** https://kestra.io/docs/installation/docker-compose
## 60. Apache APISIX

  - **Category:** API Gateway
  - **Priority:** P1
  - **Deployment class:** `infrastructure`
  - **Verdict:** **Add after TCP routing and secret controls**
  - **Description:** Cloud-native API gateway supporting routing, authentication, plugins, traffic control, observability, and service discovery.
  - **Why add it:** Adds API-management and gateway capabilities beyond a basic application reverse proxy.
  - **Ports:** Proxy, Admin API, metrics, and optional control-plane ports vary by deployment.
  - **Persistent storage:** Configuration; etcd data for traditional deployment modes.
  - **Dependencies:** Commonly etcd; dashboard or additional control components may be optional.
  - **Production and template notes:**
    - Change the default Admin API key during provisioning.
- Keep the Admin API on a private network.
- Avoid conflict with the platform's primary reverse proxy.
- Provide a dedicated gateway-node or explicitly integrated deployment mode.
  - **Official documentation/project:** https://github.com/apache/apisix-docker
## 61. Hasura GraphQL Engine

  - **Category:** Backend / API
  - **Priority:** P1
  - **Deployment class:** `standard`
  - **Verdict:** **Add the open-source engine as a pinned variant**
  - **Description:** Instant GraphQL and API layer over supported databases, with metadata, permissions, events, and remote schemas.
  - **Why add it:** A high-value backend primitive for rapidly exposing database-backed APIs.
  - **Ports:** `8080/tcp` by default.
  - **Persistent storage:** Application metadata is stored in the configured database; export metadata for backup and migration.
  - **Dependencies:** PostgreSQL is commonly used as the metadata and application database.
  - **Production and template notes:**
    - Generate the admin secret.
- Disable the development console in hardened production mode when appropriate.
- Pin a specific open-source GraphQL Engine version and distinguish it from newer hosted product lines.
- Keep database credentials in the platform secret store.
  - **Official documentation/project:** https://hasura.io/docs/2.0/deployment/deployment-guides/docker/
## 62. Milvus

  - **Category:** Database / Vector Search
  - **Priority:** P2
  - **Deployment class:** `infrastructure`
  - **Verdict:** **Add stable standalone mode first**
  - **Description:** Distributed vector database for similarity search and AI retrieval workloads.
  - **Why add it:** Adds a scalable vector-search option for users who need more than lightweight embedded or single-service vector stores.
  - **Ports:** `19530/tcp` for the service and a web or management port depending on the selected release.
  - **Persistent storage:** Vector data, metadata, object storage, and coordination data.
  - **Dependencies:** Standalone Compose commonly includes etcd and MinIO; distributed mode has more components.
  - **Production and template notes:**
    - Pin a stable release; do not automatically follow beta documentation or tags.
- Provide standalone before distributed mode.
- Persist all dependency volumes.
- Include substantial memory and disk recommendations.
  - **Official documentation/project:** https://milvus.io/docs/install_standalone-docker-compose.md
## 63. MLflow

  - **Category:** AI / MLOps
  - **Priority:** P1
  - **Deployment class:** `standard`
  - **Verdict:** **Add a secure tracking-server stack**
  - **Description:** Experiment tracking, model registry, artifact management, and model lifecycle platform.
  - **Why add it:** Adds a core MLOps service for teams training and evaluating models.
  - **Ports:** `5000/tcp` is commonly used by the tracking server.
  - **Persistent storage:** Backend database and artifact storage; local artifacts should be persistent.
  - **Dependencies:** PostgreSQL or another supported backend store; S3-compatible object storage is recommended for artifacts.
  - **Production and template notes:**
    - Do not expose an unauthenticated tracking server publicly.
- Configure allowed hosts, proxy behavior, and authentication.
- Provide PostgreSQL plus S3-compatible storage as the production template.
- Separate model-serving templates from the tracking server.
  - **Official documentation/project:** https://mlflow.org/docs/latest/self-hosting/
## 64. ZITADEL

  - **Category:** Authentication / Identity
  - **Priority:** P0
  - **Deployment class:** `infrastructure`
  - **Verdict:** **Strong identity candidate**
  - **Description:** Identity and access management platform supporting OIDC, OAuth 2.0, SAML, MFA, organizations, projects, and service accounts.
  - **Why add it:** Provides a modern identity platform and alternative to Keycloak or Authentik.
  - **Ports:** HTTP, HTTPS, gRPC, and login endpoints depend on the proxy topology.
  - **Persistent storage:** PostgreSQL and persistent configuration.
  - **Dependencies:** PostgreSQL; official Compose examples may include Traefik and a separate login service.
  - **Production and template notes:**
    - The master key must be generated correctly and treated as immutable after initialization.
- Split initialization, setup, and normal startup into lifecycle stages.
- Avoid deploying the bundled proxy when it conflicts with the platform proxy.
- Verify HTTP/2 and gRPC forwarding through the platform ingress.
  - **Official documentation/project:** https://zitadel.com/docs/self-hosting/deploy/compose
## 65. Kanidm

  - **Category:** Authentication / Directory
  - **Priority:** P1
  - **Deployment class:** `infrastructure`
  - **Verdict:** **Add after LLDAP**
  - **Description:** Modern identity-management server providing directory, authentication, OAuth/OIDC, and related identity services.
  - **Why add it:** A security-focused directory option that can serve both human and application identity use cases.
  - **Ports:** Container service commonly listens on `8443/tcp`, mapped behind HTTPS.
  - **Persistent storage:** Persist `/data` and protect server configuration and certificates.
  - **Dependencies:** None for a basic server deployment.
  - **Production and template notes:**
    - Use a stable hostname from the first deployment.
- Back up identity data before upgrades.
- Treat certificate and origin configuration as immutable deployment inputs where possible.
- Keep administrative access private or protected by strong authentication.
  - **Official documentation/project:** https://kanidm.github.io/kanidm/stable/server_configuration.html
## 66. Casdoor

  - **Category:** Authentication / Identity
  - **Priority:** P1
  - **Deployment class:** `standard or infrastructure`
  - **Verdict:** **Good additional identity option**
  - **Description:** Identity and access-management platform supporting OAuth, OIDC, SAML, social login, MFA, and directory integrations.
  - **Why add it:** Provides an application-oriented identity service with broad protocol and social-login support.
  - **Ports:** Web service port depends on the selected image configuration.
  - **Persistent storage:** Database and application configuration.
  - **Dependencies:** A supported relational database.
  - **Production and template notes:**
    - Generate the database password, application secret, and initial admin credentials.
- Set the origin and callback URLs correctly.
- Use a production database rather than a development default.
- Do not publish default credentials.
  - **Official documentation/project:** https://casdoor.org/docs/deployment/docker/
## 67. NetBird Self-Hosted

  - **Category:** VPN / Networking
  - **Priority:** P2
  - **Deployment class:** `infrastructure`
  - **Verdict:** **Add only as a coordinated stack**
  - **Description:** WireGuard-based private networking platform with management, signaling, relay, policy, peer, and dashboard components.
  - **Why add it:** Provides a broader zero-trust networking experience than a simple VPN server.
  - **Ports:** HTTPS, management, signal, relay, STUN/TURN, and WireGuard-related ports depend on the selected architecture.
  - **Persistent storage:** Management data, identity configuration, certificates, and relay configuration.
  - **Dependencies:** Multiple services; identity-provider and reverse-proxy choices affect the Compose topology.
  - **Production and template notes:**
    - Do not model it as a single ordinary web container.
- Support UDP and raw TCP routing before publishing the template.
- Add DNS, domain, and public-IP preflight checks.
- Treat upgrades as a stack-level operation.
  - **Official documentation/project:** https://docs.netbird.io/selfhosted/selfhosted-guide
## 68. OWASP Dependency-Track

  - **Category:** Security / Software Supply Chain
  - **Priority:** P0
  - **Deployment class:** `standard`
  - **Verdict:** **Add now**
  - **Description:** Software composition analysis platform for ingesting SBOMs, tracking components, vulnerabilities, policies, and risk.
  - **Why add it:** Adds supply-chain security and SBOM analysis to the developer platform.
  - **Ports:** API and frontend use separate services and ports in the official Compose stack.
  - **Persistent storage:** PostgreSQL database and persistent application data.
  - **Dependencies:** PostgreSQL; API server and frontend containers.
  - **Production and template notes:**
    - Keep the API service private behind the frontend or authenticated ingress.
- Provide a generated initial admin workflow.
- Enable scheduled vulnerability-data updates.
- Resource requirements can grow with portfolio and SBOM size.
  - **Official documentation/project:** https://docs.dependencytrack.org/getting-started/deploy-docker/
## 69. OWASP DefectDojo

  - **Category:** Security / Vulnerability Management
  - **Priority:** P1
  - **Deployment class:** `standard or infrastructure`
  - **Verdict:** **Add a reviewed production Compose variant**
  - **Description:** Vulnerability-management platform for importing scanner results, deduplicating findings, tracking remediation, and reporting risk.
  - **Why add it:** Complements scanners by providing centralized finding management and security workflow.
  - **Ports:** Web and API ports depend on the reverse-proxy layout.
  - **Persistent storage:** Database, media, reports, and background-task data.
  - **Dependencies:** Database, Redis, workers, scheduler, and web components.
  - **Production and template notes:**
    - The upstream Compose configuration needs production-specific customization.
- Generate all application and database secrets.
- Persist uploaded reports and database data.
- Include worker and scheduler healthchecks, not only the web UI.
  - **Official documentation/project:** https://docs.defectdojo.com/en/open_source/installation/
## 70. Wazuh

  - **Category:** Security / SIEM / XDR
  - **Priority:** P2
  - **Deployment class:** `dedicated-host`
  - **Verdict:** **Advanced verified stack only**
  - **Description:** Security monitoring and response platform with agents, manager, indexer, dashboards, detection, compliance, and file-integrity monitoring.
  - **Why add it:** Adds endpoint security, log analysis, SIEM, and compliance capabilities.
  - **Ports:** Agent enrollment, event ingestion, API, dashboard, and indexer ports must be routed selectively.
  - **Persistent storage:** Indexer data, manager data, certificates, configuration, and dashboards.
  - **Dependencies:** Multi-container stack with manager, indexer, and dashboard.
  - **Production and template notes:**
    - Publish a high memory and disk recommendation.
- Validate required host sysctls before deployment.
- Use generated certificates and secrets.
- Agent enrollment and upgrades require platform-level documentation.
  - **Official documentation/project:** https://documentation.wazuh.com/current/deployment-options/docker/index.html
## 71. Teleport Community Edition

  - **Category:** Security / Access
  - **Priority:** P2
  - **Deployment class:** `infrastructure`
  - **Verdict:** **Add only after multi-port ingress support**
  - **Description:** Identity-aware access plane for SSH, Kubernetes, databases, applications, and infrastructure resources.
  - **Why add it:** Adds audited, certificate-based administrative access and removes dependence on shared SSH keys.
  - **Ports:** Proxy, auth, SSH, Kubernetes, database, and web ports depend on enabled roles.
  - **Persistent storage:** Persist `/var/lib/teleport`; configuration is normally mounted under `/etc/teleport`.
  - **Dependencies:** Can run roles together for a small deployment or separately for production.
  - **Production and template notes:**
    - Start with an all-in-one small-lab template clearly labeled non-HA.
- Raw TCP and multiple public ports must be supported.
- Cluster name and public address must be stable.
- Back up cluster state and protect join tokens.
  - **Official documentation/project:** https://goteleport.com/docs/reference/deployment/docker/
## 72. Kopia

  - **Category:** Backup
  - **Priority:** P0
  - **Deployment class:** `advanced`
  - **Verdict:** **Add now with restricted mounts**
  - **Description:** Encrypted, deduplicated backup tool with snapshots, policies, multiple repository backends, and an optional web server.
  - **Why add it:** A strong general backup engine for application volumes and user-selected host paths.
  - **Ports:** `51515/tcp` is commonly used for Kopia server mode.
  - **Persistent storage:** Configuration, cache, logs, temporary data, source mounts, and local or remote repository access.
  - **Dependencies:** None; supports many remote and S3-compatible repository backends.
  - **Production and template notes:**
    - Do not mount the full host filesystem by default.
- Do not copy insecure public-listen examples into a production template.
- Store repository passwords as secrets.
- Offer repository setup and restore as explicit lifecycle actions.
  - **Official documentation/project:** https://kopia.io/docs/installation/
## 73. BorgWarehouse

  - **Category:** Backup
  - **Priority:** P1
  - **Deployment class:** `advanced`
  - **Verdict:** **Good Borg-focused candidate**
  - **Description:** Web interface and management service for hosting and administering BorgBackup repositories.
  - **Why add it:** Adds centralized, SSH-based Borg repository management for servers and workstations.
  - **Ports:** Web UI plus SSH access for backup clients.
  - **Persistent storage:** Repository data, application database, SSH keys, and configuration.
  - **Dependencies:** None for the basic Compose stack, but reliable storage is essential.
  - **Production and template notes:**
    - Repository storage permissions must be carefully controlled.
- Use a dedicated storage path rather than unrestricted host access.
- Expose SSH only on an explicitly selected host port.
- Document offsite replication because a repository on the same host is not a complete disaster-recovery plan.
  - **Official documentation/project:** https://borgwarehouse.com/docs/admin-manual/docker-installation/
## 74. ArchiveBox

  - **Category:** Archiving / Knowledge
  - **Priority:** P0
  - **Deployment class:** `standard`
  - **Verdict:** **Add now**
  - **Description:** Self-hosted web archiving suite that saves pages in multiple formats for long-term preservation and search.
  - **Why add it:** Adds a distinctive personal and organizational web-archiving use case.
  - **Ports:** Web UI commonly uses `8000/tcp`.
  - **Persistent storage:** Archive data, indexes, configuration, and downloaded browser assets.
  - **Dependencies:** The official image includes many capture tools; optional workers or search components depend on the chosen stack.
  - **Production and template notes:**
    - Archive storage can grow quickly; include quota and disk-growth guidance.
- Persist the full data directory.
- Restrict outbound network behavior when required by organizational policy.
- Use scheduled jobs for queued or recurring archival tasks.
  - **Official documentation/project:** https://github.com/ArchiveBox/ArchiveBox/wiki/Docker
## 75. Linkwarden

  - **Category:** Bookmarks / Archiving
  - **Priority:** P0
  - **Deployment class:** `standard`
  - **Verdict:** **Add now**
  - **Description:** Collaborative bookmark manager that preserves webpages, organizes collections, and supports search.
  - **Why add it:** Adds team-oriented link management and web preservation beyond minimal bookmark applications.
  - **Ports:** Web application commonly uses `3000/tcp`.
  - **Persistent storage:** PostgreSQL data, preserved pages, screenshots, and optional object-storage data.
  - **Dependencies:** PostgreSQL; optional Meilisearch and S3-compatible storage.
  - **Production and template notes:**
    - Provide a simple PostgreSQL variant and an advanced search/object-storage variant.
- Generate authentication and encryption secrets.
- Persist preserved content separately from the database.
- Document storage growth for screenshots and archived pages.
  - **Official documentation/project:** https://docs.linkwarden.app/self-hosting/installation
## 76. Mayan EDMS

  - **Category:** Document Management
  - **Priority:** P1
  - **Deployment class:** `standard`
  - **Verdict:** **Add as a complete Compose stack**
  - **Description:** Electronic document-management system with workflows, metadata, search, versioning, roles, and document processing.
  - **Why add it:** A feature-rich enterprise document-management alternative.
  - **Ports:** Web service port depends on the included proxy and Compose configuration.
  - **Persistent storage:** Database, document files, media, search indexes, and task data.
  - **Dependencies:** Multi-container stack including database, cache, workers, and search-related components.
  - **Production and template notes:**
    - Use the upstream Compose architecture rather than a partial single-container deployment.
- Back up both database and document storage.
- Declare higher memory requirements than a basic document viewer.
- Verify worker readiness before declaring the stack healthy.
  - **Official documentation/project:** https://docs.mayan-edms.com/chapters/deployment/docker_compose.html
## 77. Papermerge DMS

  - **Category:** Document Management
  - **Priority:** P1
  - **Deployment class:** `standard`
  - **Verdict:** **Add the full OCR/search stack**
  - **Description:** Document-management system focused on scanned documents, OCR, organization, and full-text retrieval.
  - **Why add it:** Provides another document-ingestion workflow with different UI and processing behavior from Paperless and Docspell.
  - **Ports:** Web port depends on deployment topology.
  - **Persistent storage:** Documents, database, OCR data, search indexes, and worker state.
  - **Dependencies:** Full deployments may include PostgreSQL, Redis, workers, and a search backend.
  - **Production and template notes:**
    - Do not label a minimal single-container setup as the full production feature set.
- Check supported architectures for every image in the selected release.
- Persist original documents independently from generated indexes.
- Allow OCR language configuration.
  - **Official documentation/project:** https://docs.papermerge.io/latest/setup/docker/
## 78. Taiga

  - **Category:** Project Management
  - **Priority:** P1
  - **Deployment class:** `standard`
  - **Verdict:** **Add after multi-service health support**
  - **Description:** Agile project-management platform supporting Scrum, Kanban, issues, epics, wiki, and team collaboration.
  - **Why add it:** Adds a mature open-source agile project-management option.
  - **Ports:** Web, API, and event/WebSocket endpoints depend on the Compose architecture.
  - **Persistent storage:** PostgreSQL, media, static assets, and message-broker data.
  - **Dependencies:** PostgreSQL, RabbitMQ, backend, frontend, asynchronous workers, events service, and proxy.
  - **Production and template notes:**
    - Correctly set the public URL in both frontend and backend configuration.
- Support WebSocket routing.
- Generate all application, database, and message-broker secrets.
- Do not report readiness until API, events, workers, and database are healthy.
  - **Official documentation/project:** https://docs.taiga.io/setup-production.html
## 79. Tandoor Recipes

  - **Category:** Productivity / Recipes
  - **Priority:** P1
  - **Deployment class:** `standard`
  - **Verdict:** **Add now**
  - **Description:** Recipe management, meal planning, shopping list, import, sharing, and household food organization application.
  - **Why add it:** A popular household application with a well-defined Docker deployment.
  - **Ports:** Web port depends on the proxy or gunicorn configuration.
  - **Persistent storage:** PostgreSQL, media uploads, static data, and application configuration.
  - **Dependencies:** PostgreSQL is recommended.
  - **Production and template notes:**
    - Use explicit image versions.
- Back up before every database-affecting update because database downgrades are not generally safe.
- Generate the application secret and database credentials.
- Persist media separately from the database.
  - **Official documentation/project:** https://docs.tandoor.dev/install/docker/
## 80. Zigbee2MQTT

  - **Category:** Home Automation / IoT
  - **Priority:** P2
  - **Deployment class:** `advanced`
  - **Verdict:** **Add only with device mapping support**
  - **Description:** Bridges Zigbee devices to MQTT using a supported coordinator, enabling integration without a vendor-specific hub.
  - **Why add it:** A high-value home-automation primitive that integrates with Home Assistant and MQTT.
  - **Ports:** Web frontend port is configurable; MQTT traffic goes to an external broker.
  - **Persistent storage:** Configuration, device database, coordinator backup, and state.
  - **Dependencies:** MQTT broker and a supported USB, serial, or network Zigbee coordinator.
  - **Production and template notes:**
    - The platform must support stable device mappings such as `/dev/serial/by-id/...`.
- Use host timezone configuration where required.
- Do not automatically request privileged mode when a specific device mapping is enough.
- Coordinator firmware and architecture compatibility must be documented.
  - **Official documentation/project:** https://www.zigbee2mqtt.io/guide/installation/02_docker.html
## 81. Z-Wave JS UI

  - **Category:** Home Automation / IoT
  - **Priority:** P2
  - **Deployment class:** `advanced`
  - **Verdict:** **Add after USB-device templates**
  - **Description:** Z-Wave control panel and gateway exposing Z-Wave devices through a web UI, MQTT, and WebSocket APIs.
  - **Why add it:** Completes a local home-automation stack for Z-Wave hardware.
  - **Ports:** Web UI and WebSocket server ports are configurable.
  - **Persistent storage:** Application store, Z-Wave network keys, device cache, logs, and configuration.
  - **Dependencies:** A supported Z-Wave USB or network controller.
  - **Production and template notes:**
    - Network security keys are sensitive secrets and must be backed up.
- Use stable device paths instead of transient `/dev/ttyUSB*` names.
- Avoid privileged mode when a narrow device mapping is sufficient.
- Loss of persistent store or security keys can break access to paired devices.
  - **Official documentation/project:** https://zwave-js.github.io/zwave-js-ui/#/getting-started/quick-start
## 82. Frigate

  - **Category:** Video / Home Automation / AI
  - **Priority:** P2
  - **Deployment class:** `dedicated-host`
  - **Verdict:** **Add as hardware-specific variants**
  - **Description:** Local network video recorder with real-time object detection and Home Assistant integration.
  - **Why add it:** A major self-hosted edge-AI application for camera monitoring.
  - **Ports:** Web, RTSP, WebRTC, and integration ports depend on enabled features.
  - **Persistent storage:** Configuration, database, recordings, snapshots, model cache, and temporary shared memory.
  - **Dependencies:** Camera streams; optional Coral, GPU, or other hardware accelerators; MQTT for some integrations.
  - **Production and template notes:**
    - Create CPU, Coral, NVIDIA, Intel, and other supported hardware variants separately.
- Declare `/dev/shm` and recording-storage requirements.
- Support device mappings and hardware-acceleration settings.
- Do not promise useful multi-camera performance from an unrestricted generic CPU template.
  - **Official documentation/project:** https://docs.frigate.video/frigate/installation/
## 83. Homebridge

  - **Category:** Home Automation
  - **Priority:** P2
  - **Deployment class:** `advanced`
  - **Verdict:** **Add with LAN-networking warning**
  - **Description:** Bridge that exposes unsupported smart-home devices and plugins to Apple HomeKit.
  - **Why add it:** Popular home-automation integration service with a large plugin ecosystem.
  - **Ports:** Web UI and HomeKit accessory ports vary; mDNS discovery is important.
  - **Persistent storage:** Configuration, accessories cache, persistence data, plugins, and logs.
  - **Dependencies:** LAN access and optional hardware or external services used by plugins.
  - **Production and template notes:**
    - Host networking is often the simplest way to make mDNS discovery work, but increases isolation risk.
- Plugin installation executes third-party code and should be clearly disclosed.
- Persist the Homebridge identity and accessory cache.
- Provide a restricted bridge-network variant only when multicast behavior is verified.
  - **Official documentation/project:** https://github.com/homebridge/homebridge/wiki/Install-Homebridge-on-Docker
## 84. vLLM

  - **Category:** AI / Model Serving
  - **Priority:** P1
  - **Deployment class:** `dedicated-host`
  - **Verdict:** **Add for GPU nodes**
  - **Description:** High-throughput large-language-model inference server with an OpenAI-compatible API.
  - **Why add it:** Provides production-oriented model serving for users with supported GPUs.
  - **Ports:** OpenAI-compatible HTTP API commonly uses `8000/tcp`.
  - **Persistent storage:** Model cache and optional compiled-kernel or runtime caches.
  - **Dependencies:** Supported GPU, driver, container runtime, sufficient VRAM, and access to model files or registries.
  - **Production and template notes:**
    - Treat GPU type, compute capability, VRAM, quantization, and model size as deployment inputs.
- Mount the model cache persistently.
- Store model-registry tokens as secrets.
- Do not expose the inference endpoint publicly without authentication and rate limits.
  - **Official documentation/project:** https://docs.vllm.ai/en/latest/deployment/docker.html
## 85. Onyx

  - **Category:** AI / Enterprise Search
  - **Priority:** P2
  - **Deployment class:** `infrastructure or dedicated-host`
  - **Verdict:** **Add as a reviewed multi-container stack**
  - **Description:** AI assistant and enterprise-search platform that connects to organizational data sources and provides retrieval-based chat and search.
  - **Why add it:** Adds connector-driven internal knowledge search beyond standalone chat interfaces.
  - **Ports:** Web, API, background, search, and model-related ports depend on the deployment edition.
  - **Persistent storage:** Relational database, vector or search data, connector state, credentials, and uploaded content.
  - **Dependencies:** Multi-container Compose stack with databases, search or vector components, background workers, and optional model services.
  - **Production and template notes:**
    - Connector credentials must be stored as encrypted secrets.
- Publish Lite and Standard variants only when their dependency differences are explicit.
- Include high memory and storage recommendations.
- Review outbound network access because connectors can reach sensitive organizational systems.
  - **Official documentation/project:** https://docs.onyx.app/deployment

---

# Recommended Rollout for the New 43 Services

## Expansion Batch A: Low-risk, high-value additions

These should be the next implementation targets:

1. Grafana Alloy
2. Prometheus Alertmanager
3. Gatus
4. ClickHouse
5. TimescaleDB
6. NATS with JetStream
7. ZITADEL
8. OWASP Dependency-Track
9. Kopia
10. ArchiveBox
11. Linkwarden
12. Tandoor Recipes

### Why this batch

- All fill clear catalog gaps.
- Their first useful topology is understandable.
- They have strong official container guidance.
- They do not require a full distributed control plane.
- Their persistent paths and health behavior can be tested predictably.

## Expansion Batch B: Standard multi-container applications

1. Grafana Tempo single-binary
2. Grafana Mimir monolithic
3. QuestDB
4. Apache Solr standalone
5. Kestra with PostgreSQL
6. Hasura GraphQL Engine
7. MLflow with PostgreSQL and S3-compatible storage
8. Kanidm
9. Casdoor
10. OWASP DefectDojo
11. Mayan EDMS
12. Papermerge DMS
13. Taiga

## Expansion Batch C: Infrastructure stacks

1. Zabbix
2. Checkmk Raw
3. OpenSearch and Dashboards
4. Redpanda
5. Temporal
6. Apache APISIX
7. Milvus
8. NetBird
9. Wazuh
10. Teleport Community Edition

## Expansion Batch D: Host and hardware-integrated services

1. BorgWarehouse
2. Zigbee2MQTT
3. Z-Wave JS UI
4. Frigate
5. Homebridge
6. vLLM
7. Onyx

---

# New Manifest Capabilities Required by This Expansion

The earlier manifest model should be extended with the following fields.

## Upstream Compose status

```yaml
upstream:
  composeRepository:
  composePath:
  composeStatus: active
  lastVerifiedRelease:
  lastVerifiedAt:
  documentationVersion:
```

Allowed `composeStatus` values:

```text
active
archived
development-only
reference-only
production-reference
not-provided
```

This prevents archived examples, such as an old Temporal Compose repository, from silently becoming permanent catalog infrastructure.

## Device mappings

```yaml
hardware:
  devices:
    - id: zigbee-coordinator
      required: true
      discovery:
        type: serial-by-id
      containerPath: /dev/ttyACM0
      permissions: rw
```

Required for:

- Zigbee2MQTT
- Z-Wave JS UI
- Frigate accelerators
- USB-connected home-automation services

## GPU constraints

```yaml
gpu:
  required: true
  vendors:
    - nvidia
  minimumVram:
  supportedComputeCapabilities:
  runtime:
  deviceCount:
  modelCompatibilityCheck:
```

Required for:

- vLLM
- Frigate accelerator variants
- LocalAI GPU variants
- Tabby
- Other model-serving services

## Raw protocol routes

```yaml
routes:
  - name: web
    protocol: https
    containerPort: 8080
  - name: grpc
    protocol: grpc
    containerPort: 7233
  - name: mqtt
    protocol: tcp
    containerPort: 1883
  - name: stun
    protocol: udp
    containerPort: 3478
```

The platform must not assume every service is HTTP.

## Host preflight checks

```yaml
preflight:
  ports:
    mustBeAvailable: []
  sysctls:
    required: []
  kernelModules:
    required: []
  devices:
    required: []
  filesystem:
    minimumFreeSpace:
  network:
    publicIpv4Required:
    publicIpv6Recommended:
    reverseDnsRequired:
```

Examples:

- OpenSearch: required `vm.max_map_count`
- Mail services: SMTP ports and reverse DNS
- Frigate: accelerator devices and shared memory
- Zigbee2MQTT: serial coordinator
- NetBird: UDP/TCP routes and public endpoint
- ClickHouse: file-descriptor limits

## Immutable initialization values

```yaml
initialization:
  immutableValues:
    - name: masterKey
      generated: true
      warning: Changing this value after initialization can make existing data unusable.
```

Useful for identity, PKI, encryption, and secret-management systems.

## Topology variants

```yaml
variants:
  - id: single
    topology: monolithic
    intendedUse: small-production
  - id: distributed
    topology: multi-service
    intendedUse: production
    requires:
      - object-storage
      - kafka-compatible-queue
```

Required for:

- Tempo
- Mimir
- OpenSearch
- Redpanda
- Temporal
- Milvus
- Zabbix
- Teleport

---

# Template Acceptance Gate

A service should not receive a `verified` badge until all applicable tests pass.

## General

- [ ] Official image or explicitly audited community image
- [ ] Explicit version or digest
- [ ] Fresh install
- [ ] Restart persistence
- [ ] Host reboot persistence
- [ ] HTTPS and WebSocket behavior
- [ ] Healthcheck reports readiness, not only process existence
- [ ] Backup and restore
- [ ] Upgrade from the previous supported version
- [ ] Rollback behavior documented
- [ ] Database ports remain private
- [ ] Generated secrets contain no known defaults

## Infrastructure

- [ ] Raw TCP and UDP routing tested
- [ ] Multi-domain routing tested
- [ ] Agent or client enrollment tested
- [ ] Certificate rotation tested
- [ ] Cluster or quorum failure behavior tested
- [ ] Host sysctls validated before deployment
- [ ] Public-port conflicts detected
- [ ] Admin APIs private by default

## Hardware

- [ ] Device discovery uses stable paths
- [ ] Device permission failure produces an understandable error
- [ ] Container restart does not lose device identity
- [ ] GPU or accelerator compatibility is checked
- [ ] CPU fallback is not advertised when operationally unusable
- [ ] Hardware-specific image architecture is verified

## Data systems

- [ ] Data directory is persisted
- [ ] Snapshot or native backup tested
- [ ] Restore into a fresh deployment tested
- [ ] Major-version migration is blocked without a documented path
- [ ] Retention and disk-growth controls are available
- [ ] Memory and file-descriptor requirements are declared

---

# Official Links for Expansion Services

| Service | Official documentation or project |
|---|---|
| Grafana Alloy | https://grafana.com/docs/alloy/latest/set-up/install/docker/ |
| Grafana Tempo | https://grafana.com/docs/tempo/latest/set-up-for-tracing/setup-tempo/deploy/locally/docker-compose/ |
| Grafana Mimir | https://grafana.com/docs/mimir/latest/get-started/ |
| Prometheus Alertmanager | https://prometheus.io/docs/alerting/latest/alertmanager/ |
| Gatus | https://github.com/TwiN/gatus |
| Zabbix | https://www.zabbix.com/container_images |
| Checkmk Raw Edition | https://docs.checkmk.com/latest/en/introduction_docker.html |
| ClickHouse | https://hub.docker.com/_/clickhouse |
| TimescaleDB | https://docs.timescale.com/self-hosted/latest/install/installation-docker/ |
| InfluxDB | https://docs.influxdata.com/influxdb/v2/install/use-docker-compose/ |
| QuestDB | https://questdb.com/docs/get-started/docker/ |
| OpenSearch and OpenSearch Dashboards | https://docs.opensearch.org/latest/install-and-configure/install-opensearch/docker/ |
| Apache Solr | https://solr.apache.org/guide/solr/latest/deployment-guide/solr-in-docker.html |
| NATS with JetStream | https://docs.nats.io/running-a-nats-service/nats_docker |
| Redpanda | https://docs.redpanda.com/current/get-started/quick-start/ |
| Temporal | https://github.com/temporalio/samples-server |
| Kestra | https://kestra.io/docs/installation/docker-compose |
| Apache APISIX | https://github.com/apache/apisix-docker |
| Hasura GraphQL Engine | https://hasura.io/docs/2.0/deployment/deployment-guides/docker/ |
| Milvus | https://milvus.io/docs/install_standalone-docker-compose.md |
| MLflow | https://mlflow.org/docs/latest/self-hosting/ |
| ZITADEL | https://zitadel.com/docs/self-hosting/deploy/compose |
| Kanidm | https://kanidm.github.io/kanidm/stable/server_configuration.html |
| Casdoor | https://casdoor.org/docs/deployment/docker/ |
| NetBird Self-Hosted | https://docs.netbird.io/selfhosted/selfhosted-guide |
| OWASP Dependency-Track | https://docs.dependencytrack.org/getting-started/deploy-docker/ |
| OWASP DefectDojo | https://docs.defectdojo.com/en/open_source/installation/ |
| Wazuh | https://documentation.wazuh.com/current/deployment-options/docker/index.html |
| Teleport Community Edition | https://goteleport.com/docs/reference/deployment/docker/ |
| Kopia | https://kopia.io/docs/installation/ |
| BorgWarehouse | https://borgwarehouse.com/docs/admin-manual/docker-installation/ |
| ArchiveBox | https://github.com/ArchiveBox/ArchiveBox/wiki/Docker |
| Linkwarden | https://docs.linkwarden.app/self-hosting/installation |
| Mayan EDMS | https://docs.mayan-edms.com/chapters/deployment/docker_compose.html |
| Papermerge DMS | https://docs.papermerge.io/latest/setup/docker/ |
| Taiga | https://docs.taiga.io/setup-production.html |
| Tandoor Recipes | https://docs.tandoor.dev/install/docker/ |
| Zigbee2MQTT | https://www.zigbee2mqtt.io/guide/installation/02_docker.html |
| Z-Wave JS UI | https://zwave-js.github.io/zwave-js-ui/#/getting-started/quick-start |
| Frigate | https://docs.frigate.video/frigate/installation/ |
| Homebridge | https://github.com/homebridge/homebridge/wiki/Install-Homebridge-on-Docker |
| vLLM | https://docs.vllm.ai/en/latest/deployment/docker.html |
| Onyx | https://docs.onyx.app/deployment |

---

# Expanded Final Recommendation

The catalog should now be developed as a **capability matrix**, not a long flat list of applications.

The 85 researched candidates should be grouped by:

1. Deployment class
2. Operational topology
3. Persistent data model
4. Public protocol requirements
5. Host-access risk
6. Hardware dependency
7. Backup maturity
8. Upgrade maturity
9. Architecture support
10. Template verification status

The strongest next milestone is not adding all 43 new services at once. It is shipping **Expansion Batch A** as verified templates while building the manifest features needed for Batch B, C, and D.

A template that starts a container but cannot safely back up, update, restore, authenticate, route, or validate its host requirements is not a production-ready one-click service.
