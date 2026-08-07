module github.com/jedarden/commitgraph/containers/login-revalidation-worker

go 1.25.0

require (
	github.com/jedarden/commitgraph v0.0.0-00010101000000-000000000000
	github.com/lib/pq v1.12.3
)

// pkg/client/queueapi (imported by main.go since cg-56gg2) lives in the
// repo root module, which is never published — resolve it from the local
// checkout instead of trying to fetch github.com/jedarden/commitgraph.
replace github.com/jedarden/commitgraph => ../..
