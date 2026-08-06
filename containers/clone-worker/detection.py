"""Detection catalog for AI coding tool attribution.

This module provides the detection patterns and logic for identifying which
AI coding tools produced a commit. It supports multi-tool commits and checks
four signal tiers:
1. Co-Authored-By trailer emails
2. Author emails (bot-authored commits)
3. Author name patterns
4. Body text patterns

The catalog covers 15+ tools across the four signal tiers.
"""

from __future__ import annotations

import re
from typing import Dict, List, Set, Optional


# ── Detection Catalog (15+ tools across 4 signal tiers) ─────────────────────────────

# Signal Tier 1: Co-Authored-By Trailer Emails
# These patterns match emails in Co-Authored-By trailers, which indicate
# that an AI tool assisted with the commit.
TRAILER_EMAILS: Dict[str, Set[str]] = {
    "claude-code": {"noreply@anthropic.com"},
    "cursor": {"cursoragent@cursor.com"},
    "aider": {"noreply@aider.chat"},
    "continue": {"noreply@continue.dev"},
    "codex": {"noreply@openai.com"},
    "devin": {"noreply@cognition.ai"},
    "windsurf": {"windsurf@codeium.com"},
    "codeium": {"codeium@codeium.com", "noreply@codeium.com"},
    "replit": {"noreply@replit.com", "ghostwriter@replit.com"},
    "cody": {"cody@sourcegraph.com", "noreply@sourcegraph.com"},
    "blackbox": {"noreply@blackbox.ai"},
    "tabnine": {"noreply@tabnine.com"},
    "codestral": {"noreply@codestral.mistral.ai"},
}

# Signal Tier 2a: Author Emails (bot-authored commits)
# These patterns match the git author email when the commit was authored
# directly by an AI bot.
AUTHOR_EMAILS: Dict[str, Set[str]] = {
    "openhands": {"openhands@all-hands.dev"},
    "cubic": {"contact@cubic.dev"},
    "replit-bot": {"noreply@replit.com"},
    "codeium-bot": {"bot@codeium.com"},
}

# Signal Tier 2b: Author Name Patterns
# These patterns match the git author name field for AI bots that use
# specific naming conventions.
AUTHOR_NAME_PATTERNS: Dict[str, List[re.Pattern]] = {
    "claude-code": [re.compile(r"claude\[bot\]", re.I)],
    "copilot": [
        re.compile(r"^copilot(\[bot\])?$", re.I),
        re.compile(r"^copilot-swe-agent\[bot\]$", re.I),
    ],
    "continue": [
        re.compile(r"^continue\[bot\]$", re.I),
        re.compile(r"^Continue Agent$"),
    ],
    "devin": [re.compile(r"^devin-ai-integration\[bot\]$", re.I)],
    "jules": [re.compile(r"^google-labs-jules\[bot\]$", re.I)],
    "cubic": [re.compile(r"^cubic-dev-ai\[bot\]$", re.I)],
    # Sweep AI (2023) — a GitHub App, so its author name is the deterministic
    # `<app-slug>[bot]` form, same as devin/jules/cubic above. Added from the
    # 2023-2024 coverage audit (bf-612rb); anchored, so a wrong guess can only
    # fail to match, never mis-attribute.
    "sweep": [re.compile(r"^sweep-ai\[bot\]$", re.I)],
    "netlify-coding": [re.compile(r"^netlify-coding\[bot\]$", re.I)],
    "aider": [re.compile(r"\(aider\)", re.I)],
    "codeium": [re.compile(r"^codeium\[bot\]$", re.I)],
    "cody": [
        re.compile(r"^cody\[bot\]$", re.I),
        re.compile(r"^sourcegraph-cody\[bot\]$", re.I),
    ],
    "blackbox": [re.compile(r"^blackbox\[bot\]$", re.I)],
    "tabnine": [re.compile(r"^tabnine\[bot\]$", re.I)],
    "replit": [
        re.compile(r"^replit\[bot\]$", re.I),
        re.compile(r"^ghostwriter", re.I),
    ],
    "codestral": [re.compile(r"^codestral", re.I)],
}

