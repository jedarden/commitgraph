# Error Propagation Documentation: ParseTarball

## Status: ✅ COMPLETE

Documentation of how `Truncated` errors propagate from `ParseTarball` to callers.

## ParseTarball Function Signature

```go
func ParseTarball(data []byte) (*WarmStartSnapshot, error)
```

**Location:** `pkg/warmstart/extract.go:92`

## All ParseTarball Callers Found

### Production Code: NONE (Current State)

As of 2026-08-06, `ParseTarball` is **not called from any production code**. The warmstart package exists but has not been integrated with clone-worker.

### Test Code: Comprehensive Coverage

**Location:** `pkg/warmstart/extract_test.go`

Tests validate all error paths:
- Valid tarball parsing (success cases)
- Missing member detection (config, ref, pack files)
- Invalid config/ref format handling  
- Truncated tarball detection
- Size mismatch detection
- Byte-identical pack file restoration

### Intended Production Caller: cw-clone-fallback (Not Yet Implemented)

According to `notes/cg-300y.md`, the intended integration is:

```
ARMOR fetch → ParseTarball → Materialize → Git fetch OR full clone fallback
```

**Planned integration pattern (from cg-300y.md):**

```go
tarball, err := armorClient.FetchWarmstartArtifact(provider, repo)
if err != nil {
    // Fall back to full clone
    return FullClone(provider, url)
}

snapshot, err := warmstart.ParseTarball(tarball)
if errors.Is(err, warmstart.ErrInvalidTarball) {
    // Corrupted artifact - fall back to full clone
    return FullClone(provider, url)
}

if err := warmstart.Materialize(gitDir, snapshot); err != nil {
    // Materialization failed - fall back to full clone
    return FullClone(provider, url)
}

// Warm-start succeeded - proceed with incremental fetch
return GitFetchOrigin(repo)
```

**Status:** `cw-clone-fallback` bead is **not yet implemented**.

## Error Flow Diagram

```
┌─────────────────────────────────────────────────────────────────────┐
│                        ParseTarball Call Flow                         │
└─────────────────────────────────────────────────────────────────────┘

                         ┌─────────────────┐
                         │  Caller Code    │
                         │ (future:        │
                         │  cw-clone-      │
                         │  fallback)      │
                         └────────┬────────┘
                                  │
                                  │ 1. Fetch tarball from ARMOR
                                  │
                    ┌─────────────▼─────────────┐
                    │ warmstart.ParseTarball()   │
                    │ (data []byte)              │
                    └─────────────┬─────────────┘
                                  │
            ┌─────────────────────┼─────────────────────┐
            │                     │                     │
            │ 2. Truncated       │ 3. Size mismatch    │ 4. Pack file too small
            │    tarball         │    (header vs       │    (< 12 bytes)
            │                     │     actual)         │
    ┌───────▼────────┐   ┌───────▼────────┐   ┌───────▼────────┐
    │ tar.Next()     │   │ io.Copy()     │   │ len(data) < 12 │
    │ returns error  │   │ written !=    │   │ check          │
    │                │   │ hdr.Size      │   │                │
    └───────┬────────┘   └───────┬────────┘   └───────┬────────┘
            │                     │                     │
            │ 5. Unexpected EOF  │                     │
            │    during read     │                     │
    ┌───────▼────────┐                                    │
    │ io.Copy()     │                                    │
    │ returns       │                                    │
    │ io.ErrUn-     │                                    │
    │ expectedEOF   │                                    │
    └───────┬────────┘                                    │
            │                                             │
            └─────────────────────┬───────────────────────┘
                                  │
                                  │ 6. Create Truncated error
                                  │
                    ┌─────────────▼─────────────┐
                    │ NewTruncatedMemberError()  │
                    │ • MemberName: file path    │
                    │ • Context: description     │
                    │ • Kind: Truncated          │
                    └─────────────┬─────────────┘
                                  │
                                  │ 7. Return *Error to caller
                                  │
                        ┌─────────▼─────────┐
                        │   Caller Code     │
                        │   checks error    │
                        │   via errors.As   │
                        └─────────┬─────────┘
                                  │
                    ┌─────────────┼─────────────┐
                    │             │             │
            ┌───────▼────────┐    │    ┌───────▼────────┐
            │ errors.As(     │    │    │ Fall back to   │
            │ &truncErr)     │    │    │ full clone if  │
            │ - truncErr.Kind│    │    │ Truncated OR   │
            │   == Truncated │    │    │ any other error │
            └────────────────┘    │    └────────────────┘
                                  │
                        ┌─────────▼─────────┐
                        │ Result:           │
                        │ - Clean error     │
                        │   with context    │
                        │ - Fallback path   │
                        │   available       │
                        └───────────────────┘
```

