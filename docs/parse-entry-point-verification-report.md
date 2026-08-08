# Parse Entry Point Function Locations - Verification Report

**Generated:** 2026-08-08  
**Task:** cg-1tjlh  
**Purpose:** Verify cataloged function locations against actual codebase

## Summary

- **Total Functions:** 11
- **Verified:** 11 (100%)
- **Accurate Locations:** 9 (81.8%)
- **Discrepancies Found:** 2 (18.2%)

## Detailed Verification Results

### ✅ Accurate Locations (9 functions)

| Function | Package | Catalog Line | Actual Line | Status |
|----------|---------|--------------|-------------|---------|
| parseDate | cmd/audit-logs | 211 | 211 | ✅ Match |
| parseAliasesFromConfigMap | cmd/load-admin-aliases | 227 | 227 | ✅ Match |
| parseInsertLine | cmd/verify-email-resolution-dump | 65 | 65 | ✅ Match |
| parseDump | cmd/load-email-resolution-from-queue-api | 163 | 163 | ✅ Match |
| parseValuesString | cmd/load-email-resolution-from-queue-api | 197 | 197 | ✅ Match |
| parseTime | cmd/load-email-resolution-from-queue-api | 290 | 290 | ✅ Match |
| parseTimePtr | cmd/load-email-resolution-from-queue-api | 312 | 312 | ✅ Match |
| parseQueryParams | pkg/handler | 107 | 107 | ✅ Match |
| parseDate | pkg/handler | 174 | 174 | ✅ Match |

### ❌ Discrepancies Found (2 functions)

| Function | Package | Catalog Line | Actual Line | Difference | Status |
|----------|---------|--------------|-------------|------------|---------|
| parseConfigKey | pkg/warmstart | 441 | 465 | +24 lines | ❌ Incorrect |
| parseDate | cmd/get-audit-logs | 216 | 219 | +3 lines | ❌ Incorrect |

## Discrepancy Details

### 1. parseConfigKey (pkg/warmstart/extract.go)

**Cataloged Location:** Line 441  
**Actual Location:** Line 465  
**Offset:** +24 lines

**Function Signature:**
```go
func parseConfigKey(key string) (string, string)
```

**Explanation:** The function is located at line 465, not 441. The catalog may have been created before code refactoring or additions shifted the function location.

### 2. parseDate (cmd/get-audit-logs/main.go)

**Cataloged Location:** Line 216  
**Actual Location:** Line 219  
**Offset:** +3 lines

**Function Signature:**
```go
func parseDate(dateStr string) (time.Time, error)
```

**Explanation:** The function is located at line 219, not 216. Minor offset likely due to small code additions.

## Recommendations

1. **Update the catalog** to reflect accurate line numbers for the 2 functions with discrepancies
2. **Set up automated validation** to catch line number drift in future catalog updates
3. **Consider using more stable identifiers** (e.g., function signature + package) rather than relying solely on line numbers

## Verified Function Distribution

**By Package:**
- cmd/audit-logs: 1 function
- cmd/load-admin-aliases: 1 function  
- cmd/verify-email-resolution-dump: 1 function
- cmd/load-email-resolution-from-queue-api: 4 functions
- cmd/get-audit-logs: 1 function
- pkg/warmstart: 1 function
- pkg/handler: 2 functions

**By Function Name (including duplicates):**
- parseDate: 3 instances (different packages)
- parseTime: 1 instance
- parseTimePtr: 1 instance
- parseDump: 1 instance
- parseValuesString: 1 instance
- parseInsertLine: 1 instance
- parseAliasesFromConfigMap: 1 instance
- parseConfigKey: 1 instance
- parseQueryParams: 1 instance

## Custom Types Referenced

All custom types referenced in the catalog were verified to exist at their specified locations:
- ConfigMap (cmd/load-admin-aliases/main.go:185)
- AliasEntry (cmd/load-admin-aliases/main.go:196)
- QueueAPIRow (cmd/load-email-resolution-from-queue-api/main.go:147)
- queryParams (pkg/handler/audit_logs.go:96)

## Conclusion

The catalog is largely accurate with 81.8% of function locations verified correctly. The 2 discrepancies found are minor line number offsets that can be easily corrected. All functions have been successfully located in the codebase, and no ambiguous cases (overloads, duplicates) were found that would prevent unambiguous identification.

**Next Steps:** Update the catalog JSON file with corrected line numbers for the 2 functions with discrepancies.
