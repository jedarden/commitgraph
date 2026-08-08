#!/usr/bin/bash
# track-infra-costs.sh - Log infrastructure costs to VictoriaLogs for commitgraph
#
# Logs:
# - Spot node bid spend (bid price × uptime)
# - ARMOR write volume for commitgraph prefix
# - Comparison against p50 percentile pricing ($0.006/hr)
#
# Run periodically (cron, systemd timer) to build time-series cost data.
# Query with query-infra-costs.sh for visibility.

set -euo pipefail

# Configuration
CLUSTER="${CLUSTER:-ord-devimprint}"
NAMESPACE="${NAMESPACE:-commitgraph}"
LOG_ENDPOINT="${LOG_ENDPOINT:-http://vlogs-server.monitoring.svc.cluster.local:9428/insert/elasticsearch}"
COMMITGRAPH_PREFIX="${COMMITGRAPH_PREFIX:-commitgraph}"

# p50 pricing from Rackspace Spot percentiles.json (2026-08-04)
P50_PRICING_MH="${P50_PRICING_MH:-0.006}"  # mh.vs1.large-ord
P50_PRICING_CH="${P50_PRICING_CH:-0.006}"  # ch.vs1.medium-ord

# Current bid prices (from rackspace-spot-terraform/clusters/ord-devimprint/main.tf)
BID_PRICE_CH="${BID_PRICE_CH:-0.001}"      # Current ch.vs1.medium-ord bid
BID_PRICE_MH="${BID_PRICE_MH:-0.02}"       # mh.vs1.large-ord bid (API minimum)

# Get current timestamp
TIMESTAMP=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

# Track metrics
track_node_costs() {
    local server_class="$1"
    local bid_price="$2"
    local p50_price="$3"

    # Get node uptime from Kubernetes API
    # Use read-only kubectl-proxy via Traefik
    local node_info
    node_info=$(kubectl --server=http://traefik-"${CLUSTER}":8001 get nodes \
        -l "servers.ngpc.rxt.io/class=${server_class}" \
        -o json --namespace="${NAMESPACE}" 2>/dev/null || echo '{"items":[]}')

    local node_count
    node_count=$(echo "$node_info" | jq '.items | length')

    if [ "$node_count" -eq 0 ]; then
        echo "No nodes found for ${server_class}, skipping cost tracking"
        return
    fi

    local total_hours=0
    local hourly_cost=0

    # Calculate uptime and cost for each node
    echo "$node_info" | jq -c '.items[]' | while read -r node; do
        local node_name
        node_name=$(echo "$node" | jq -r '.metadata.name')

        local creation_time
        creation_time=$(echo "$node" | jq -r '.metadata.creationTimestamp')

        # Calculate uptime in hours
        local uptime_seconds
        uptime_seconds=$(($(date -d "$creation_time" +%s 2>/dev/null || echo 0)))
        local current_seconds=$(date +%s)
        local uptime_hours=$(echo "scale=4; ($current_seconds - $uptime_seconds) / 3600" | bc)

        # Calculate costs
        local bid_spend=$(echo "scale=6; $bid_price * $uptime_hours" | bc)
        local p50_spend=$(echo "scale=6; $p50_price * $uptime_hours" | bc)

        # Log to VictoriaLogs
        log_metric "node_cost" \
            "node_name=${node_name}" \
            "server_class=${server_class}" \
            "bid_price=${bid_price}" \
            "p50_price=${p50_price}" \
            "uptime_hours=${uptime_hours}" \
            "bid_spend=${bid_spend}" \
            "p50_spend=${p50_spend}" \
            "savings_vs_p50=$(echo "scale=6; $p50_spend - $bid_spend" | bc)"

        total_hours=$(echo "scale=4; $total_hours + $uptime_hours" | bc)
        hourly_cost=$(echo "scale=6; $hourly_cost + $bid_spend" | bc)
    done

    echo "Tracked ${node_count} × ${server_class} nodes: ${total_hours} total hours, \$${hourly_cost}/hr spend"
}

# Track ARMOR write volume
track_armor_write_volume() {
    # ARMOR metrics are not directly exposed via the proxy
    # This is a placeholder for future implementation when ARMOR metrics are available
    # For now, we log a placeholder to establish the schema

    log_metric "armor_write_volume" \
        "prefix=${COMMITGRAPH_PREFIX}" \
        "write_bytes=0" \
        "write_count=0" \
        "note=Metrics not yet available via ARMOR API"

    echo "ARMOR write volume tracking: placeholder (API metrics not yet available)"
}

# Log a metric to VictoriaLogs
log_metric() {
    local metric_type="$1"
    shift

    local fields=("$@")
    local field_json="{"
    for field in "${fields[@]}"; do
        local key="${field%%=*}"
        local value="${field#*=}"
        if [ "$field_json" != "{" ]; then
            field_json+=","
        fi
        field_json+="\"${key}\":\"${value}\""
    done
    field_json+="}"

    local log_entry="{
        \"timestamp\": \"${TIMESTAMP}\",
        \"metric_type\": \"${metric_type}\",
        \"cluster\": \"${CLUSTER}\",
        \"namespace\": \"${NAMESPACE}\",
        \"app\": \"commitgraph-cost-tracker\",
        ${field_json#,}
    }"

    # Send to VictoriaLogs
    if ! curl -s -X POST "${LOG_ENDPOINT}" \
        -H "Content-Type: application/x-ndjson" \
        -H "AccountID: 0" \
        -H "ProjectID: 0" \
        --data-raw "{\"index\":{}}${log_entry}" \
        > /dev/null 2>&1; then
        echo "Warning: Failed to log metric to VictoriaLogs" >&2
    fi
}

main() {
    echo "Tracking infrastructure costs at ${TIMESTAMP}"

    # Track ch.vs1.medium-ord (current worker nodes)
    track_node_costs "ch.vs1.medium-ord" "$BID_PRICE_CH" "$P50_PRICING_CH"

    # Track mh.vs1.large-ord (Postgres-dedicated node)
    track_node_costs "mh.vs1.large-ord" "$BID_PRICE_MH" "$P50_PRICING_MH"

    # Track ARMOR write volume
    track_armor_write_volume

    echo "Cost tracking complete"
}

main "$@"
