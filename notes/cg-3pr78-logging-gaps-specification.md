# Production Logging Gaps Specification

**Task:** cg-3pr78 - Identify and document missing log information  
**Based on:** cg-5kndk logging review (cg-62hsv test run)  
**Production Scale:** 349,425 author/login pairs to process  
**Date:** 2026-08-06

---

## Executive Summary

The commitgraph system has **excellent logging in ingest operations** (`ingestlog`, `audit` packages) but **zero logging in critical operational areas** (`warmstart`, service operations, performance metrics). This specification maps identified gaps to production requirements and prioritizes what MUST be implemented for successful production operation at scale.

**Key Finding:** The warmstart package - responsible for processing git repository warm-start snapshots - has NO application logging. During the cg-62hsv test run, zero application-level logs were produced, making it impossible to debug production issues or monitor progress.

---

## Production Requirements Mapping

### Scale Requirements (349,425 records)

| Operation | Record Count | Estimated Duration | Logging Need |
|-----------|--------------|-------------------|--------------|
| Email resolution seed | 349,425 pairs | 2-4 hours | CRITICAL - progress tracking |
| Warmstart extraction | 98,747 repos | 8-16 hours | CRITICAL - per-repo visibility |
| Batch imports | Variable | Unknown without logs | CRITICAL - completion detection |
| Validation operations | All records | Variable | HIGH - failure diagnosis |

**Production Reality:** Without progress logging, a 10-hour operation could hang silently for hours before detection. With 349K+ records, even a 1% failure rate means 3,493 problematic records that need individual diagnosis.

---

## Critical Gaps (MUST IMPLEMENT - Production Blockers)

### Gap 1: warmstart Package - Zero Application Logging

**Priority:** CRITICAL - Blocks production deployment  
**Impact:** Cannot debug warmstart failures or monitor progress during large-scale operations  
**Production Requirement:** Must process 98,747 repos with warmstart snapshots

#### Missing Information:

1. **Tarball Extraction Progress**
   - **What's missing:** No logging during tarball parsing
   - **Why it matters:** Operations need to know if extraction is running, hung, or failed
   - **Production impact:** A 10-hour warmstart extraction could fail silently at hour 2

2. **File-by-File Processing Visibility**
   - **What's missing:** No logging of which files are being extracted from tarballs
   - **Why it matters:** When validation fails, operators need to know exactly which file caused the failure
   - **Production impact:** Cannot debug "missing .ref file" errors without knowing which pack files were found

3. **Validation Failure Context**
   - **What's missing:** Error messages exist but aren't logged for analysis
   - **Why it matters:** Operators need to see patterns in validation failures (corruption vs. missing files)
   - **Production impact:** With 98K repos, cannot manually inspect each failure

4. **Performance Metrics**
   - **What's missing:** No timing data, memory usage, or file size metrics
   - **Why it matters:** Cannot detect performance regression or resource exhaustion
   - **Production impact:** Memory leaks or slowdowns go undetected until OOM

#### Specification for warmstart Logging:

```go
// REQUIRED: ParseTarball operation logging
logger.LogInfo("ParseTarball started", 
    "size_bytes", len(data),
    "repo_count", repoCount)

// REQUIRED: Per-file extraction logging (debug level)
logger.LogDebug("Extracting file", 
    "path", header.Name,
    "size_bytes", header.Size,
    "file_type", classifyFileType(header.Name))

// REQUIRED: Validation completion
logger.LogInfo("Validation completed",
    "pack_files_found", len(packFiles),
    "ref_files_found", len(refFiles),
    "missing_refs", len(missingRefFiles))

// REQUIRED: Error context with full details
if len(missingRefFiles) > 0 {
    logger.LogError("Missing .ref files detected",
        "missing_files", missingRefFiles,
        "total_pack_files", len(snapshot.PackFiles),
        "severity", "critical")
}

// REQUIRED: Performance summary
logger.LogInfo("ParseTarball completed",
    "duration_ms", time.Since(start).Milliseconds(),
    "files_processed", totalFiles,
    "bytes_processed", totalBytes)
```

**Implementation Location:** Create `pkg/warmstart/logger.go` following `pkg/ingestlog/logger.go` patterns

---

### Gap 2: Batch Processing Progress Tracking

**Priority:** CRITICAL - Blocks production operations  
**Impact:** No visibility into long-running batch operations (349,425 records)  
**Production Requirement:** Must monitor seed operations and track completion

#### Missing Information:

1. **Records Processed Counter**
   - **What's missing:** No running count of how many records have been processed
   - **Why it matters:** Operators need ETA and completion percentage for multi-hour jobs
   - **Production impact:** Cannot tell if a 4-hour seed operation is progressing or hung