# Signal Tier 3: Body Text / Custom Trailer Patterns
# These patterns match specific text in commit message bodies or custom trailers.
BODY_PATTERNS: Dict[str, re.Pattern] = {
    "claude-code": re.compile(r"Generated with Claude Code", re.I),
    "cursor": re.compile(r"Made-with:\s*Cursor", re.I),
    "aider": re.compile(r"Generated\s+by\s+aider", re.I),
    "continue": re.compile(r"Generated\s+with\s+Continue", re.I),
    "codex": re.compile(r"Generated\s+by\s+Codex", re.I),
    "codeium": re.compile(r"Made\s+with\s+Codeium|Generated\s+by\s+Codeium", re.I),
    "cody": re.compile(r"Generated\s+by\s+Cody|Sourcegraph\s+Cody", re.I),
    "blackbox": re.compile(r"Generated\s+by\s+Blackbox", re.I),
    "tabnine": re.compile(r"Generated\s+by\s+TabNine|TabNine\s+Assistant", re.I),
}

# Compile all unique tool names for reference
ALL_TOOLS: Set[str] = (
    set(TRAILER_EMAILS) | set(AUTHOR_EMAILS) | set(AUTHOR_NAME_PATTERNS) | set(BODY_PATTERNS)
)


def detect_tools(
    author_email: str,
    author_name: str,
    coauthor_trailer: str,
    commit_message: str,
) -> Set[str]:
    """Detect which AI coding tools produced a commit.

    Returns a set of matched tool names (supports multi-tool commits).
    Checks all 4 signal tiers:
    1. Co-Authored-By trailer emails
    2. Author emails (bot-authored)
    3. Author name patterns
    4. Body text patterns

    Args:
        author_email: Git author email
        author_name: Git author name
        coauthor_trailer: Co-Authored-By trailer value (extracted by git log)
        commit_message: Full commit message (subject + body)

    Returns:
        Set of tool names (empty if no matches)
    """
    detected: Set[str] = set()

    # Signal Tier 1: Co-Authored-By trailer emails
    if coauthor_trailer:
        trailer_email_lower = coauthor_trailer.lower()
        for tool, emails in TRAILER_EMAILS.items():
            if any(trailer_email_lower == e.lower() for e in emails):
                detected.add(tool)

    # Signal Tier 2a: Author emails
    author_email_lower = author_email.lower()
    for tool, emails in AUTHOR_EMAILS.items():
        if any(author_email_lower == e.lower() for e in emails):
            detected.add(tool)

    # Signal Tier 2b: Author name patterns
    for tool, patterns in AUTHOR_NAME_PATTERNS.items():
        if any(pattern.search(author_name) for pattern in patterns):
            detected.add(tool)

    # Signal Tier 3: Body text patterns
    message_lower = commit_message.lower()
    for tool, pattern in BODY_PATTERNS.items():
        if pattern.search(message_lower):
            detected.add(tool)

    return detected


def get_tools_for_signal_tier(tier: int) -> Set[str]:
    """Get all tools that have detection patterns for a specific signal tier.

    Args:
        tier: Signal tier number (1-3)

    Returns:
        Set of tool names with patterns for that tier
    """
    if tier == 1:
        return set(TRAILER_EMAILS.keys())
    elif tier == 2:
        return set(AUTHOR_EMAILS.keys()) | set(AUTHOR_NAME_PATTERNS.keys())
    elif tier == 3:
        return set(BODY_PATTERNS.keys())
    else:
        return set()


def get_pattern_count_for_tool(tool: str) -> Dict[str, int]:
    """Get the number of detection patterns across all signal tiers for a tool.

    Args:
        tool: Tool name

    Returns:
        Dict with counts per signal tier (1-3)
    """
    counts: Dict[str, int] = {}

    # Tier 1: Trailer emails
    counts["trailer_emails"] = len(TRAILER_EMAILS.get(tool, set()))

    # Tier 2a: Author emails
    counts["author_emails"] = len(AUTHOR_EMAILS.get(tool, set()))

    # Tier 2b: Author name patterns
    counts["author_name_patterns"] = len(AUTHOR_NAME_PATTERNS.get(tool, []))

    # Tier 3: Body patterns
    counts["body_patterns"] = 1 if tool in BODY_PATTERNS else 0

    return counts


# Matches "Co-Authored-By: Name <email>" trailer lines (case-insensitive,
# leading whitespace tolerated). Group 1 is the email inside the angle
# brackets — the value detect_tools' Tier 1 compares against TRAILER_EMAILS.
_COAUTHOR_TRAILER_RE = re.compile(r"^\s*co-authored-by:.*?<([^<>]+)>", re.IGNORECASE | re.MULTILINE)


