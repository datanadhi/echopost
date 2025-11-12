package tools

import (
	"context"
	"net"
	"os"
	"sync"

	pb "github.com/datanadhi/echopost/logagentpb"

	"google.golang.org/grpc"
)

// server implements the gRPC LogAgent service.
// Each agent runs a local gRPC server that receives logs from SDKs
// and writes them into Pebble for temporary storage.
type server struct {
	pb.UnimplementedLogAgentServer
	config *ServerConfig
}

// StartGRPCServer starts a local gRPC server bound to a Unix socket.
// It listens for log messages sent by SDKs or client applications.
// The server is gracefully stopped when the provided context is cancelled.
func (c *ServerConfig) StartGRPCServer(ctx context.Context, wg *sync.WaitGroup) error {
	// Clean up any stale socket file before binding
	_ = os.Remove(c.SocketPath)

	// Start Unix socket listener
	lis, err := net.Listen("unix", c.SocketPath)
	if err != nil {
		LogJson("grpc_listen_error", map[string]any{"error": err.Error()})
		return err
	}

	// Ensure socket is world-accessible (SDKs may run as different users)
	_ = os.Chmod(c.SocketPath, 0777)

	// Create and register the gRPC server
	s := grpc.NewServer()
	pb.RegisterLogAgentServer(s, &server{config: c})

	// Start serving gRPC requests in a background goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := s.Serve(lis); err != nil {
			LogJson("grpc_server_error", map[string]any{"error": err.Error()})
		}
	}()

	// Gracefully shut down the server when the context is cancelled
	wg.Add(1)
	go func() {
		defer wg.Done()
		<-ctx.Done()
		LogJson("grpc_server_stopping", nil)
		s.GracefulStop()
		_ = lis.Close()
		LogJson("grpc_server_stopped", nil)
	}()

	return nil
}
