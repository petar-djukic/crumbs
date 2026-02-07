# Use Case: JSONL Git Roundtrip

## Summary

A user creates crumbs, commits the JSONL files to git, deletes the SQLite database, re-attaches the cupboard, and verifies that all data is intact. This validates that JSONL files are the source of truth and that the SQLite backend rebuilds state correctly from JSONL on attach.

## Actor and Trigger

The actor is a developer or coding agent working in a git repository with cupboard data. The trigger is any scenario where the SQLite database is missing or stale—such as cloning a repository, switching branches, or recovering from a corrupted database.

## Flow

This use case validates the git integration conventions from eng01-git-integration and the JSONL persistence requirements from prd-sqlite-backend. The tracer bullet demonstrates the complete roundtrip: create data, persist to JSONL, commit to git, delete the ephemeral database, re-attach, verify data integrity.

1. Initialize a cupboard in a git repository.

```bash
mkdir -p /tmp/jsonl-roundtrip-demo
cd /tmp/jsonl-roundtrip-demo
git init
mkdir -p data
cat > config.yaml <<EOF
backend: sqlite
data_dir: ./data
EOF
echo "data/cupboard.db" >> .gitignore
export CUPBOARD_CONFIG=/tmp/jsonl-roundtrip-demo/config.yaml
```

The `.gitignore` excludes `cupboard.db` per eng01-git-integration: JSONL files are committed, SQLite database is ephemeral.

2. Create crumbs using the cupboard CLI.

```bash
cupboard create --type task --title "First task" --description "Initial work item" --json
# Capture: TASK1_ID=<uuid>
cupboard create --type epic --title "Epic one" --description "Parent work" --labels "code,infra" --json
# Capture: EPIC_ID=<uuid>
cupboard create --type task --title "Second task" --description "Follow-up work" --json
# Capture: TASK2_ID=<uuid>
```

Each `cupboard create` calls `Table.Set("", crumb)` which generates a UUID v7 (prd-cupboard-core R8), persists to SQLite, and writes to JSONL atomically (prd-sqlite-backend R5.2). The immediate sync strategy ensures JSONL files are current after every write (prd-sqlite-backend R16.2).

3. Verify JSONL files exist and contain the data.

```bash
ls -la data/
# Expected files: crumbs.jsonl, trails.jsonl, links.jsonl, properties.jsonl,
#                 categories.jsonl, crumb_properties.jsonl, cupboard.db, etc.
cat data/crumbs.jsonl
# Expected: Three lines, one JSON object per crumb
wc -l data/crumbs.jsonl
# Expected: 3
```

The JSONL format follows prd-sqlite-backend R2: one JSON object per line, RFC 3339 timestamps, lowercase hyphenated UUIDs.

4. Commit the JSONL files to git.

```bash
git add config.yaml .gitignore
git add data/*.jsonl
git commit -m "Add initial cupboard data"
```

Per eng01-git-integration, JSONL files are the source of truth and are committed to git. The SQLite database is gitignored and regenerated from JSONL on every `Attach`.

5. Verify the SQLite database exists (pre-deletion baseline).

```bash
ls data/cupboard.db
# Expected: File exists
cupboard list --json | jq length
# Expected: 3
```

6. Delete the SQLite database to simulate a fresh clone or branch switch.

```bash
rm data/cupboard.db
ls data/cupboard.db
# Expected: File not found
```

This simulates what happens when a developer clones the repository (only JSONL files are in git) or switches to a branch with different data. The SQLite database is ephemeral and not version-controlled.

7. Re-attach by running a cupboard command.

```bash
cupboard list --json | jq length
# Expected: 3
```

On attach, the SQLite backend performs the startup sequence (prd-sqlite-backend R4):
1. Creates `cupboard.db` with the schema (R4.3, R4.4)
2. Loads each JSONL file into the corresponding SQLite table (R4.5)
3. Validates foreign key relationships (R4.6)
4. Returns a ready Cupboard instance (R4.7)

8. Verify all crumbs are intact.

```bash
cupboard show $TASK1_ID
# Expected: Title "First task", State "open", Type "task"
cupboard show $EPIC_ID
# Expected: Title "Epic one", Type "epic", Labels "code,infra"
cupboard show $TASK2_ID
# Expected: Title "Second task", State "open", Type "task"
```

Each `cupboard show` calls `Table.Get(id)` which queries SQLite (now rebuilt from JSONL) and hydrates the row into a Crumb entity (prd-sqlite-backend R14).

9. Verify queries work correctly.

```bash
cupboard ready --json --type task
# Expected: JSON array with TASK1_ID and TASK2_ID (state is open)
cupboard ready --json --type epic
# Expected: JSON array with EPIC_ID
```

`Table.Fetch` with filter queries the rebuilt SQLite database (prd-crumbs-interface R9, R10).

10. Modify data and verify JSONL is updated.

```bash
cupboard update $TASK1_ID --status in_progress
cat data/crumbs.jsonl | grep $TASK1_ID
# Expected: Line shows updated state
cupboard close $TASK1_ID
cat data/crumbs.jsonl | grep $TASK1_ID
# Expected: Line shows closed state
```