def extract_coauthor_emails(commit_message: str) -> List[str]:
    """Extract every Co-Authored-By trailer email from a commit message.

    A commit can carry multiple trailers (pair programming, multi-tool
    commits); each is a separate Tier-1 signal.
    """
    if not commit_message:
        return []
    return [m.strip() for m in _COAUTHOR_TRAILER_RE.findall(commit_message)]


# Domains that carry ordinary human co-authors. A Co-Authored-By trailer on one
# of these is a person, not a tool, so it is not a catalog-gap candidate.
# Deliberately small: the point of unmatched_signals is to OVER-report, and a
# human domain that slips through costs one log line.
_HUMAN_TRAILER_DOMAINS: Set[str] = {
    "users.noreply.github.com",
    "gmail.com",
    "googlemail.com",
    "outlook.com",
    "hotmail.com",
    "yahoo.com",
    "icloud.com",
    "me.com",
    "protonmail.com",
    "proton.me",
    "qq.com",
    "163.com",
    "126.com",
}

# Known NON-AI automation. These dominate any bot-shaped scan by orders of
# magnitude — the first live pass reported 17,268 unclaimed signals for a
# single 2021 partition, of which the entire top ten was dependency bots, CI,
# and merge queues. Reporting them buries the thing the scan exists to find,
# so they are excluded to leave a usable signal.
#
# Membership means "this bot does not author code with an AI agent", which is
# a claim that can expire: remove an entry the moment a bot gains AI
# authorship (several dependency bots have shipped AI features). Names are
# matched exactly, so a new AI bot can never be silently swallowed by a
# too-broad rule.
_NON_AI_AUTOMATION_NAMES: Set[str] = {
    # dependency / version bumps
    "dependabot[bot]",
    "dependabot-preview[bot]",
    "renovate[bot]",
    "renovate-bot",
    "greenkeeper[bot]",
    "scala-steward",
    "pyup-bot",
    "dotnet-maestro[bot]",
    "snyk-bot",
    "whitesource-bolt-for-github[bot]",
    # CI / merge automation
    "github-actions[bot]",
    "bors[bot]",
    "mergify[bot]",
    "kodiakhq[bot]",
    "azure-pipelines[bot]",
    "pre-commit-ci[bot]",
    "restyled-io[bot]",
    "stale[bot]",
    "semantic-release-bot",
    "release-please[bot]",
    "changeset-bot[bot]",
    "codecov[bot]",
    "coveralls",
    "sonarcloud[bot]",
    "netlify[bot]",
    "vercel[bot]",
    "delete-merged-branch[bot]",
    # codegen / localization / assets
    "api-clients-generation-pipeline[bot]",
    "imgbot[bot]",
    "allcontributors[bot]",
    "weblate",
    "crowdin-bot",
    "transifex-integration[bot]",
}

# Automation trailer domains, same rationale as the names above.
_NON_AI_AUTOMATION_DOMAINS: Set[str] = {
    "renovateapp.com",
    "datadoghq.com",
    "dependabot.com",
}

# A Co-Authored-By trailer is far more often a human colleague than a tool, and
# humans do NOT mostly use consumer mail: the second live pass surfaced
# brad.jorsch@automattic.com, juzhiyuan@apache.org, 6543@obermui.de and
# slankes@eonerc.rwth-aachen.de in the top ten — corporate, project and
# university addresses that no domain denylist can enumerate.
#
# What separates a tool is that its address is IMPERSONAL. Every AI trailer in
# the catalog is noreply@/​<tool>agent@/bot@ rather than a person's name, so the
# trailer channel only reports local parts carrying such a marker (or a .ai
# domain, which is overwhelmingly vendor-owned).
#
# Recall limit, stated rather than assumed: a tool whose address is bare
# <toolname>@<vendor> with no marker — cody@sourcegraph.com,
# windsurf@codeium.com, ghostwriter@replit.com — is lexically identical to a
# person and is NOT reported here. Those surface through the author-name
# channel if the tool also commits as a bot, or must be added deliberately.
_IMPERSONAL_LOCAL_RE = re.compile(r"(noreply|no-reply|bot|agent|automation)", re.I)

# A .ai-domain rule was tried and REMOVED on evidence. The prediction that it
# would report humans was registered in a test before deployment; a full pass
# then produced exactly one .ai trailer — thomas@grid.ai, an engineer at an AI
# company — across every partition, three times, and no tools at all. A rule
# whose entire observed output is a false positive is worse than the recall it
# buys, so the trailer channel now keys solely on an impersonal local part.