2. **Records Skipped Tracking**
   - **What's missing:** No logging of WHY records are skipped (duplicate? error? filtered?)
   - **Why it matters:** Need to distinguish between successful skips and unexpected failures
   - **Production impact:** Silent data loss if errors are misclassified as "skipped"

3. **Records Ingested Counter**
   - **What's missing:** No count of successfully written records
   - **Why it matters:** Cannot verify data integrity after operation completes
   - **Production impact:** Don't know if 349,425 records or 200,000 actually made it to database

4. **Progress Rate and ETA**
   - **What's missing:** No records/second calculation or time-to-completion estimate
   - **Why it matters:** Operations planning and timeout management
   - **Production impact:** Cannot set appropriate timeouts or alert on slowdowns

#### Specification for Batch Progress Logging:

```go
// REQUIRED: Operation start
logger.LogInfo("Batch operation started",
    "operation_type", "email_resolution_seed",
    "total_records", 349425,
    "source", "claude-leaderboard_export")

// REQUIRED: Periodic progress (every 10K records or 30 seconds)
logger.LogProgress("Processing batch",
    "processed", processedCount,
    "skipped", skippedCount,
    "ingested", ingestedCount,
    "failed", failedCount,
    "rate_per_sec", currentRate,
    "eta_minutes", estimatedTimeRemaining.Minutes())

// REQUIRED: Categorized skip reasons
logger.LogDebug("Record skipped",
    "reason", "duplicate_email_login",
    "email", record.Email,
    "login", record.Login)

// REQUIRED: Operation completion summary
logger.LogInfo("Batch operation completed",
    "duration_total_ms", totalDuration.Milliseconds(),
    "processed_total", processedCount,
    "ingested_total", ingestedCount,
    "skipped_total", skippedCount,
    "failed_total", failedCount,
    "final_rate_per_sec", overallRate)
```

**Implementation Location:** Add to existing `pkg/ingestlog/logger.go` - already has PeriodicReporter infrastructure

---

### Gap 3: Service Health and Connection Monitoring

**Priority:** HIGH - Critical for production reliability  
**Impact:** Cannot detect database connection issues or service degradation  
**Production Requirement:** Must monitor database connections and external service health

#### Missing Information:

1. **Database Connection Pool Status**
   - **What's missing:** No logging of connection pool utilization
   - **Why it matters:** Connection exhaustion is a common failure mode
   - **Production impact:** Operations fail silently when connections run out

2. **External API Health**
   - **What's missing:** No logging of GitHub API response times or error rates
   - **Why it matters:** API rate limits or slowdowns need detection
   - **Production impact:** Cannot distinguish between client errors and API problems

3. **Background Worker Status**
   - **What's missing:** No heartbeat or status logging from long-running workers
   - **Why it matters:** Workers can hang without detection
   - **Production impact:** Jobs appear "running" but are actually deadlocked

#### Specification for Service Health Logging:

```go
// REQUIRED: Connection pool metrics (every 60 seconds)
logger.LogDebug("Database connection pool status",
    "open_connections", pool.Stat().OpenConnections,
    "in_use", pool.Stat().InUse,
    "idle", pool.Stat().Idle,
    "wait_count", pool.Stat().WaitCount,
    "wait_duration_ms", pool.Stat().WaitDuration.Milliseconds(),
    "max_idle_closed", pool.Stat().MaxIdleClosed,
    "max_lifetime_closed", pool.Stat().MaxLifetimeClosed)

// REQUIRED: API endpoint health
logger.LogInfo("API health check",
    "endpoint", "github_api",
    "response_time_ms", responseTime.Milliseconds(),
    "status_code", statusCode,
    "rate_limit_remaining", remaining,
    "rate_limit_reset", resetTime)

// REQUIRED: Worker heartbeat
logger.LogInfo("Worker heartbeat",
    "worker_type", "clone-worker",
    "worker_id", workerID,
    "repos_processed", reposProcessed,
    "uptime_seconds", uptime.Seconds(),
    "memory_usage_mb", currentMemMB,
    "goroutines", runtime.NumGoroutine())
```

**Implementation Location:** Create `pkg/service/health.go` with periodic health reporters

---

## Important Gaps (SHOULD IMPLEMENT - Operational Excellence)

### Gap 4: Performance Metrics and Resource Monitoring

**Priority:** HIGH - Necessary for capacity planning  
**Impact:** Cannot detect memory leaks or performance regression  
**Production Requirement:** Must track resource usage for 349K record operations

#### Missing Information:

1. **Memory Usage Tracking**
   - **What's missing:** No logging of memory consumption during operations
   - **Why it matters:** Memory leaks are invisible until OOM kills the process
   - **Production impact:** Silent failures with no diagnostic data

