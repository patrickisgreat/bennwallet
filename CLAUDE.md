# BennWallet

## What is this project?

BennWallet is a shared-expense tracker for two people. Each user records transactions they paid for or owe, tags them against YNAB categories, settles debts with each other, and pulls reports that match what YNAB shows. A Go API serves a React single-page app from one binary.

## Tech Stack

- **Backend**: Go 1.24, Gorilla Mux, `database/sql` + `lib/pq`, Firebase Admin SDK for token verification
- **Database**: PostgreSQL only. SQLite was removed; do not add it back.
- **Frontend**: React 19 + TypeScript, Vite, Tailwind CSS, Vitest
- **Integrations**: YNAB API (categories and transaction sync), Firebase Auth
- **Deploy**: Docker image built by GitHub Actions and pushed to DigitalOcean. `fly.toml` and the `*fly*.sh` scripts at the repo root are legacy from the Fly.io era.

## Common Commands

```bash
# Backend (run from backend/)
go build ./...                       # Compile everything
go vet ./...                         # Static checks
go test ./...                        # Needs a reachable PostgreSQL (see Testing)
go run . --port :8080                # Run the API + static frontend
go run . migrate --dry-run           # List pending migrations
go run . migrate                     # Apply pending migrations
RESET_DB=true go run .               # Drop, recreate and seed, then exit

# Frontend (run from repo root)
npm run dev                          # Vite dev server
npm run build                        # Production build into dist/
npm test                             # Vitest
npm run lint                         # ESLint
```

## Project Structure

```
backend/
├── main.go              # Flags, startup order, route table, `migrate` subcommand
├── database/
│   ├── db.go            # Connection config from env, InitDB, base schema + default seed
│   └── schema.go        # Column helpers used by migrations
├── migrations/
│   ├── main.go          # The ordered migration registry and runner
│   ├── base_schema.go   # CreateBaseSchema / DropAllTables used on reset
│   └── *.go             # One file per migration; seeds guard themselves with isProduction()
├── middleware/          # AuthMiddleware (Firebase), CORS, permission checks
├── handlers/            # HTTP handlers, one file per resource; *_test.go hit a real DB
├── services/            # Business logic: settlements, permissions, filters, YNAB, secrets, env loading
├── models/              # Structs and the SQL that loads/saves them
├── security/            # AES encryption for stored YNAB tokens
├── testutil/            # Creates a fresh, migrated, seeded test database per test
└── scripts/init-db.sh   # Copied into the Docker image; connection check at container start
src/                     # React app
```

## Architecture Notes

### Startup order matters

`main.go` runs in this order: load dotenv files, initialise encryption, `database.InitDB` (connect, create base tables, seed defaults on `RESET_DB`), fix the YNAB config table, run migrations, then start serving. A `RESET_DB=true` run exits after migrations unless `--no-exit` is passed. Keep new startup work in that order and in `main.go`; do not add a second entrypoint.

### Two schema paths exist; keep them in sync

`database.CreatePostgresSchema` builds tables on every start with `CREATE TABLE IF NOT EXISTS`. `migrations.CreateBaseSchema` builds them on a reset. Both must agree on every table and column, and any new column also needs a migration in the registry so existing databases pick it up. A change to one without the others is the most common way to break production while tests still pass.

### Migrations are a Go slice, not files on disk

`migrations/main.go` holds `registry`, the single ordered list. Add a migration by writing a function `func(*sql.DB) error` and appending it. Names are recorded in the `migrations` table and never re-run, so a migration must be safe to leave applied forever. Seed migrations call `isProduction()` and no-op in production; keep that guard on any seed you add.

### Auth has a deliberate dev bypass

If no Firebase credentials are found in the environment, `middleware.AuthMiddleware` injects a fixed admin user ID into every request. That is how tests and local development work. Production is only secure because `FIREBASE_SERVICE_ACCOUNT_JSON` (or the Base64 variant) is set. Never add another bypass and never change the behaviour when credentials are present without an auth test.

### Environment detection

"Production" means any of `APP_ENV`, `NODE_ENV`, `ENVIRONMENT` or `ENV` equals `production`. Database settings come from `DATABASE_URL` or the `DB_*` variables. Local `.env.local`, `.env` and `../.env` are read at startup without overriding variables already set.

### Settlements

A settlement groups transactions between two users and tracks what remains owed. `services/settlement.go` owns the balance math; handlers only validate and marshal. The three settlement test files in `handlers/` are the regression suite for this feature and run against a real database.

