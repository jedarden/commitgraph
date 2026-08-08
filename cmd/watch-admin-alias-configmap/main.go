// watch-admin-alias-configmap watches the admin-alias ConfigMap for changes
// and re-runs the loader whenever it changes.
//
// This command runs as a long-lived Deployment in the cluster, watching
// the mounted ConfigMap file for modifications and triggering the alias
// loader whenever changes are detected.
//
// Usage:
//
//	watch-admin-alias-configmap -configmap-path <path> [flags]
//
// The command:
// - Runs the loader once on startup (initial sync)
// - Watches the ConfigMap file for changes
// - Re-runs the loader on each change (with debouncing)
// - Logs all actions and errors
// - Continues watching even after errors
//
// Flags are passed through to load-admin-aliases except -configmap, which
// specifies the file to watch.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/jedarden/commitgraph/pkg/errors"
	"github.com/jedarden/commitgraph/pkg/watcher"
)

var (
	// Postgres connection flags (passed through to load-admin-aliases)
	dbHost     = flag.String("db-host", "", "PostgreSQL host (required)")
	dbPort     = flag.String("db-port", "5432", "PostgreSQL port")
	dbName     = flag.String("db-name", "commitgraph", "PostgreSQL database name")
	dbUser     = flag.String("db-user", "", "PostgreSQL user (required)")
	dbPassword = flag.String("db-password", "", "PostgreSQL password (required)")
	sslMode    = flag.String("sslmode", "require", "PostgreSQL SSL mode")

	// Watcher flags
	configMapPath = flag.String("configmap-path", "",
		"Path to admin-alias-configmap.yml to watch (required)")
	pollInterval = flag.Duration("poll-interval", 5*time.Second,
		"How often to check for file changes")
	debounce = flag.Duration("debounce", 5*time.Second,
		"Wait time after last change before triggering loader (prevents duplicate runs)")

	// Removal behavior (passed through to load-admin-aliases)
	autoDeleteRemoved = flag.Bool("auto-delete-removed", false,
		"Automatically delete aliases removed from the ConfigMap")
)

func main() {
	flag.Usage = usage
	flag.Parse()

	// Create CLI handler for consistent error handling
	cliHandler := errors.NewCLIHandler("watch-admin-alias-configmap")

	if *dbHost == "" {
		cliHandler.HandleError(errors.RequiredFlagError("db-host"))
	}
	if *dbUser == "" {
		cliHandler.HandleError(errors.RequiredFlagError("db-user"))
	}
	if *dbPassword == "" {
		cliHandler.HandleError(errors.RequiredFlagError("db-password"))
	}
	if *configMapPath == "" {
		cliHandler.HandleError(errors.RequiredFlagError("configmap-path"))
	}

	// Resolve path
	configPath, err := watcher.ResolvePath(*configMapPath)
	if err != nil {
		cliHandler.HandleError(errors.ResourceErrorf("watch-admin-alias-configmap", "resolve_path", *configMapPath, "failed to resolve configmap path: %v", err))
	}

	// Verify the file exists
	if _, err := os.Stat(configPath); err != nil {
		cliHandler.HandleError(errors.ResourceErrorf("watch-admin-alias-configmap", "stat_file", configPath, "configmap file does not exist: %v", err))
	}

	log.Printf("Starting admin-alias-configmap watcher...\n")
	log.Printf("  ConfigMap path: %s\n", configPath)
	log.Printf("  Poll interval: %v\n", *pollInterval)
	log.Printf("  Debounce: %v\n", *debounce)
	log.Printf("  Auto-delete removed: %v\n", *autoDeleteRemoved)

	// Build base arguments for load-admin-aliases
	baseArgs := []string{
		"-db-host", *dbHost,
		"-db-port", *dbPort,
		"-db-name", *dbName,
		"-db-user", *dbUser,
		"-db-password", *dbPassword,
		"-sslmode", *sslMode,
		"-configmap", configPath,
	}
	if *autoDeleteRemoved {
		baseArgs = append(baseArgs, "-auto-delete-removed=true")
	}

	// Create callback that runs load-admin-aliases
	callback := func() error {
		return runLoader(baseArgs)
	}

	// Create and run watcher
	fw := watcher.NewFileWatcher(configPath, *pollInterval, *debounce, callback)

	// Run with initial sync
	if err := fw.Run(true /* initialRun */); err != nil {
		cliHandler.HandleError(errors.ConfigErrorf("watch-admin-alias-configmap", "watcher failed: %v", err))
	}
}

