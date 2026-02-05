# Use Case: Property Enforcement

## Summary

A developer defines custom properties, verifies built-in properties are seeded, and validates that all crumbs automatically have values for all defined properties. This tracer bullet validates the property enforcement mechanism: auto-initialization on crumb creation and backfill on property definition.

## Actor and Trigger

The actor is a developer or automated test harness. The trigger is the need to validate that the property system enforces the invariant: every crumb has a value for every defined property, with no gaps or partial state.

## Flow

1. **Open cupboard with seeded properties**: Call `OpenCupboard` with a SQLite backend. The backend seeds built-in properties (priority, type, description, owner, labels, dependencies) during initialization.

2. **Verify built-in properties**: Call `Properties().List()` to confirm the six built-in properties exist with correct value types and categories.

3. **Add a crumb**: Call `Crumbs().Add("Implement feature X")`. The operation creates the crumb and auto-initializes all six built-in properties with their default values.

4. **Verify property initialization**: Call `Crumbs().GetProperties(crumbID)`. Confirm it returns all six properties with default values (empty strings for text, empty lists for list types, null for categorical).

5. **Set a property value**: Call `Crumbs().SetProperty(crumbID, priorityPropertyID, highCategoryID)` to set priority on the crumb. Verify UpdatedAt changes.

6. **Get a property value**: Call `Crumbs().GetProperty(crumbID, priorityPropertyID)`. Verify it returns the value set in the previous step.

7. **Define a custom property**: Call `Properties().Define("estimate", "Story point estimate", "integer")`. The operation creates the property and backfills all existing crumbs with the default value (0 for integer).

8. **Verify backfill occurred**: Call `Crumbs().GetProperties(crumbID)`. Confirm the crumb now has seven properties including "estimate" with value 0.

9. **Add another crumb after property definition**: Call `Crumbs().Add("Another task")`. Verify the new crumb has all seven properties (six built-in plus estimate) auto-initialized.

10. **Update custom property**: Call `Crumbs().SetProperty(crumbID, estimatePropertyID, 5)` to set the estimate to 5.

11. **Clear property to default**: Call `Crumbs().ClearProperty(crumbID, estimatePropertyID)`. Verify the estimate resets to 0 (the default), not null or unset.

12. **Verify invariant holds**: Call `Crumbs().GetProperties()` on both crumbs. Confirm each has exactly seven properties with no gaps.

13. **Close the cupboard**: Call `Close()`.

## Architecture Touchpoints

This use case exercises the following interfaces and components:

| Interface | Operations Used |
|-----------|-----------------|
| Cupboard | OpenCupboard, Close |
| CrumbTable | Add, SetProperty, GetProperty, GetProperties, ClearProperty |
| PropertyTable | Define, List |

We validate:

- Built-in property seeding (prd-properties-interface R8, prd-sqlite-backend R9)
- Crumb creation with property auto-initialization (prd-crumbs-interface R3.7)
- Property definition with backfill to existing crumbs (prd-properties-interface R4.9)
- SetProperty updates value and timestamp (prd-crumbs-interface R10)
- GetProperty retrieves current value (prd-crumbs-interface R11)
- ClearProperty resets to default, not null (prd-crumbs-interface R12.2)
- Invariant: every crumb has every property (no partial state)

## Success Criteria

The demo succeeds when:

- [ ] Properties().List() returns six built-in properties after initialization
- [ ] Newly added crumbs have all defined properties with default values
- [ ] SetProperty updates value and changes UpdatedAt
- [ ] GetProperty returns the current value
- [ ] Properties().Define() creates property and backfills existing crumbs
- [ ] GetProperties() returns all properties (never partial) for any crumb
- [ ] ClearProperty() resets value to default, not null or unset
- [ ] Crumbs added after Define have the new property auto-initialized
- [ ] No crumb ever has fewer properties than are defined

Observable demo script:

```bash
# Run the demo binary or test
go test -v ./internal/sqlite -run TestPropertyEnforcement

# Or run a CLI demo
crumbs demo properties --datadir /tmp/crumbs-demo
```

## Out of Scope

This use case does not cover:

- Core CRUD operations (Add, Get, Archive, Purge, Fetch) - see rel01.0-uc003
- Category management beyond built-in categories
- Property deletion or renaming
- Trail operations - see rel03.0-uc001
- Concurrent property definition and crumb creation
- Property value validation beyond type checking

## Dependencies

- rel01.0-uc001 (Cupboard lifecycle) must pass
- rel01.0-uc003 (Core CRUD) must pass
- prd-properties-interface must be implemented (PropertyTable operations)
- prd-crumbs-interface property methods must be implemented

## Risks and Mitigations

| Risk | Mitigation |
|------|------------|
| Backfill performance on large datasets | Test with 1000+ crumbs to validate acceptable latency |
| Atomic transaction failures | Verify rollback behavior when backfill fails mid-operation |
| Default value edge cases (null vs zero) | Explicit test cases for each value type's default |
| Race condition on Define + Add | Document that Define should complete before concurrent Adds |