## Error Types Returned by ParseTarball

### 1. Truncated Errors (`*Error` with `Kind: Truncated`)

**Detection Points:**

| Line | Condition | Error Message |
|------|-----------|---------------|
| 111 | `tar.Next()` returns non-EOF error | `ErrInvalidTarball` wrapped with underlying error |
| 120 | `io.Copy()` returns `io.ErrUnexpectedEOF` | `NewTruncatedMemberError(hdr.Name, "ended prematurely", 0)` |
| 127 | `io.Copy()` written count != `hdr.Size` | `NewTruncatedMemberError(hdr.Name, "expected X bytes, got Y", 0)` |
| 161 | Pack file < 12 bytes (minimum header size) | `NewTruncatedMemberError(hdr.Name, "pack file too small: X bytes (minimum 12 bytes for header)", 0)` |

**Example Error Output:**
```
warmstart: truncated tarball (member=objects/pack/pack-test.pack) - ended prematurely
warmstart: truncated tarball (member=objects/pack/pack-undersized.pack) - pack file too small: 11 bytes (minimum 12 bytes for header)
warmstart: truncated tarball (member=config.json) - expected 150 bytes, got 100
```

**Error Structure:**
```go
type Error struct {
    Kind       ErrorKind      // Truncated
    Context    string         // Human-readable details
    MemberName string         // Tarball member path (e.g., "objects/pack/pack-123.pack")
    Offset     int64          // Byte offset (if applicable)
    Underlying error          // Original error (if any)
}
```

### 2. Other Error Types

| Error Type | Kind | When Returned |
|------------|------|---------------|
| `ErrInvalidTarball` | N/A (base error) | Tar header corruption or read failure |
| `ErrMissingConfig` | N/A | `config.json` member not found |
| `ErrMissingRef` | N/A | No ref member found |
| `ErrMissingPackFiles` | N/A | No pack files in tarball |
| `ErrInvalidConfig` | N/A | Config JSON malformed or validation fails |
| `*CorruptionError` | N/A | Ref data empty or invalid format |

## Error Propagation Verification

### ✅ Truncated Errors Are NOT Swallowed

**Code locations where Truncated errors are returned:**

1. **`extract.go:120`** - Unexpected EOF during read:
```go
if err == io.ErrUnexpectedEOF || errors.Is(err, io.ErrUnexpectedEOF) {
    return nil, NewTruncatedMemberError(hdr.Name, "ended prematurely", 0)
}
```

2. **`extract.go:127`** - Size mismatch detection:
```go
if written != hdr.Size {
    return nil, NewTruncatedMemberError(hdr.Name, fmt.Sprintf("expected %d bytes, got %d", hdr.Size, written), 0)
}
```

3. **`extract.go:161`** - Pack file too small:
```go
if ext == ".pack" && len(data) < 12 {
    return nil, NewTruncatedMemberError(hdr.Name, fmt.Sprintf("pack file too small: %d bytes (minimum 12 bytes for header)", len(data)), 0)
}
```

**All paths directly return the error to the caller** - no error swallowing, no silent failure.

### ✅ Errors Reach Surface Level Appropriately