// runLoader executes the load-admin-aliases command with the given arguments.
func runLoader(args []string) error {
	log.Println("Executing load-admin-aliases...")

	// Find the load-admin-aliases binary
	// First try the compiled binary in the same directory
	binPath := "load-admin-aliases"

	cmd := exec.Command(binPath, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	startTime := time.Now()
	err := cmd.Run()
	duration := time.Since(startTime)

	if err != nil {
		log.Printf("load-admin-aliases failed after %v: %v\n", duration, err)
		return fmt.Errorf("loader failed: %w", err)
	}

	log.Printf("load-admin-aliases completed successfully in %v\n", duration)
	return nil
}

// usage prints the usage message.
func usage() {
	fmt.Fprintf(os.Stderr, `watch-admin-alias-configmap: Watch admin-alias ConfigMap and auto-sync

This command runs as a long-lived Deployment, watching the admin-alias-configmap.yml
file for changes and re-running the loader whenever it changes.

Usage:
  watch-admin-alias-configmap [flags]

Flags:
  -configmap-path string
        Path to admin-alias-configmap.yml to watch (required)
        Example: /etc/config/admin-alias-configmap.yml
  -poll-interval duration
        How often to check for file changes (default 5s)
  -debounce duration
        Wait time after last change before triggering (default 5s)
        Prevents duplicate runs when the file changes rapidly

  Postgres connection flags (passed through to load-admin-aliases):
  -db-host string
        PostgreSQL host (required)
  -db-port string
        PostgreSQL port (default "5432")
  -db-name string
        PostgreSQL database name (default "commitgraph")
  -db-user string
        PostgreSQL user (required)
  -db-password string
        PostgreSQL password (required)
  -sslmode string
        PostgreSQL SSL mode (default "require")
  -auto-delete-removed
        Automatically delete aliases removed from ConfigMap (default false)

What it does:
  1. Runs load-admin-aliases once on startup (initial sync)
  2. Watches the ConfigMap file for modifications
  3. Waits for debounce period after last change
  4. Re-runs load-admin-aliases if change is still present
  5. Continues watching even after errors

Time bound:
  Changes are reflected in user_aliases within:
  - poll-interval + debounce + load time
  - Default: 5s + 5s + ~1-2s = ~11-12 seconds worst case
  - Average: much faster (poll hits change mid-cycle)

Idempotency:
  load-admin-aliases is idempotent (ON CONFLICT DO UPDATE), so rapid
  re-runs are safe. Debouncing avoids unnecessary work but isn't required
  for correctness.

Deployment:
  This is meant to run as a Kubernetes Deployment with:
  - The ConfigMap mounted as a volume
  - Postgres credentials via Secret
  - Restart policy: Always (run forever)
  - Resource limits: CPU/memory requests appropriate for polling

Example K8s deployment:
  apiVersion: apps/v1
  kind: Deployment
  metadata:
    name: admin-alias-configmap-watcher
    namespace: commitgraph
  spec:
    replicas: 1
    selector:
      matchLabels:
        app: admin-alias-configmap-watcher
    template:
      metadata:
        labels:
          app: admin-alias-configmap-watcher
      spec:
        containers:
        - name: watcher
          image: <image>
          args:
          - -configmap-path=/etc/config/admin-alias-configmap.yml
          - -db-host=$(DB_HOST)
          - -db-user=$(DB_USER)
          - -db-password=$(DB_PASSWORD)
          env:
          - name: DB_HOST
            value: postgres-commitgraph.commitgraph.svc.cluster.local
          - name: DB_USER
            valueFrom:
              secretKeyRef:
                name: postgres-credentials
                key: username
          - name: DB_PASSWORD
            valueFrom:
              secretKeyRef:
                name: postgres-credentials
                key: password
          volumeMounts:
          - name: config
            mountPath: /etc/config
            readOnly: true
        volumes:
        - name: config
          configMap:
            name: admin-alias-configmap
`)
	cliHandler := errors.NewCLIHandler("watch-admin-alias-configmap")
	cliHandler.HandleError(errors.InvalidFlagValueError("usage", "help requested", "showing usage"))
}
