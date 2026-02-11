// Package types defines the shared contract layer for the Crumbs system.
// It contains entity structs, interfaces, error types, and constants.
// No implementation lives here; backend packages in internal/ implement
// the Cupboard and Table interfaces.
//
// Implements: prd001-cupboard-core (Config, Cupboard, Table interfaces);
//
//	prd003-crumbs-interface (Crumb struct, state constants, entity methods);
//	prd004-properties-interface (Property, Category structs, value types);
//	prd005-metadata-interface (Metadata, Schema structs);
//	prd006-trails-interface (Trail struct, state constants, entity methods);
//	prd007-links-interface (Link struct, link type constants);
//	prd008-stash-interface (Stash, StashHistoryEntry structs, entity methods).
//
// See docs/ARCHITECTURE.md for system design.
package types
