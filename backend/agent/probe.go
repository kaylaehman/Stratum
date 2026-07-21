package agent

import (
	"context"
	"crypto/tls"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	stratumv1 "github.com/KAE-Labs/stratum/proto/gen/stratum/v1"
)

// agentPort is the default TCP port the agent gRPC server listens on.
const agentPort = 7750

// probeTimeout bounds a single agent liveness probe.
const probeTimeout = 5 * time.Second

// ProbeAgent dials host:agentPort with mTLS (ServerName pinned to the node's
// stable SAN) and executes a Ping RPC. It returns true when the agent is
// reachable and responds, false otherwise. A nil tlsCfg always returns false.
func ProbeAgent(ctx context.Context, nodeID, host string, tlsCfg *tls.Config) bool {
	if tlsCfg == nil || host == "" {
		return false
	}

	addr := fmt.Sprintf("%s:%d", host, agentPort)
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(credentials.NewTLS(pinnedTLS(tlsCfg, nodeID))))
	if err != nil {
		return false
	}
	defer conn.Close()

	cctx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()

	rpc := stratumv1.NewAgentServiceClient(conn)
	resp, err := rpc.Ping(cctx, &stratumv1.PingRequest{Nonce: "probe"})
	return err == nil && resp.GetNonce() == "probe"
}