## Conventions

- `gofmt` and `go vet` clean at all times; `staticcheck` and `deadcode` are run before merging and should report nothing.
- Parameterised SQL only. Build dynamic `WHERE` clauses with positional `$n` arguments, never with string interpolation of values.
- Errors bubble up with `fmt.Errorf("...: %w", err)`; handlers decide the HTTP status. Do not log and swallow.
- Log with `log.Printf`, no debug prefixes, no secrets, no tokens. Mask with `database.MaskPassword` where a connection string is logged.
- Test helpers shared within a package live in `*_test.go` files so they never ship in the binary.

## Code Standards

- **DRY**: Extract shared logic. If a query or check appears twice, it belongs in `services/` or `models/`.
- **Single responsibility**: One handler per HTTP action, one service function per business operation. If a function needs "and" to describe it, split it.
- **Small functions**: Prefer many short, composable functions over long ones.
- **No over-engineering**: Write the minimum code that solves the problem. No speculative interfaces, no abstractions for one caller.
- **Intention-revealing names**: Code should read like prose and need few comments.
- **No dead code**: Remove unused functions, imports, commented-out blocks, backup files and compiled binaries. If `deadcode` reports it, delete it.

## Testing

No PR merges without tests covering the behaviour it introduces or changes.

### How backend tests run

Tests create a brand-new PostgreSQL database per test through `testutil.SetupPostgresTestDB`, run the full migration registry against it (which also seeds the development data), and drop it on cleanup. They need a server reachable with these variables, defaulting to `localhost:5432` with `postgres`/`postgres`:

```bash
POSTGRES_HOST POSTGRES_PORT POSTGRES_USER POSTGRES_PASSWORD
```

A throwaway server for local runs:

```bash
docker run -d --rm --name bennwallet-test-pg -e POSTGRES_PASSWORD=postgres -p 55432:5432 postgres:16-alpine
POSTGRES_PORT=55432 go test ./...
```

CI starts the same kind of container in `.github/workflows/ci-cd.yml`.

### The pyramid

- **Unit tests** for pure logic in `models/` and `services/` that need no database. They should run in milliseconds.
- **Integration tests** for anything that touches SQL or HTTP. Use the real test database; do not mock `*sql.DB`. Test the contract at the boundary: request and response shape, status code, rows written.
- **Frontend** behaviour is covered with Vitest in `src/`.

### Rules

- Test behaviour, not implementation. A test that breaks on a rename is testing the wrong thing.
- Each test owns its data. Create what it needs and never depend on another test's rows or on execution order.
- A test that cannot fail is not a test. Break the code once to confirm the test catches it.
- Name tests for what they assert: `TestCreateSettlement_RejectsUnknownCounterparty`, not `TestSettlement2`.
- "It's a small change", "it's hard to test" and "tests in a follow-up" are not reasons to skip tests.

## Security

- Never commit secrets. `.env*`, service-account JSON and `*.db` files are gitignored; keep them that way.
- YNAB tokens are encrypted at rest with `ENCRYPTION_KEY`. The default development key is insecure by design; production must set a real one.
- Validate input at the handler boundary: IDs, dates, amounts, pagination.
- Every route except `/health` sits behind `AuthMiddleware`. Resource ownership is checked in `services/` and `middleware/permissions.go`; a new resource type needs both.
- Follow OWASP guidance for injection, broken access control and sensitive data exposure.

## Git Workflow

- Work on a feature branch unless the owner explicitly asks for a direct push to `main`.
- Small, focused commits with conventional prefixes: `feat:`, `fix:`, `refactor:`, `test:`, `chore:`, `docs:`, `perf:`, `style:`.
- Messages describe what and why: `fix: record note column migration once` not `update schema.go`.
- Open PRs with `gh pr create`, titled with the same prefixes, with a summary and a test plan in the body.
- The owner reviews and merges PRs. Do not merge autonomously.
- Never add AI attribution trailers, co-author lines or "generated with" notes to commits or PRs.

### PR Description Template

```markdown
## Scope

<!-- What you changed and why. closes #<issue> -->

## Implementation

<!-- How: the flow, any refactor, trade-offs, anything reviewers should look at closely. -->

## How to Test

<!-- 1) Automated: which tests you added or updated. 2) Manual: steps to see the change. -->

## Schema & Risk

<!-- Does this touch CreatePostgresSchema, CreateBaseSchema, the migration registry, auth,
or anything that changes stored data? Confirm both schema paths and the registry agree. -->
```
