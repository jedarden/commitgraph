#!/usr/bin/bash
# query-infra-costs.sh - Query and report infrastructure costs from VictoriaLogs
#
# Provides visibility into:
# - Spot node bid spend (bid price × uptime)
# - ARMOR write volume for commitgraph prefix
# - Comparison against p50 percentile pricing ($0.006/hr)
#
# Usage:
#   ./scripts/query-infra-costs.sh [time_range]
#
# Examples:
#   ./scripts/query-infra-costs.sh           # Default: last 24 hours
#   ./scripts/query-infra-costs.sh 7d       # Last 7 days
#   ./scripts/query-infra-costs.sh 30d      # Last 30 days
#   ./scripts/query-infra-costs.sh 1h      # Last hour

set -euo pipefail

# Configuration
CLUSTER="${CLUSTER:-ord-devimprint}"
NAMESPACE="${NAMESPACE:-commitgraph}"
VLOGS_ENDPOINT="${VLOGS_ENDPOINT:-http://traefik-ardenone-manager:8001/api/v1/namespaces/monitoring/services/vlogs-server:http/proxy}"
COMMITGRAPH_PREFIX="${COMMITGRAPH_PREFIX:-commitgraph}"

# Time range (VictoriaLogs syntax)
TIME_RANGE="${1:-24h}"  # Default: last 24 hours

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Query VictoriaLogs
query_vlogs() {
    local query="$1"

    curl -s -X POST "${VLOGS_ENDPOINT}/api/v1/query" \
        -H "Accept: application/json" \
        -G \
        --data-urlencode "query=${query}" \
        --data-urlencode "time_range=${TIME_RANGE}" \
        --data-urlencode "limit=10000" | jq '.responses[0].result // []'
}

# Format currency
format_currency() {
    local amount="$1"
    printf "$%0.2f" "$amount"
}

# Report node costs
report_node_costs() {
    echo -e "${BLUE}=== Spot Node Costs ===${NC}"
    echo "Cluster: ${CLUSTER}"
    echo "Time Range: ${TIME_RANGE}"
    echo ""

    # Query for node cost metrics
    local query='metric_type="node_cost" AND cluster="'${CLUSTER}'" AND namespace="'${NAMESPACE}'"'
    local results
    results=$(query_vlogs "$query")

    if [ "$(echo "$results" | jq 'length')" -eq 0 ]; then
        echo -e "${YELLOW}No node cost metrics found. Cost tracking may not be running.${NC}"
        echo ""
        echo "To start tracking, run: ./scripts/track-infra-costs.sh"
        return
    fi

    # Group by server class
    echo "$results" | jq -r '.[] | .server_class' | sort -u | while read -r server_class; do
        echo -e "${GREEN}Server Class: ${server_class}${NC}"

        local class_data
        class_data=$(echo "$results" | jq "[.[] | select(.server_class==\"${server_class}\")]")

        local node_count
        node_count=$(echo "$class_data" | jq 'map(.node_name) | unique | length')
        echo "  Nodes: ${node_count}"

        local total_hours
        total_hours=$(echo "$class_data" | jq '[.uptime_hours | tonumber] | add // 0')
        echo "  Total Uptime: $(printf "%.2f" "$total_hours") hours"

        local bid_price
        bid_price=$(echo "$class_data" | jq -r '.[0].bid_price // "0"')
        local p50_price
        p50_price=$(echo "$class_data" | jq -r '.[0].p50_price // "0"')

        local total_bid_spend
        total_bid_spend=$(echo "$class_data" | jq '[.bid_spend | tonumber] | add // 0')
        local total_p50_spend
        total_p50_spend=$(echo "$class_data" | jq '[.p50_spend | tonumber] | add // 0')
        local savings
        savings=$(echo "$total_p50_spend - $total_bid_spend" | bc)

        echo "  Bid Price: \$${bid_price}/hr (p50: \$${p50_price}/hr)"
        echo "  Total Bid Spend: $(format_currency "$total_bid_spend")"
        echo "  p50 Equivalent: $(format_currency "$total_p50_spend")"
        echo "  Savings vs p50: $(format_currency "$savings")"
        echo ""
    done
}

# Report ARMOR write volume
report_armor_write_volume() {
    echo -e "${BLUE}=== ARMOR Write Volume ===${NC}"
    echo "Prefix: ${COMMITGRAPH_PREFIX}"
    echo ""

    # Query for ARMOR write volume metrics
    local query='metric_type="armor_write_volume" AND prefix="'${COMMITGRAPH_PREFIX}'"'
    local results
    results=$(query_vlogs "$query")

    if [ "$(echo "$results" | jq 'length')" -eq 0 ]; then
        echo -e "${YELLOW}No ARMOR write volume metrics found.${NC}"
        echo ""
        echo "Note: ARMOR write volume tracking requires ARMOR API metrics, which are not yet available."
        return
    fi

    local total_bytes
    total_bytes=$(echo "$results" | jq '[.write_bytes | tonumber] | add // 0')
    local total_count
    total_count=$(echo "$results" | jq '[.write_count | tonumber] | add // 0')

    echo "  Total Writes: ${total_count}"
    echo "  Total Volume: $(numfmt --to=si --round=nearest "$total_bytes" 2>/dev/null || echo "$total_bytes")"
    echo ""
}

# Summary comparison
report_summary() {
    echo -e "${BLUE}=== Cost Summary vs p50 Pricing ===${NC}"
    echo ""

    local query='metric_type="node_cost" AND cluster="'${CLUSTER}'" AND namespace="'${NAMESPACE}'"'
    local results
    results=$(query_vlogs "$query")

    if [ "$(echo "$results" | jq 'length')" -eq 0 ]; then
        return
    fi

    local total_bid_spend
    total_bid_spend=$(echo "$results" | jq '[.bid_spend | tonumber] | add // 0')
    local total_p50_spend
    total_p50_spend=$(echo "$results" | jq '[.p50_spend | tonumber] | add // 0')
    local total_hours
    total_hours=$(echo "$results" | jq '[.uptime_hours | tonumber] | add // 0')

    echo "  Total Node Uptime: $(printf "%.2f" "$total_hours") hours"
    echo "  Total Bid Spend: $(format_currency "$total_bid_spend")"
    echo "  p50 Equivalent: $(format_currency "$total_p50_spend")"
    echo "  Total Savings: $(format_currency "$(echo "$total_p50_spend - $total_bid_spend" | bc)")"
    echo ""

    # Compare against plan.md cited pricing
    echo -e "${GREEN}Reference Pricing (from plan.md):${NC}"
    echo "  p50 (both classes): \$0.006/hr"
    echo "  ch.vs1.medium-ord current bid: \$0.001/hr"
    echo "  mh.vs1.large-ord current bid: \$0.02/hr (API minimum)"
    echo ""
}

main() {
    echo "Infrastructure Cost Report"
    echo "Generated: $(date -u +"%Y-%m-%dT%H:%M:%SZ")"
    echo ""

    report_node_costs
    report_armor_write_volume
    report_summary

    echo -e "${BLUE}Query complete${NC}"
}

main "$@"
