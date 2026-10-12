---
name: generate-compose
description: First-run TechLead task. Inspect the codebase and write the shared dev docker-compose file plus the env file Developers use to connect.
---

# Generate shared dev compose

Goal: one docker-compose stack shared by every Developer agent of this project.

1. Detect what the service needs with codegraph and config files: database engine + version (ORM config,
   migrations, existing `docker-compose*.yml`, `.env.example`), caches/queues (Redis, RabbitMQ, Kafka), object storage.
2. Write `$DOCKET_DATA/docker-compose.yml`:
   - Pin image versions that match the codebase. Use named volumes. Add healthchecks.
   - Publish on uncommon host ports (e.g. Postgres `55432:5432`) so it never clashes with the user's own services.
   - Only dependencies, never the application itself.
3. Write `$DOCKET_DATA/dev-db.env` with the exact env vars the app reads (e.g. `DATABASE_URL=...`) pointing at
   `localhost:<host port>`. `KEY=value` lines only.
4. If the project needs no database or external service, write `services: {}` and an empty env file.

The daemon runs `docker compose up -d` after your run finishes. Do not start containers yourself.
