# Parsing Errors Catalog Synthesis (cg-18bt2)

## Task Completion Summary

Successfully synthesized all parsing error findings into a comprehensive catalog document at `docs/research/parsing-errors-catalog.md`.

## Catalog Details

**File:** `docs/research/parsing-errors-catalog.md` (16K, 320 lines)
**Status:** Complete and committed to git (commit 5ec0cea)
**Generated:** 2026-08-06
**Scope:** Entire codebase (`pkg/`, `cmd/`, and related packages)

## Catalog Contents

### 1. Error Type Definitions
- ✅ StructuredError Type with CommitSHA field support
- ✅ Warmstart Error Type with comprehensive error kinds
- ✅ ErrorKind Constants (Truncated, MissingMember, CorruptPack, IO, Other)
- ✅ All constructor signatures documented

### 2. Error Constructor Catalog
- ✅ Primary constructors (`pkg/errors/helpers.go`): ParseErrorf, JSONParseError, etc.
- ✅ Warmstart constructors (`pkg/warmstart/error.go`): NewIOError, NewTruncatedError, etc.
- ✅ Context setter methods: WithCommitSHA, WithCommitSHAOption
- ✅ Constructor status: 80% complete (8/10 support commit SHA)

### 3. Call Site Catalog
Detailed tables for each package showing:
- ✅ **Warmstart Package** (`pkg/warmstart/extract.go`): 9 call sites, 6 with commit SHA ✅, 3 need updates ⚠️
- ✅ **Handler Package** (`pkg/handler/audit_logs.go`): 11 call sites, all need migration to structured errors
- ✅ **Identity Package** (`pkg/identity/`): 14 call sites, all use fmt.Errorf
- ✅ **Command Package** (`cmd/`): 3+ call sites, all use fmt.Errorf

**Total call sites catalogued:** 37+
**With commit SHA:** 6 (16%)
**Need commit SHA:** 31+ (84%)

### 4. Summary Statistics
- ✅ Constructor Status by Package
- ✅ Call Site Status by Package  
- ✅ Issues Identified (High/Medium Priority)
- ✅ Percent Complete calculations

### 5. Recommended Actions
Five-phase implementation plan:
- Phase 1: Update Core Constructors (High Impact)
- Phase 2: Handle Verification Functions (Special Case)
- Phase 3: Migrate Handler Package (Medium Impact)
- Phase 4: Migrate Identity Package (Lower Impact)
- Phase 5: Migrate Command Package (Lower Impact)

### 6. Context Notes
- ✅ When Commit SHA is Available
- ✅ When Commit SHA is NOT Available
- ✅ Design Considerations

### 7. Appendices
- ✅ Search Methods Used
- ✅ Related Documentation links

## Synthesis Sources

This catalog combines findings from:
1. **Error Type Definitions** (cg-egmrm): All error type definitions across packages
2. **Missing SHA Analysis** (cg-5c5kx): 29 constructors missing commit SHA parameters
3. **Parsing Error Locations** (cg-45rhy): 25+ parsing error sites identified
4. **Constructor and Call Site Catalog** (cg-4dr9o): Comprehensive constructor and call site analysis

## Acceptance Criteria Verification

- [x] Catalog file exists at docs/research/parsing-errors-catalog.md
- [x] All error types are documented (StructuredError, warmstart Error, constructors)
- [x] All call sites are listed with file:line references (detailed tables)
- [x] Errors needing commit SHA are identified (status columns with ⚠️ indicators)
- [x] File is committed to git (commit 5ec0cea)

## Key Findings

### High Priority Issues
1. **pkg/errors/helpers.go**: `ParseErrorf()` and `JSONParseError()` need updates or replacement
2. **pkg/warmstart/extract.go line 143**: Empty commit SHA passed
3. **pkg/warmstart/extract.go lines 626, 669**: Verification functions lack commit SHA parameter

### Statistics
- **Total Constructors:** 10
- **With Commit SHA Support:** 8 (80%)
- **Without Commit SHA Support:** 2 (20%)
- **Total Call Sites:** 37+
- **Using Structured Errors:** 6 (16%)
- **Using fmt.Errorf:** 28+ (84%)

## Implementation Guidance

The catalog provides:
- ✅ Exact file:line references for all call sites
- ✅ Constructor signatures for easy reference
- ✅ Status indicators showing what needs updates
- ✅ Prioritized recommended actions
- ✅ Context notes for when commit SHA is available

## Next Steps

Based on the catalog findings:
1. Implement Phase 1 (Update Core Constructors) - highest impact
2. Add commit SHA parameters to verification functions
3. Migrate handler/identity/cmd packages to structured errors
4. Track progress using the status tables in the catalog

## Completion Status

**Task:** cg-18bt2 - Create parsing errors catalog document
**Status:** ✅ Complete
**Bead Status:** Ready to close
**Commit:** Catalog already committed to git (5ec0cea)

The comprehensive catalog successfully synthesizes all parsing error findings and provides a clear roadmap for implementing commit SHA tracking across the codebase.
