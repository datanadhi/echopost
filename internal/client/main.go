//go:build ignore
// +build ignore

// Package main provides a simple test client for the EchoPost agent.
// This client connects over the UNIX gRPC socket and sends a configurable
// number of log messages for validation and debugging purposes.
//
// Note: This file is excluded from all production builds and release artifacts.
// It exists solely for local testing and should not be imported or packaged.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"time"

	pb "data-nadhi-agent/logagentpb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// logJSON prints structured logs to stdout for consistent output.
func logJSON(event string, fields map[string]any) {
	entry := map[string]any{
		"timestamp": time.Now().UTC().Format(time.RFC3339Nano),
		"event":     event,
	}

	for k, v := range fields {
		entry[k] = v
	}

	b, _ := json.Marshal(entry)
	fmt.Println(string(b))
}

func main() {
	baseDir := flag.String("datanadhi", "./.datanadhi", "path to datanadhi folder")
	count := flag.Int("count", 10, "number of logs to send")
	interval := flag.Duration("interval", 300*time.Millisecond, "interval between sends")
	flag.Parse()

	socket := fmt.Sprintf("unix:%s/data-nadhi-agent.sock", *baseDir)
	conn, err := grpc.Dial(socket, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("failed to connect to agent: %v", err)
	}
	defer conn.Close()

	client := pb.NewLogAgentClient(conn)
	logJSON("client_connected", map[string]any{"socket": socket})

	for i := 0; i < *count; i++ {
		// Prepare a simple JSON payload
		jsonPayload := fmt.Sprintf(`{"msg":"log message %d","level":"INFO"}`, i)

		req := &pb.LogRequest{
			JsonData:  jsonPayload,
			Pipelines: []string{"test-pipeline"},
		}

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		resp, err := client.SendLog(ctx, req)
		cancel()

		if err != nil {
			logJSON("send_error", map[string]any{
				"index": i,
				"error": err.Error(),
			})
			continue
		}

		logJSON("send_success", map[string]any{
			"index":   i,
			"success": resp.Success,
			"message": resp.Message,
		})

		time.Sleep(*interval)
	}

	logJSON("client_done", map[string]any{"sent_count": *count})
}