2. **Operation Latency Histograms**
   - **What's missing:** No distribution data for operation timings
   - **Why it matters:** Need to detect percentile regression (p95, p99)
   - **Production impact:** Cannot tune timeouts or detect degradation

3. **Throughput Metrics**
   - **What's missing:** No records/second or operations/second tracking
   - **Why it matters:** Cannot benchmark performance improvements
   - **Production impact:** Cannot validate performance optimizations

#### Specification for Performance Logging:

```go
// REQUIRED: Memory usage (every 30 seconds during large operations)
var m runtime.MemStats
runtime.ReadMemStats(&m)
logger.LogDebug("Memory usage",
    "alloc_mb", m.Alloc / 1024 / 1024,
    "total_alloc_mb", m.TotalAlloc / 1024 / 1024,
    "sys_mb", m.Sys / 1024 / 1024,
    "heap_alloc_mb", m.HeapAlloc / 1024 / 1024,
    "heap_inuse_mb", m.HeapInuse / 1024 / 1024,
    "gc_pause_ns", m.PauseTotalNs)

// REQUIRED: Operation latency with percentiles
logger.LogInfo("Operation latency",
    "operation", "warmstart_extract",
    "count_ms", latencies.Count,
    "mean_ms", latencies.Mean,
    "p50_ms", latencies.Percentile(50),
    "p95_ms", latencies.Percentile(95),
    "p99_ms", latencies.Percentile(99),
    "max_ms", latencies.Max)
```

**Implementation Location:** Add to `pkg/service/metrics.go` with periodic collection

---

### Gap 5: Error Classification and Recovery Guidance

**Priority:** MEDIUM - Improves operational efficiency  
**Impact:** Errors exist but lack actionable guidance  
**Production Requirement:** Need clear recovery paths for common failures

#### Missing Information:

1. **Error Type Classification**
   - **What's missing:** Errors returned but not classified (timeout? network? validation?)
   - **Why it matters:** Different error types need different responses
   - **Production impact:** All errors treated equally, delaying correct response

2. **Recovery Suggestions**
   - **What's missing:** No automated guidance for error recovery
   - **Why it matters:** Operators waste time researching solutions
   - **Production impact:** Longer MTTR (mean time to recovery)

#### Specification for Error Classification:

```go
// REQUIRED: Error classification (already in ingestlog - extend to warmstart)
logger.LogErrorWithRecovery("Operation failed",
    "error_type", "network_timeout",
    "error_message", err.Error(),
    "recovery_suggestion", "Check network connectivity to GitHub API",
    "recovery_command", "curl -I https://api.github.com",
    "severity", "warning",
    "retry_safe", true)

// REQUIRED: Validation error with context
logger.LogErrorWithRecovery("Validation failed",
    "error_type", "missing_ref_file",
    "repo_url", repoURL,
    "missing_files", missingRefs,
    "recovery_suggestion", "Re-clone repository without partial clone filter",
    "severity", "error",
    "retry_safe", false)
```

**Implementation Location:** Extend existing `pkg/ingestlog/logger.go` patterns to `pkg/warmstart/logger.go`

---

## Nice-to-Have Gaps (CAN IMPLEMENT - Enhanced Observability)

### Gap 6: Cache Performance Metrics

**Priority:** LOW - Optimization opportunity  
**Impact:** Cannot measure cache effectiveness

#### Missing Information:
- Cache hit/miss ratios
- Cache eviction rates
- Cache memory utilization

**Why it's nice-to-have:** With 349K records, cache effectiveness matters for performance, but doesn't block operations.

---

### Gap 7: Request Tracing and Correlation

**Priority:** LOW - Debugging enhancement  
**Impact:** Cannot trace requests across service boundaries

#### Missing Information:
- Request IDs that span multiple operations
- Trace IDs for distributed tracing
- Correlation of logs across services

**Why it's nice-to-have:** Useful for complex debugging, but single-service operations don't require it.

---

## Implementation Priority Order

### Phase 1: Production Blockers (MUST implement before production)

| Priority | Gap | Est. Effort | Production Impact |
|----------|-----|-------------|-------------------|
| 1 | Gap 1: warmstart package logging | 2-3 days | UNBLOCKS production deployment |
| 2 | Gap 2: Batch progress tracking | 1-2 days | ENABLES monitoring of 349K record operations |
| 3 | Gap 3: Service health monitoring | 2-3 days | PREVENTS silent service failures |

**Phase 1 Total: 5-8 days**

### Phase 2: Operational Excellence (SHOULD implement for production readiness)

