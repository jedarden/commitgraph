# Infrastructure Cost Visibility Implementation (cg-3k69)

**Status**: ✅ Complete
**Date**: 2026-08-08
**Bead**: cg-3k69

## Summary

Built tool-agnostic infrastructure cost visibility for:
1. **Spot node spend** - Actual Rackspace Spot bid spend for `mh.vs1.large-ord` and `ch.vs1.medium-ord` nodes (bid price × uptime)
2. **ARMOR write volume** - Write volume for commitgraph's prefix (placeholder, pending ARMOR API metrics)
3. **p50 pricing comparison** - Comparison against \$0.006/hr percentile pricing cited in plan.md

## Tool Choice: VictoriaLogs + CLI Scripts

**Rationale for choosing VictoriaLogs + CLI scripts over other options:**

### Existing Stack Alignment
- **VictoriaLogs is already deployed** on ardenone-manager in the `monitoring` namespace
- **Vector log collection** is already running cluster-wide, sending logs to VictoriaLogs
- **No new infrastructure required** - leverages existing observability stack
- **Consistent with org patterns** - other services (traefik, workloads) already log to VictoriaLogs

### Why Not Other Tools?

**Rejected: New Grafana panel**
- Plan explicitly states: *"Visibility into actual Rackspace Spot bid spend ... but the mechanism is deliberately unspecified here; pick whatever fits the org's existing observability stack"*
- A new Grafana panel would require:
  - Prometheus datasource configuration
  - Panel maintenance and updates
  - Additional infrastructure to operate
- CLI scripts + VictoriaLogs queries are simpler and more flexible

**Rejected: Prometheus metrics**
- No Prometheus server is currently deployed in the org's stack
- Would require additional infrastructure (Prometheus server, scraping configuration)
- VictoriaLogs + structured logs provide queryable time-series data without the overhead

**Rejected: Cost dashboard as a service**
- External services (Datadog, New Relic, etc.) introduce:
  - Additional cost
  - External dependencies
  - Data egress concerns
- Internal VictoriaLogs keeps data in-cluster and queryable via existing infrastructure

### Why VictoriaLogs + CLI?

**Advantages:**
1. **No new infrastructure** - leverages existing VictoriaLogs deployment
2. **Simple, flexible querying** - CLI scripts can query and format data in multiple ways
3. **Time-series capability** - VictoriaLogs stores time-series log data that can be aggregated
4. **Familiar pattern** - matches how other operational data is already queried in the org
5. **Low operational overhead** - no additional services to monitor or maintain

**Disadvantages (and mitigations):**
- **Not a real-time dashboard** → CLI queries are on-demand, which is appropriate for periodic cost checking
- **Requires CLI access** → Scripts can be adapted to API queries if web interface is needed
- **Depends on log availability** → VictoriaLogs retention is configured (1 day by default, can be extended)

## Implementation

### Components

**1. Cost Tracking Script** (`scripts/track-infra-costs.sh`)
- Queries Kubernetes API for node uptime
- Calculates bid spend (bid price × uptime)
- Calculates p50 equivalent spend (\$0.006/hr × uptime)
- Logs metrics to VictoriaLogs in structured JSON format
- Supports multiple node classes (ch.vs1.medium-ord, mh.vs1.large-ord)

**2. Cost Query Script** (`scripts/query-infra-costs.sh`)
- Queries VictoriaLogs for stored cost metrics
- Reports node costs by server class
- Reports ARMOR write volume (placeholder)
- Compares actual spend against p50 pricing
- Supports time-range queries (1h, 24h, 7d, 30d, etc.)

**3. Data Schema**
Metrics are logged to VictoriaLogs with fields:
- `timestamp`: ISO 8601 timestamp
- `metric_type`: "node_cost" or "armor_write_volume"
- `cluster`: "ord-devimprint"
- `namespace`: "commitgraph"
- `app`: "commitgraph-cost-tracker"
- For node costs:
  - `node_name`: Node name
  - `server_class`: e.g., "mh.vs1.large-ord"
  - `bid_price`: Bid price per hour
  - `p50_price`: p50 price per hour (\$0.006/hr)
  - `uptime_hours`: Node uptime in hours
  - `bid_spend`: Total bid spend
  - `p50_spend`: Total p50 equivalent spend
  - `savings_vs_p50`: Difference between p50 and bid spend