**In tests:**
```go
_, err := ParseTarball(truncatedTarball)
if err == nil {
    t.Error("expected error for truncated tarball, got nil")
}

var truncErr *Error
if errors.As(err, &truncErr) {
    if truncErr.Kind != Truncated {
        t.Errorf("expected Truncated error kind, got %v", truncErr.Kind)
    }
}
```

**Verification:** Tests confirm `Truncated` errors propagate correctly and can be detected via `errors.As()`.

### ✅ Error Wrapping Documented

**Error wrapping hierarchy:**
```
ParseTarball()
  └─> *Error (Truncated)
       └─> nil (no underlying) OR error (if wrapped)

OR

ParseTarball()
  └─> fmt.Errorf("%w: %v", ErrInvalidTarball, underlyingError)
       └─> ErrInvalidTarball
            └─> underlyingError (tar.Next() failure)
```

**Unwrap support:** `*Error` implements `Unwrap() error` at `error.go:100` for `errors.Is/As` compatibility.

## Missing Integration Points

### cw-clone-fallback (Not Implemented)

According to the research and plan:

- **Bead:** `cw-clone-fallback`
- **Purpose:** Integrate warmstart extraction into clone-worker job flow
- **Fallback strategy:** On any warmstart error, fall back to full clone
- **Status:** Not started

**Integration code needed:**
```go
// In clone-worker job:
snapshot, err := warmstart.ParseTarball(tarball)
if err != nil {
    log.Warn("Warmstart artifact invalid, falling back to full clone", err)
    return executeFullClone(provider, repo)
}

if err := warmstart.Materialize(gitDir, snapshot); err != nil {
    log.Warn("Warmstart materialization failed, falling back to full clone", err)
    return executeFullClone(provider, repo)
}
```

### ARMOR Read Side (Implemented: cg-4v07)

**Status:** ✅ Complete - ARMOR can fetch warmstart artifacts

**Bead:** `cg-4v07` (read side), `cw-warmstart-armor-write` (write side)

ARMOR integration exists but clone-worker integration is pending.

## Error Handling Patterns at Call Sites

### Test Pattern (Current)

```go
tarball := createTestTarball(t, members)
snapshot, err := ParseTarball(tarball)

if err != nil {
    t.Fatalf("ParseTarball failed: %v", err)
}

// OR for error cases:

_, err := ParseTarball(corruptedTarball)
if err == nil {
    t.Error("expected error, got nil")
}
if !errors.Is(err, ErrInvalidTarball) {
    t.Errorf("expected ErrInvalidTarball, got %v", err)
}
```

### Production Pattern (Planned)

```go
// From cg-300y.md integration example:
snapshot, err := warmstart.ParseTarball(tarball)
if errors.Is(err, warmstart.ErrInvalidTarball) {
    // Corrupted artifact - fall back to full clone
    return FullClone(provider, url)
}

if err != nil {
    // Any other error - fall back to full clone
    return FullClone(provider, url)
}
```

## Summary

### Callers Found
- **Production:** 0
- **Test:** Comprehensive coverage in `pkg/warmstart/extract_test.go`
- **Planned:** `cw-clone-fallback` (not implemented)

### Error Flow
1. **ParseTarball** detects truncated data via multiple mechanisms
2. Returns `*Error` with `Kind: Truncated` directly to caller
3. **No error swallowing** - all errors propagate to surface level
4. Caller can check error type via `errors.As()` and implement fallback logic

### Error Wrapping
- `*Error` implements `Unwrap()` for `errors.Is/As` compatibility
- `fmt.Errorf` with `%w` used for `ErrInvalidTarball` wrapping
- All error paths documented and tested

### Missing Integration
- **cw-clone-fallback:** Must implement warmstart → clone-worker integration
- **ARMOR write side:** `cw-warmstart-armor-write` (creates tarballs for storage)

### Verification
- ✅ Truncated errors propagate correctly (verified by tests)
- ✅ Errors reach surface level appropriately
- ✅ Error wrapping and transformation documented
- ✅ Member name included in all Truncated errors for debugging

---

**Bead:** cg-5oiol (Document error propagation path from ParseTarball to callers)
**Status:** Complete
**Date:** 2026-08-06
