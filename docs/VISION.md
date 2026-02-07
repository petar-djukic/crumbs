# Crumbs Vision

## Executive Summary

Crumbs is a general-purpose storage system for work items with built-in support for exploratory work sessions. We use the breadcrumb metaphor (Hansel and Gretel): the **cupboard** holds all work items (crumbs), and **trails** are exploration paths you can complete (merge crumbs into the permanent record) or abandon (backtrack—clean up the entire trail). We provide a command-line tool and Go library. We are not a workflow engine, coordination framework, or message queue.

## The Problem

Agents that explore solutions need storage that supports how they actually work—exploring, hitting dead ends, and backtracking. A coding agent exploring implementation approaches, a chess engine evaluating move sequences, a planning system testing strategies—all create tasks and subtasks as they explore. Sometimes the approach works and those tasks become permanent work. Sometimes the agent realizes the approach will not work and needs to abandon the entire exploration without polluting the main task list.

Current task storage systems lack support for this exploratory workflow. They couple directly to a specific database or workflow engine, making it difficult to switch backends. More importantly, they have no concept of tentative work sessions. Agents must either commit failed exploration tasks to the permanent record or manually track and clean up abandoned items. Neither is acceptable—the first pollutes the task history with dead ends, the second is error-prone and complex.

## What This Does

Crumbs solves this by providing storage with first-class support for trails. We use the breadcrumb metaphor (Hansel and Gretel) because it naturally captures how exploratory work flows.

**Crumbs** are individual work items. You drop crumbs as you explore an implementation. Each crumb can depend on other crumbs, forming a directed acyclic graph (DAG) of the workflow. Crumbs have terminal states: **pebble** (completed successfully—permanent and enduring, like the pebbles Hansel used to find his way home) or **dust** (failed or abandoned—swept away, like the bread crumbs eaten by birds).

**Trails** are exploration sessions --  DAG subgraphs of the workflow -- that agents create while exploring an approach. Each trail is a DAG: crumbs within the trail have explicit dependencies on each other, forming the graph structure. You can drop new crumbs on a trail at any time and specify their dependencies. Trails can also branch: you can deviate (start a new trail that branches from a crumb on the current trail), dead-end (the approach fails and you abandon the entire trail DAG), or merge back (complete the trail successfully and the entire DAG becomes part of the permanent record). A path is a special case where the trail is linear (each crumb has at most one dependency).

**Stashes** enable crumbs on a trail to share state. A stash can hold resources (files, URLs), artifacts (outputs from one crumb as inputs to another), context (shared configuration), counters (atomic numeric state), or locks (mutual exclusion). Stashes are scoped to a trail or global, versioned, and maintain a full history of changes for auditability.

**Properties** extend entities with custom attributes. We design properties as a general mechanism that applies to crumbs, trails, and stashes uniformly. Every entity has a value for every defined property in its scope—there is no "not set" state. Built-in crumb properties (priority, type, description, owner, labels) are available out of the box. Applications can define additional properties at runtime without schema migrations.

Trail properties describe exploration sessions. A research project might define properties such as hypothesis, approach, or risk-level for trails. When agents create trails for different implementation approaches, they annotate each trail with properties that help them compare alternatives and decide which path to pursue.

Stash properties describe shared state containers. A build system might define properties such as build-stage, artifact-type, or retention-policy for stashes. When crumbs on a trail produce artifacts, the stash properties help categorize and manage those outputs across the exploration session.

**Cupboard** is the storage system that holds all crumbs, trails, and stashes.

The **Cupboard** interface provides table access and lifecycle management. You call `cupboard.GetTable("crumbs")` to get a Table, then use uniform CRUD operations: `Get(id)`, `Set(id, entity)`, `Delete(id)`, and `Fetch(filter)`. When `id` is empty, Set generates a UUID v7 and creates a new entity. All entity types (Crumb, Trail, Property, Link, Stash, Metadata) use the same Table interface.

Entity types have methods that modify struct fields in memory. Crumbs have `Pebble()` (mark completed) and `Dust()` (mark failed/abandoned), plus property methods. Trails have `Complete()` and `Abandon()`. After calling entity methods, you persist changes with `Table.Set`. Crumbs are linked to trails via the links table using `belongs_to` relationships.

The storage system provides a SQLite backend with JSONL files as the source of truth and SQLite as the query engine. All identifiers use UUID v7 for time-ordered, sortable IDs. Properties are first-class entities—you define new properties at runtime without schema migrations.

We provide both a command-line tool and a Go library. Any agent that needs backtracking can use Crumbs: coding agents exploring implementation approaches, task boards managing work items, game-playing agents (chess, go) evaluating move sequences, planning systems testing strategies, and more. The library and CLI also support personal task tracking.

## What Success Looks Like

We measure success along three dimensions.

### Performance and Scale

Operations complete with low latency as crumb counts and concurrent trails grow. We establish performance baselines as the codebase expands and refine targets based on real usage patterns.

### Developer Experience

Developers integrate the Go library quickly. The API is synchronous and type-safe, using a uniform Table interface for all entity types. Adding a new backend takes hours, not days. Defining new properties or metadata tables requires no schema migrations.

### Agent Workflow

Agents create trails for exploration, drop crumbs as they work, and mark trails as abandoned or completed. Completing a trail merges its crumbs into the permanent record; abandoning a trail deletes its crumbs. Trail-based exploration feels natural for any agent that needs to try approaches, backtrack from dead ends, and commit successful paths.

## What This Is NOT

We are not building a workflow engine. Coordination semantics (claiming work, timeouts, announcements) belong in layers above this storage—frameworks like Task Fountain that build on Crumbs.

We are not building a message queue. Crumbs stores work items; it does not route messages or provide pub/sub.

We are not building an HTTP/RPC API. Applications using Crumbs define their own APIs. The command-line tool provides a local interface; distributed coordination is out of scope.

We are not building replication or multi-region support.

We are not building a general-purpose database. Crumbs is purpose-built for work item storage with trails, properties, and metadata.
