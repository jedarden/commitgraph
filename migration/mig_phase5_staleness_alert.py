#!/usr/bin/env python3
"""
Phase 5 Public Leaderboard Staleness Alert

Monitors the age of the frozen public leaderboard.json and alerts when
approaching or exceeding the maximum staleness threshold.

Reference decision: docs/notes/cg-1tkq-phase5-staleness-threshold.md
Golden snapshot generation time: 2026-08-03T22:05:42Z
Maximum staleness: 30 days (hard deadline 2026-09-02T22:05:42Z)
Review checkpoint: 14 days (2026-08-17T22:05:42Z)
"""

import sys
from datetime import datetime, timezone
from pathlib import Path
import argparse
import logging

# Configure logging
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)

# Decision constants from docs/notes/cg-1tkq-phase5-staleness-threshold.md
GOLDEN_SNAPSHOT_TIME = "2026-08-03T22:05:42Z"
MAX_STALENESS_DAYS = 30
REVIEW_CHECKPOINT_DAYS = 14

# Parse the golden snapshot time
GOLDEN_SNAPSHOT_DT = datetime.fromisoformat(
    GOLDEN_SNAPSHOT_TIME.replace('Z', '+00:00')
)

def check_staleness() -> dict:
    """
    Check the current staleness of the frozen leaderboard.json.

    Returns:
        dict: Staleness status with age in days and alert level
    """
    now = datetime.now(timezone.utc)
    age_seconds = (now - GOLDEN_SNAPSHOT_DT).total_seconds()
    age_days = age_seconds / 86400  # Convert to days

    status = {
        'golden_snapshot_time': GOLDEN_SNAPSHOT_TIME,
        'current_time': now.isoformat(),
        'age_days': round(age_days, 2),
        'age_seconds': age_seconds,
        'max_staleness_days': MAX_STALENESS_DAYS,
        'review_checkpoint_days': REVIEW_CHECKPOINT_DAYS,
        'alert_level': None,
        'message': None,
        'actions': []
    }

    if age_days >= MAX_STALENESS_DAYS:
        status['alert_level'] = 'CRITICAL'
        status['message'] = (
            f"Leaderboard is {age_days:.1f} days old. "
            f"Maximum staleness ({MAX_STALENESS_DAYS} days) exceeded."
        )
        status['actions'] = [
            "IMMEDIATE: Pull frozen leaderboard.json from public serving",
            "Replace with reconstruction message showing golden snapshot date",
            "Alert operator: pipeline must publish fresh snapshot or public serving stays down",
            "Do NOT restore public serving until fresh snapshots are publishing"
        ]

    elif age_days >= REVIEW_CHECKPOINT_DAYS:
        status['alert_level'] = 'WARNING'
        status['message'] = (
            f"Leaderboard is {age_days:.1f} days old. "
            f"Review checkpoint ({REVIEW_CHECKPOINT_DAYS} days) reached."
        )
        status['actions'] = [
            "ASSESS Phase 5 progress: Has discovery restarted?",
            "ASSESS downstream presentation layer: Has it started?",
            "If both are 'no progress' → escalate to operator with two options:",
            "  Option A: Implement minimal public-serving fallback (top-100 with anti-scraping)",
            "  Option B: Pull frozen leaderboard.json and serve 'under reconstruction' message"
        ]

    else:
        status['alert_level'] = 'INFO'
        status['message'] = (
            f"Leaderboard is {age_days:.1f} days old. "
            f"Within acceptable staleness threshold."
        )
        status['actions'] = [
            f"Monitor until {REVIEW_CHECKPOINT_DAYS}-day review checkpoint",
            "Continue pipeline build normally"
        ]

    return status

def main():
    parser = argparse.ArgumentParser(
        description='Monitor frozen leaderboard staleness and alert on thresholds'
    )
    parser.add_argument(
        '--format',
        choices=['human', 'json'],
        default='human',
        help='Output format (default: human)'
    )
    parser.add_argument(
        '--exit-code-on-critical',
        action='store_true',
        help='Exit with code 1 on CRITICAL alert level (for monitoring integration)'
    )

    args = parser.parse_args()

    status = check_staleness()

    # Log the appropriate level
    if status['alert_level'] == 'CRITICAL':
        logger.error(status['message'])
    elif status['alert_level'] == 'WARNING':
        logger.warning(status['message'])
    else:
        logger.info(status['message'])

    # Output the status
    if args.format == 'json':
        import json
        print(json.dumps(status, indent=2))
    else:
        # Human-readable format
        print("\n" + "=" * 70)
        print("PHASE 5 PUBLIC LEADERBOARD STALENESS STATUS")
        print("=" * 70)
        print(f"\nGolden snapshot generated: {status['golden_snapshot_time']}")
        print(f"Current time: {status['current_time']}")
        print(f"Age: {status['age_days']} days")
        print(f"\nMaximum staleness: {status['max_staleness_days']} days")
        print(f"Review checkpoint: {status['review_checkpoint_days']} days")
        print(f"\nALERT LEVEL: {status['alert_level']}")
        print(f"\n{status['message']}")

        if status['actions']:
            print("\nRECOMMENDED ACTIONS:")
            for i, action in enumerate(status['actions'], 1):
                print(f"  {i}. {action}")

        print("\n" + "=" * 70)
        print("Reference: docs/notes/cg-1tkq-phase5-staleness-threshold.md")
        print("=" * 70 + "\n")

    # Exit with error code on critical if requested
    if args.exit_code_on_critical and status['alert_level'] == 'CRITICAL':
        sys.exit(1)

    return 0

if __name__ == '__main__':
    sys.exit(main())