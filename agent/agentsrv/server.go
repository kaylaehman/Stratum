// Package agentsrv implements the AgentService gRPC server. It wires together
// the watch and initdetect packages and handles the Ping, WatchFiles, and
// DetectInit RPCs.
package agentsrv

import (
	"context"
	"log/slog"

	"google.golang.org/grpc"

	"github.com/kaylaehman/stratum/agent/initdetect"
	"github.com/kaylaehman/stratum/agent/watch"
	stratumv1 "github.com/kaylaehman/stratum/proto/gen/stratum/v1"
)

// agentVersion is stamped at build time via -ldflags; falls back to "dev".
var agentVersion = "dev"

// Server implements stratumv1.AgentServiceServer.
type Server struct {
	stratumv1.UnimplementedAgentServiceServer
	watchPaths []string
	logger     *slog.Logger
}

// New creates a Server for the given watch paths.
func New(watchPaths []string, logger *slog.Logger) *Server {
	return &Server{watchPaths: watchPaths, logger: logger}
}

// Register attaches the server to a grpc.Server.
func (s *Server) Register(grpcSrv *grpc.Server) {
	stratumv1.RegisterAgentServiceServer(grpcSrv, s)
}

// Ping echoes the nonce and reports the agent version.
func (s *Server) Ping(_ context.Context, req *stratumv1.PingRequest) (*stratumv1.PingResponse, error) {
	return &stratumv1.PingResponse{Nonce: req.GetNonce(), AgentVersion: agentVersion}, nil
}

// WatchFiles starts inotify watches on the requested paths and streams events
// until the client cancels.
func (s *Server) WatchFiles(req *stratumv1.WatchFilesRequest, stream grpc.ServerStreamingServer[stratumv1.WatchFilesResponse]) error {
	ctx := stream.Context()

	paths := req.GetPaths()
	if len(paths) == 0 {
		paths = s.watchPaths
	}

	w, err := watch.New(s.logger)
	if err != nil {
		return err
	}
	defer w.Close()

	for _, p := range paths {
		if addErr := w.Add(p, req.GetRecursive()); addErr != nil {
			s.logger.Warn("agentsrv: watch add", "path", p, "error", addErr)
		}
	}

	go w.Run(req.GetRecursive())

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case ev, ok := <-w.Events():
			if !ok {
				return nil
			}
			if sendErr := stream.Send(ev); sendErr != nil {
				return sendErr
			}
		}
	}
}

// DetectInit returns the init system active on this host.
func (s *Server) DetectInit(_ context.Context, _ *stratumv1.DetectInitRequest) (*stratumv1.DetectInitResponse, error) {
	sys, desc := initdetect.Detect()
	return &stratumv1.DetectInitResponse{InitSystem: sys, Description: desc}, nil
}
