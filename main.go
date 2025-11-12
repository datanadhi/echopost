package main

import (
	"context"
	t "github.com/datanadhi/echopost/tools"
	"flag"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	flow "github.com/datanadhi/flowhttp/client"
)

// Entry point for the Data Nadhi log agent.
// This binary runs a local gRPC service that receives logs, stores them in Pebble,
// and periodically pushes them to the main server when health checks pass.
func main() {
	// Command-line flags
	baseDir := flag.String("datanadhi", "./.datanadhi", "path to datanadhi folder")
	apiKey := flag.String("api-key", "", "API key used when flushing Pebble logs")
	serverHost := flag.String("health-url", "http://data-nadhi-server:5000", "Main server health check URL")
	flag.Parse()

	// Context for cancellation (graceful shutdown)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle SIGINT / SIGTERM to stop the agent gracefully
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
	}()

	wg := sync.WaitGroup{}

	// Initialize server configuration
	config := t.ServerConfig{
		ApiKey:     *apiKey,
		ServerHost: *serverHost,
		Files:      t.Files{},
	}

	// Setup local files and Pebble DB
	if fileErr := config.CreateRequiredFiles(*baseDir); fileErr != nil {
		t.LogJson("file_setup_error", map[string]any{"error": fileErr.Error()})
		return
	}
	defer config.CloseFiles()
	defer config.DisableAcceptingFlag()

	t.LogJson("agent_started", map[string]any{"socket": config.SocketPath})

	// Start local gRPC server for receiving logs from SDKs
	if err := config.StartGRPCServer(ctx, &wg); err != nil {
		t.LogJson("grpc_start_error", map[string]any{"error": err.Error()})
		return
	}

	// Start background Pebble DB flusher
	config.FlushPebbleDBOnInterval(ctx, &wg)

	client := flow.NewClient(5 * time.Second)

mainRoutine:
	for {
		switch {
		case ctx.Err() != nil:
			break mainRoutine
		// When main server is reachable (healthy)
		case config.IsHealthSuccess(client):
			_ = config.DisableAcceptingFlag()
			t.LogJson("main_healthy_not_accepting_logs", nil)

			// Flush any buffered data before attempting upload
			t.FlushPebbleDB(config.Db)
			time.Sleep(100 * time.Millisecond)

			// If Pebble is empty, exit the agent
			if t.PebbleIsEmpty(config.Db) {
				t.LogJson("pebble_empty_exiting", nil)
				break mainRoutine
			}

			// Push pending logs to the main server
			if err := config.ProcessPebble(ctx); err != nil {
				t.LogJson("pebble_process_error", map[string]any{"error": err.Error()})
				if ctx.Err() != nil {
					break mainRoutine
				}
				continue
			}

			// Stop if context canceled during upload
			if ctx.Err() != nil {
				break mainRoutine
			}

			// Wait before next health check
			time.Sleep(10 * time.Second)
			continue

		// When main server is unhealthy or unreachable
		case config.AcceptingFlag == nil:
			_ = config.EnableAcceptingFlag()
			t.LogJson("main_unhealthy_accepting_logs", nil)

			// Keep flushing Pebble periodically to persist data
			t.FlushPebbleDB(config.Db)
			time.Sleep(5 * time.Second)

		// Default state (e.g., still unhealthy)
		default:
			t.FlushPebbleDB(config.Db)
			time.Sleep(5 * time.Second)
		}
	}

	// Stop all background tasks and exit
	cancel()
	wg.Wait()
}