_BOT_NAME_RE = re.compile(r"\[bot\]\s*$", re.I)
# Attacker-controlled text (a repo can put anything in a trailer) reaches a log
# line from here, so cap length and drop control characters — the same
# untrusted-input policy the serializer applies to web output.
_SIGNAL_MAX_LEN = 120
_CONTROL_CHARS_RE = re.compile(r"[\x00-\x1f\x7f]")


def _sanitize_signal(value: str) -> str:
    return _CONTROL_CHARS_RE.sub("", value)[:_SIGNAL_MAX_LEN]


def unmatched_signals(
    author_email: str,
    author_name: str,
    commit_message: str,
    detected: Set[str] | None = None,
) -> List[str]:
    """Bot-shaped signals on a commit that the catalog did NOT match.

    The catalog is written from recollection of tool conventions, which makes
    it structurally weakest on the past: most patterns are bot accounts and
    ``Generated with ...`` trailers, conventions largely adopted AFTER the
    tools themselves shipped. That produced a real false conclusion — every
    2021-2023 partition detects zero rows, which reads as "no AI commits
    existed" when it is equally consistent with "the catalog does not encode
    2023's conventions" (bf-612rb).

    This turns that blind spot into data. Anything shaped like a tool —
    a Co-Authored-By trailer on a non-human domain, or a ``…[bot]`` author
    name — that no tool claimed is reported, so the next catalog addition
    comes from what the corpus actually contains rather than from what
    someone remembered.

    Known non-AI automation is excluded. Without that filter the output is
    unusable: the first live pass reported 17,268 signals for one 2021
    partition and the entire top ten was dependency bots, CI and merge
    queues, which would bury a real agent appearing a handful of times.

    Returns sanitized, deduplicated candidates.
    """
    if detected is None:
        detected = detect_tools_for_commit(author_email, author_name, commit_message)
    if detected:
        return []  # already attributed; not a gap

    out: List[str] = []
    for email in extract_coauthor_emails(commit_message):
        domain = email.rpartition("@")[2].lower()
        if not domain or domain in _HUMAN_TRAILER_DOMAINS:
            continue
        if domain in _NON_AI_AUTOMATION_DOMAINS:
            continue
        local = email.rpartition("@")[0].lower()
        # Known non-AI automation reaches the trailer channel too
        # (snyk-bot@snyk.io appeared 36x in one partition). The name denylist
        # is the same decision, so apply it on both channels rather than
        # letting an already-excluded bot back in through a different field.
        if local in _NON_AI_AUTOMATION_NAMES or f"{local}[bot]" in _NON_AI_AUTOMATION_NAMES:
            continue
        if not _IMPERSONAL_LOCAL_RE.search(local):
            continue  # a person's address, not a tool's
        out.append(f"trailer:{_sanitize_signal(email.lower())}")
    if author_name and _BOT_NAME_RE.search(author_name):
        name = author_name.strip().lower()
        if name not in _NON_AI_AUTOMATION_NAMES:
            out.append(f"author:{_sanitize_signal(name)}")
    return sorted(set(out))


def detect_tools_for_commit(
    author_email: str,
    author_name: str,
    commit_message: str,
) -> Set[str]:
    """Detect tools for a commit whose trailers live inside the message.

    The staged Commit Parquet schema carries the full message (subject +
    body + trailers) but no pre-extracted Co-Authored-By value, so this is
    the entry point the filter-worker uses: it extracts every trailer email
    and unions ``detect_tools`` across them (plus one pass for the tiers
    that don't depend on the trailer).
    """
    trailers = extract_coauthor_emails(commit_message) or [""]
    detected: Set[str] = set()
    for trailer in trailers:
        detected |= detect_tools(
            author_email=author_email or "",
            author_name=author_name or "",
            coauthor_trailer=trailer,
            commit_message=commit_message or "",
        )
    return detected


__all__ = [
    "TRAILER_EMAILS",
    "AUTHOR_EMAILS",
    "AUTHOR_NAME_PATTERNS",
    "BODY_PATTERNS",
    "ALL_TOOLS",
    "detect_tools",
    "detect_tools_for_commit",
    "extract_coauthor_emails",
    "unmatched_signals",
    "get_tools_for_signal_tier",
    "get_pattern_count_for_tool",
]
__version__ = "1.2.0"