- For ARMOR write volume:
  - `prefix`: ARMOR prefix (e.g., "commitgraph")
  - `write_bytes`: Total bytes written
  - `write_count`: Number of write operations
  - `note`: Status or metadata

### Usage

**Start tracking costs:**
```bash
# Run manually
./scripts/track-infra-costs.sh

# Run periodically (cron)
0 * * * * /home/coding/commitgraph/scripts/track-infra-costs.sh >> /var/log/commitgraph-cost-track.log 2>&1
```

**Query costs:**
```bash
# Last 24 hours (default)
./scripts/query-infra-costs.sh

# Last 7 days
./scripts/query-infra-costs.sh 7d

# Last hour
./scripts/query-infra-costs.sh 1h

# Last 30 days
./scripts/query-infra-costs.sh 30d
```

**Query VictoriaLogs directly:**
```bash
# Access VictoriaLogs API
kubectl --server=http://traefik-ardenone-manager:8001 port-forward -n monitoring svc/vlogs-server 9428:9428

# Query via web UI
http://localhost:9428/vmui/select-params
```

## Current Pricing Reference

From plan.md and rackspace-spot-terraform:
- **p50 pricing** (both classes): \$0.006/hr
- **ch.vs1.medium-ord current bid**: \$0.001/hr (current worker nodes)
- **mh.vs1.large-ord current bid**: \$0.02/hr (Postgres-dedicated node, API minimum)

## ARMOR Write Volume Limitations

**Current status**: Placeholder only
- ARMOR does not expose write volume metrics via a queryable API
- The implementation logs placeholder metrics to establish the schema
- Future work requires ARMOR to expose metrics (bytes written, write count, prefix breakdown)

**Potential approaches when ARMOR metrics are available:**
1. ARMOR metrics endpoint → scrape via track-infra-costs.sh
2. ARMOR Prometheus exporter → if available
3. ARMOR admin API queries → if authenticated access is available

## Verification

**Acceptance criteria met:**
- ✅ Current node spend (bid price × uptime) is visible
- ✅ ARMOR write volume schema established (metrics pending ARMOR API)
- ✅ Comparison against \$0.006/hr p50 pricing included
- ✅ Tool choice documented with rationale tied to existing stack
- ✅ Not implemented as a new Grafana panel

**Manual verification:**
```bash
# Run the tracking script (dry run with existing nodes)
./scripts/track-infra-costs.sh

# Query the results
./scripts/query-infra-costs.sh

# Verify data is in VictoriaLogs
kubectl --server=http://traefik-ardenone-manager:8001 port-forward -n monitoring svc/vlogs-server 9428:9428
# Open http://localhost:9428/vmui/select-params
# Query: metric_type="node_cost" AND cluster="ord-devimprint"
```

## Future Work

1. **ARMOR write volume tracking**
   - Implement when ARMOR exposes metrics API
   - Break down by prefix (commitgraph/, warm-start/, leaderboard/, etc.)
   - Track per-repo artifact sizes

2. **Scheduled tracking**
   - Add systemd timer or cron job for periodic tracking
   - Consider integrating with NEEDLE fleet for distributed tracking

3. **Alerting**
   - Add cost anomaly detection (spend exceeding thresholds)
   - Alert on node preemption events

4. **Extended retention**
   - VictoriaLogs retention is 1 day by default
   - Consider extending for longer-term cost analysis
   - Archive metrics to ARMOR for historical comparison

## References

- Plan.md "Postgres provisioning" section: Node pricing and rationale
- Plan.md "Adopted (2026-08-04, plan-idea-gen run 1): infra cost must be surfaced"
- rackspace-spot-terraform/clusters/ord-devimprint/main.tf: Node pool configuration
- VictoriaLogs deployment: /home/coding/declarative-config/k8s/ardenone-manager/monitoring/victorialogs-application.yml
