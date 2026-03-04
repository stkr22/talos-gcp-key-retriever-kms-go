package health

import (
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

// Register creates and registers a gRPC health check server.
// It sets the overall server status and the KMS service status to SERVING.
func Register(s *grpc.Server) *health.Server {
	hsrv := health.NewServer()
	healthpb.RegisterHealthServer(s, hsrv)
	hsrv.SetServingStatus("sidero.kms.KMSService", healthpb.HealthCheckResponse_SERVING)
	hsrv.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
	return hsrv
}