| Priority | Gap | Est. Effort | Operational Value |
|----------|-----|-------------|-------------------|
| 4 | Gap 4: Performance metrics | 1-2 days | ENABLES capacity planning |
| 5 | Gap 5: Error classification | 1 day | REDUCES MTTR |

**Phase 2 Total: 2-3 days**

### Phase 3: Enhanced Observability (CAN implement for optimization)

| Priority | Gap | Est. Effort | Optimization Value |
|----------|-----|-------------|-------------------|
| 6 | Gap 6: Cache metrics | 1 day | Tunes performance |
| 7 | Gap 7: Request tracing | 2-3 days | Advanced debugging |

**Phase 3 Total: 3-4 days**

---

## Quick Reference: What Each Gap Enables

| Gap | Enables Production To... | Current State |
|-----|--------------------------|---------------|
| warmstart logging | Debug tarball extraction failures | **COMPLETELY BLIND** - zero visibility |
| Batch progress | Monitor 349K record operations | **NO CLUE** if operation is progressing or hung |
| Service health | Detect connection issues | **SILENT FAILURES** until operation crashes |
| Performance metrics | Plan capacity and detect leaks | **NO DATA** for sizing or regression detection |
| Error classification | Faster incident response | **MANUAL RESEARCH** for every error |
| Cache metrics | Optimize performance | **NO VISIBILITY** into cache effectiveness |
| Request tracing | Debug complex flows | **NO CORRELATION** across operations |

---

## Implementation Notes

### Reuse Existing Patterns

The `ingestlog` package already has excellent logging infrastructure:
- `PeriodicReporter` for background status updates
- `LogStats()` for aggregate statistics
- `LogBatchProgress()` for real-time progress
- `LogErrorWithRecovery()` for classified errors with guidance

**DO NOT reinvent** - reuse these patterns for warmstart and service logging.

### Log Format Consistency

All new logging MUST follow existing patterns:
- Structured JSON output for machine consumption
- Human-readable summaries for operators
- Consistent field names across all packages
- Timestamp in ISO 8601 format (UTC)

### Performance Considerations

With 349,425 records:
- **DEBUG level:** File-by-file progress (disableable in production)
- **INFO level:** Operation milestones and summaries (always on)
- **ERROR level:** All failures (always on)

**DO NOT** log at INFO level for every record - use PeriodicReporter for batch updates.

---

## Acceptance Criteria Checklist

Each gap implementation MUST:

- [ ] Follow existing `ingestlog` patterns and field names
- [ ] Provide structured JSON output + human-readable summaries
- [ ] Include clear "why this matters" rationale in code comments
- [ ] Support configurable log levels (DEBUG/INFO/ERROR)
- [ ] Handle logger failures gracefully (fallback to stderr)
- [ ] Include unit tests for logging behavior
- [ ] Document example log output in this specification

---

## Next Steps

1. **Create child bead for Gap 1** (warmstart package logging)
2. **Create child bead for Gap 2** (batch progress tracking)
3. **Create child bead for Gap 3** (service health monitoring)
4. **Implement in priority order** (Phase 1 → Phase 2 → Phase 3)
5. **Update this specification** with example log output from implementation

---

## Appendix: Production Operation Examples

### Example 1: 349K Record Seed Operation WITHOUT Logging (Current State)

```
[14:00:00] Starting seed operation...
[18:00:00] Still running... (no progress visible)
[22:00:00] Still running... (is it working? hung? failed?)
[02:00:00] Operation completed (but how many records actually made it?)
```

**Problem:** 12 hours of blind operation with zero visibility.

### Example 2: 349K Record Seed Operation WITH Logging (This Specification)

```
[14:00:00] INFO: Batch operation started, total_records=349425, source=claude-leaderboard_export
[14:00:30] INFO: Processing batch, processed=10000, skipped=50, ingested=9950, failed=0, rate_per_sec=333.3, eta_minutes=175
[14:01:00] INFO: Processing batch, processed=20000, skipped=102, ingested=19898, failed=0, rate_per_sec=340.1, eta_minutes=171
[14:01:30] INFO: Processing batch, processed=30000, skipped=158, ingested=29842, failed=0, rate_per_sec=345.2, eta_minutes=168
...
[18:15:45] INFO: Processing batch, processed=349425, skipped=1842, ingested=347583, failed=0, rate_per_sec=318.7, eta_minutes=0
[18:15:45] INFO: Batch operation completed, duration_total_ms=15455000, processed_total=349425, ingested_total=347583, skipped_total=1842, failed_total=0, final_rate_per_sec=22.6
```

**Solution:** Full visibility with progress tracking, ETA, and completion verification.

---

**Document Status:** COMPLETE  
**Ready for Implementation:** YES  
**Next Action:** Create child beads for Phase 1 gaps (warmstart, batch progress, service health)