Writes update SQLite first, then persist atomically to JSONL (prd-sqlite-backend R5.1, R5.2).

11. Commit the updated JSONL files.

```bash
git add data/*.jsonl
git commit -m "Close first task"
git log --oneline
# Expected: Two commits with JSONL changes
```

Per eng01-git-integration, JSONL changes are committed alongside code changes, enabling traceability between task state and commit history.

12. Repeat the roundtrip: delete database, re-attach, verify.

```bash
rm data/cupboard.db
cupboard show $TASK1_ID
# Expected: State "closed"
cupboard ready --json --type task
# Expected: Only TASK2_ID (TASK1_ID is closed)
```

This confirms the roundtrip works for modified data as well as initial data.

## Architecture Touchpoints

Table 1: Components and references

| Component | Operation | Reference |
|-----------|-----------|-----------|
| SQLite backend | Startup sequence: delete db, create schema, load JSONL | prd-sqlite-backend R4 |
| SQLite backend | JSONL as source of truth | prd-sqlite-backend R1.2 (cupboard.db is ephemeral) |
| SQLite backend | Atomic write pattern for JSONL | prd-sqlite-backend R5.2 |
| SQLite backend | Immediate sync strategy | prd-sqlite-backend R16.2 |
| SQLite backend | Entity hydration from SQLite rows | prd-sqlite-backend R14 |
| Cupboard interface | Attach initializes backend | prd-cupboard-core R4 |
| Git integration | JSONL files committed, cupboard.db gitignored | eng01-git-integration § Data Directory in Git |
| Git integration | Commit JSONL alongside code changes | eng01-git-integration § Commit Conventions |

This use case exercises:

- **JSONL persistence**: All writes persist to JSONL files atomically (prd-sqlite-backend R5)
- **Startup loading**: JSONL files are loaded into SQLite on attach (prd-sqlite-backend R4)
- **Git integration**: JSONL files are the versioned source of truth (eng01-git-integration)
- **Database regeneration**: SQLite is ephemeral and rebuilt from JSONL (prd-sqlite-backend R1.2)

## Success / Demo Criteria

Run the following sequence and verify observable outputs.

Table 2: Demo script

| Step | Command | Verify |
|------|---------|--------|
| 1 | `git init && mkdir data && echo "data/cupboard.db" >> .gitignore` | Repository initialized with cupboard.db gitignored |
| 2 | `cupboard create --type task --title "Test" --json` | Crumb created, JSON output includes crumb_id |
| 3 | `cat data/crumbs.jsonl \| wc -l` | Output is `1` (one crumb in JSONL) |
| 4 | `ls data/cupboard.db` | File exists (SQLite database created) |
| 5 | `git add data/*.jsonl && git commit -m "Add crumb"` | Commit succeeds, JSONL files tracked |
| 6 | `rm data/cupboard.db` | Database deleted |
| 7 | `cupboard list --json \| jq length` | Output is `1` (data intact after rebuild) |
| 8 | `cupboard show <crumb_id>` | Output shows correct title, state, type |
| 9 | `ls data/cupboard.db` | File exists (database regenerated on attach) |
| 10 | `cupboard update <crumb_id> --status in_progress` | Update succeeds |
| 11 | `rm data/cupboard.db && cupboard show <crumb_id>` | State shows as in_progress (JSONL has updated state) |

The roundtrip demonstrates that:
- JSONL files are the source of truth
- SQLite is ephemeral and rebuilds from JSONL
- Git tracks JSONL, not SQLite
- Data survives database deletion

## Out of Scope

- Merge conflict resolution when JSONL files diverge between branches (eng01-git-integration § JSONL Merge Behavior describes this but it is not validated here)
- Trail-scoped worktrees (eng01-git-integration § Trails as Worktrees)
- Corrupt JSONL recovery (prd-sqlite-backend R7 describes error handling but this use case assumes valid files)
- Cross-process access (prd-sqlite-backend R8.5 explicitly does not support this)
- Non-immediate sync strategies (prd-sqlite-backend R16.3, R16.4)

## Dependencies

- SQLite backend with JSONL persistence (prd-sqlite-backend R1–R5)
- Startup sequence loads JSONL into SQLite (prd-sqlite-backend R4)
- Cupboard interface with Attach/Detach (prd-cupboard-core R4, R5)
- Table interface with Get/Set/Fetch (prd-cupboard-core R3)
- Issue-tracking CLI commands (rel00.0-uc003-issue-tracking-cli)

## Risks and Mitigations

Table 3: Risks

| Risk | Mitigation |
|------|------------|
| JSONL write fails silently, SQLite ahead of JSONL | Immediate sync strategy writes JSONL on every commit; next attach reconciles from JSONL (prd-sqlite-backend R5.4) |
| JSONL files grow unbounded | JSONL uses stable insertion order; rows update in place, new rows append (eng01-git-integration § JSONL Merge Behavior) |
| User commits cupboard.db by mistake | .gitignore template and eng01-git-integration documentation prevent this |
| JSONL format changes break old files | Unknown fields are ignored (prd-sqlite-backend R7.2); forward compatibility maintained |
