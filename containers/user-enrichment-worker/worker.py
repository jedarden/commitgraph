#!/usr/bin/env python3
"""
User Enrichment Worker — Identity Layer 1: author_email → GitHub login.

Speaks queue-api 2.x's email-resolution claim contract (the durable
once-per-email-forever cache, plan.md §Identity Layer 1):

  1. POST /email-resolution/claim   — lease a batch, highest AI-commit
                                      priority first (the aggregator's Stage-1
                                      queue feed sets priorities)
  2. resolve each email:
       - {id}+{login}@users.noreply.github.com → local parse, zero API cost
       - private/local domains → provable non-match, zero API cost
       - otherwise ONE GitHub commit-search call (author-email:) — the
         expensive path the permanent cache exists to amortize
  3. POST /identity/ingest          — record login (positive) or null
                                      (negative cache; never re-attempted)

The old generation ALSO scanned devimprint's ARMOR fact-cube layout for
"ghost users" to self-feed the queue — retired: the feed is the aggregator's
Stage-1 upsert now (plan.md "Queue feed"), and neither ARMOR nor the fact
cube exists in this build.
"""

import logging
import os
import socket
import time
from typing import Optional

import requests
from requests.adapters import HTTPAdapter
from urllib3.util.retry import Retry

logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(message)s")
log = logging.getLogger("user-enrichment-worker")

QUEUE_URL = os.environ["QUEUE_API_URL"].rstrip("/")
INGEST_URL = os.environ.get("INGEST_URL", QUEUE_URL).rstrip("/")
QUEUE_API_INTERNAL_TOKEN = os.environ.get("QUEUE_API_INTERNAL_TOKEN", "")
GITHUB_TOKEN = os.environ.get("GITHUB_TOKEN", "")
WORKER_ID = os.environ.get("WORKER_ID", socket.gethostname())
PROVIDER = os.environ.get("PROVIDER", "github")
CLAIM_BATCH = int(os.environ.get("CLAIM_BATCH", "20"))
IDLE_SLEEP_SECS = int(os.environ.get("IDLE_SLEEP_SECS", "60"))
# GitHub commit search shares the ~30 req/min search budget with
# search-worker; stay well under our half of it.
API_CALL_INTERVAL_SECS = float(os.environ.get("API_CALL_INTERVAL_SECS", "6"))


def _session() -> requests.Session:
    s = requests.Session()
    retry = Retry(total=3, backoff_factor=1, status_forcelist=[500, 502, 503, 504])
    s.mount("https://", HTTPAdapter(max_retries=retry))
    s.mount("http://", HTTPAdapter(max_retries=retry))
    if QUEUE_API_INTERNAL_TOKEN:
        s.headers["Authorization"] = f"Bearer {QUEUE_API_INTERNAL_TOKEN}"
    return s


q = _session()

# ── Resolution core ──────────────────────────────────────────────────────────

_PRIVATE_TLD = {".local", ".lan", ".internal", ".home", ".localdomain"}


def _is_private_email(email: str) -> bool:
    """True for addresses that can never map to a GitHub login."""
    domain = email.split("@")[-1].lower() if "@" in email else ""
    return any(domain.endswith(t) for t in _PRIVATE_TLD)


def _extract_login_from_noreply(email: str) -> Optional[str]:
    """{id}+{login}@users.noreply.github.com → login (bot logins kept as-is)."""
    if not email.endswith("@users.noreply.github.com"):
        return None
    local = email.split("@")[0]
    if "+" in local:
        candidate = local.split("+", 1)[1]
        return candidate or None
    return local or None


class RateLimited(Exception):
    def __init__(self, retry_after: int):
        self.retry_after = retry_after


def lookup_github_login(email: str) -> tuple[Optional[str], bool]:
    """Resolve one email. Returns (login_or_None, api_called)."""
    noreply = _extract_login_from_noreply(email)
    if noreply is not None:
        return noreply, False
    if _is_private_email(email) or not GITHUB_TOKEN:
        return None, False

    r = requests.get(
        "https://api.github.com/search/commits",
        headers={
            "Authorization": f"token {GITHUB_TOKEN}",
            "Accept": "application/vnd.github.cloak-preview+json",
        },
        params={"q": f"author-email:{email}", "per_page": "1"},
        timeout=15,
    )
    if r.status_code == 200:
        items = r.json().get("items", [])
        if items and items[0].get("author"):
            return items[0]["author"].get("login"), True
        return None, True
    if r.status_code in (404, 422):
        return None, True
    if r.status_code in (403, 429):
        retry_after = int(r.headers.get("Retry-After", "300"))
        raise RateLimited(retry_after)
    log.warning("GitHub lookup for %s returned %d", email, r.status_code)
    return None, True


# ── Queue-api client (2.x email-resolution contract) ─────────────────────────


def claim_batch() -> list[dict]:
    try:
        r = q.post(
            f"{QUEUE_URL}/email-resolution/claim",
            json={"provider": PROVIDER, "worker_id": WORKER_ID, "limit": CLAIM_BATCH},
            timeout=15,
        )
        if r.status_code == 200:
            return r.json().get("claimed") or []
        log.warning("claim returned %d: %s", r.status_code, r.text[:200])
    except requests.RequestException as e:
        log.warning("claim failed: %s", e)
    return []


def record_resolution(email: str, login: Optional[str]) -> None:
    """Post resolution to the ingest endpoint with source='live' and current timestamp."""
    try:
        from datetime import datetime

        r = q.post(
            f"{INGEST_URL}/identity/ingest",
            json=[{
                "email": email,
                "github_username": login,
                "source": "live",
                "resolved_at": datetime.utcnow().isoformat() + "Z",
            }],
            timeout=15,
        )
        if r.status_code != 200:
            log.warning("ingest %s returned %d: %s", email, r.status_code, r.text[:200])
    except requests.RequestException as e:
        log.warning("ingest %s failed: %s", email, e)


# ── Main loop ────────────────────────────────────────────────────────────────


def main() -> None:
    log.info(
        "user-enrichment-worker starting id=%s queue=%s batch=%d",
        WORKER_ID, QUEUE_URL, CLAIM_BATCH,
    )
    while True:
        batch = claim_batch()
        if not batch:
            time.sleep(IDLE_SLEEP_SECS)
            continue

        log.info("claimed %d emails", len(batch))
        resolved = negative = 0
        for item in batch:
            email = (item.get("author_email") or "").strip().lower()
            if not email:
                continue
            try:
                login, api_called = lookup_github_login(email)
            except RateLimited as e:
                log.warning("rate limited; sleeping %ds (unresolved rows reclaim on lease expiry)", e.retry_after)
                time.sleep(min(e.retry_after, 600))
                continue
            except Exception as e:  # noqa: BLE001
                log.warning("lookup error for %s: %s — leaving for lease reclaim", email, e)
                continue

            record_resolution(email, login)
            if login:
                resolved += 1
            else:
                negative += 1
            if api_called:
                time.sleep(API_CALL_INTERVAL_SECS)

        log.info("batch done: %d resolved, %d negative-cached", resolved, negative)


if __name__ == "__main__":
    main()
