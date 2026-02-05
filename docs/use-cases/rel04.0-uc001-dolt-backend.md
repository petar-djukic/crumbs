# Use Case: Dolt Backend with Version Control

## Summary

A developer attaches a Dolt backend to a cupboard, performs CRUD operations, creates commits for checkpoints, and uses branching to explore alternatives. This tracer bullet validates the pluggable backend architecture and Dolt-specific version control features.

## Actor and Trigger

The actor is a developer or team using Dolt for version-controlled task tracking. The trigger is the need for audit trails, branching workflows, or collaborative task management with merge capabilities.

## Flow

1. **Configure Dolt backend**: Construct a Config with Backend="dolt" and DoltConfig specifying DSN (database connection string) and Branch (default branch name, e.g., "main").

2. **Attach cupboard**: Call `cupboard.Attach(config)`. The Dolt backend connects to the database, verifies or creates the schema, and checks out the specified branch.

3. **Verify connection**: Call `cupboard.GetTable("crumbs")` and perform a `Fetch()` to confirm the backend is operational.

4. **Create crumbs**: Add several crumbs via the Table interface:
   - `table.Set("", &Crumb{Name: "Task A"})`
   - `table.Set("", &Crumb{Name: "Task B"})`

5. **Verify persistence**: Call `table.Fetch()` and confirm both crumbs exist.

6. **Create a Dolt commit**: Use Dolt-specific operations to commit the current state:
   - Call `cupboard.Commit("Initial tasks")` or equivalent backend method
   - This creates a Dolt commit with the message

7. **Modify data**: Update a crumb's state:
   - `entity, _ := table.Get(crumbID)`
   - `crumb := entity.(*Crumb)`
   - `crumb.SetState("ready")`
   - `table.Set(crumb.CrumbID, crumb)`

8. **Create another commit**: Call `cupboard.Commit("Mark Task A ready")`.

9. **View commit history**: Use Dolt-specific operations to list commits:
   - `commits, _ := cupboard.Log()`
   - Verify two commits exist with correct messages

10. **Create a branch**: Call `cupboard.Branch("experiment")` to create a new branch from current HEAD.

11. **Switch to branch**: Call `cupboard.Checkout("experiment")` to switch to the experiment branch.

12. **Make changes on branch**: Add a crumb and commit:
    - `table.Set("", &Crumb{Name: "Experimental task"})`
    - `cupboard.Commit("Add experimental task")`

13. **Switch back to main**: Call `cupboard.Checkout("main")`. Verify the experimental task does not exist on main.

14. **Merge branch (optional)**: If merge is supported:
    - Call `cupboard.Merge("experiment")`
    - Verify the experimental task now exists on main

15. **Detach cupboard**: Call `cupboard.Detach()` to close the Dolt connection.

## Architecture Touchpoints

This use case exercises the following interfaces and components:

| Interface | Operations Used |
|-----------|-----------------|
| Cupboard | Attach, Detach, GetTable |
| Table | Get, Set, Fetch |
| Dolt-specific | Commit, Log, Branch, Checkout, Merge (optional) |

We validate:

- Pluggable backend architecture (prd-cupboard-core R2)
- Dolt backend configuration (prd-dolt-backend)
- Standard Table operations work with Dolt backend
- Dolt commit creation preserves point-in-time state
- Branch creation and switching
- Data isolation between branches
- Merge workflow (if supported)

## Success Criteria

The demo succeeds when:

- [ ] Attach with Dolt config connects to database
- [ ] GetTable returns functional Table interface
- [ ] CRUD operations (Get, Set, Fetch) work correctly
- [ ] Commit creates a Dolt commit with message
- [ ] Log returns commit history
- [ ] Branch creates a new branch
- [ ] Checkout switches branches
- [ ] Data on branches is isolated (changes on experiment not visible on main)
- [ ] Detach closes connection cleanly
- [ ] (Optional) Merge combines branch changes

Observable demo script:

```bash
# Run the demo binary or test
go test -v ./internal/dolt -run TestDoltBackend

# Or run a CLI demo with Dolt
crumbs demo dolt --dsn "file:///tmp/crumbs-dolt"
```

## Out of Scope

This use case does not cover:

- DynamoDB backend - separate use case
- Conflict resolution during merge
- Remote Dolt operations (push, pull, clone)
- Dolt SQL queries beyond ORM operations
- Multi-user collaboration workflows
- Performance optimization for large histories

## Dependencies

- rel01.0-uc001 (Cupboard lifecycle) must pass with SQLite
- prd-cupboard-core backend abstraction must support multiple backends
- prd-dolt-backend must be implemented
- Dolt database must be available (local file or remote)

## Risks and Mitigations

| Risk | Mitigation |
|------|------------|
| Dolt not installed on system | Document Dolt installation; skip tests if unavailable |
| Schema migration between Dolt versions | Version schema; test upgrade paths |
| Performance with large commit history | Pagination for Log; limit history depth |
| Merge conflicts | Start with fast-forward only; defer conflict resolution |
