# Retired Epoch Test Fixtures

This directory contains test fixture data for testing retired epoch functionality in the commitgraph system.

## Purpose

These fixtures are designed to test the critical requirement that systems handling encryption epochs must NOT silently skip older partitions still sitting on retired epochs. The preflight check tool must properly identify and handle retired encryption keys.

## Retired Epoch Keys

The following retired epoch keys are available for testing:

### Primary Retired Epoch Key
- **key_id**: `epoch-2023-12-retired`
- **epoch**: `2023-12`
- **status**: `retired`
- **created_at**: `2023-12-01T00:00:00Z`
- **retired_at**: `2024-08-01T00:00:00Z`
- **description**: Retired encryption key for December 2023 epoch
- **manifest**: `manifest-retired-epoch.json`

### Ancient Retired Epoch Key
- **key_id**: `epoch-2022-06-ancient`
- **epoch**: `2022-06`
- **status**: `retired`
- **created_at**: `2022-06-01T00:00:00Z`
- **retired_at**: `2023-12-01T00:00:00Z`
- **description**: Ancient retired encryption key for June 2022 epoch
- **manifest**: `manifest-multi-epoch.json`

## Current Epoch Keys

### Current Epoch Key
- **key_id**: `epoch-2024-08-current`
- **epoch**: `2024-08`
- **status**: `current`
- **created_at**: `2024-08-01T00:00:00Z`
- **description**: Current encryption key for August 2024 epoch
- **manifest**: `manifest-current-epoch.json`

## Manifest Files

### manifest-retired-epoch.json
Contains a single retired epoch manifest with key_id `epoch-2023-12-retired`. Use this for testing basic retired epoch detection.

### manifest-current-epoch.json
Contains a single current epoch manifest with key_id `epoch-2024-08-current`. Use this for baseline comparison and current epoch testing.

### manifest-multi-epoch.json
Contains three encryption epochs:
- `epoch-2022-06-ancient` (retired)
- `epoch-2023-12-retired` (retired)  
- `epoch-2024-08-current` (current)

Use this for testing scenarios with multiple mixed epochs.

## Usage Examples

### Loading a Retired Epoch Manifest
```json
import json

with open('testdata/fixtures/retired-epoch/manifest-retired-epoch.json', 'r') as f:
    manifest = json.load(f)
    
# Access retired epoch key
retired_key = manifest['encryption_keys'][0]
print(f"Retired key_id: {retired_key['key_id']}")
print(f"Status: {retired_key['status']}")
```

### Testing Multi-EPOCH Scenarios
```json
import json

with open('testdata/fixtures/retired-epoch/manifest-multi-epoch.json', 'r') as f:
    manifest = json.load(f)
    
# Count retired vs current epochs
retired_count = sum(1 for k in manifest['encryption_keys'] if k['status'] == 'retired')
current_count = sum(1 for k in manifest['encryption_keys'] if k['status'] == 'current')

print(f"Retired epochs: {retired_count}")
print(f"Current epochs: {current_count}")
```

## Test Coverage

These fixtures support testing for:
1. **Retired epoch detection**: Identifying partitions using retired encryption keys
2. **Multi-epoch handling**: Processing multiple partitions with different encryption epochs
3. **Preflight validation**: Ensuring systems properly report retired epoch usage
4. **Migration compatibility**: Verifying that retired epoch data can still be accessed and migrated

## Fixture Structure

```
testdata/fixtures/retired-epoch/
├── README.md                          # This documentation
├── manifest-retired-epoch.json       # Single retired epoch fixture
├── manifest-current-epoch.json      # Single current epoch fixture  
└── manifest-multi-epoch.json        # Mixed epoch fixture (2 retired, 1 current)
```

## Key ID Format

Epoch keys follow the format: `epoch-YYYY-MM-status`

- `YYYY`: 4-digit year
- `MM`: 2-digit month
- `status`: `current`, `retired`, or `ancient`

## Important Notes

- The retired epoch key_id `epoch-2023-12-retired` is the primary test fixture for retired epoch testing
- All timestamps are in UTC (Z suffix)
- Manifest files use standard JSON format for easy parsing in tests
- Fixtures are designed to be minimal but realistic representations of production data