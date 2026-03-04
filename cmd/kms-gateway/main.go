package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"

	gcpkms "cloud.google.com/go/kms/apiv1"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"google.golang.org/api/option"
	"google.golang.org/grpc"

	"github.com/talos-gcp-key-retriever-kms-go/internal/config"
	"github.com/talos-gcp-key-retriever-kms-go/internal/gateway"
	"github.com/talos-gcp-key-retriever-kms-go/internal/health"

	kms "github.com/siderolabs/kms-client/api/kms"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "configuration error: %s\n", err)
		os.Exit(1)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if err := run(ctx, cfg); err != nil {
		fmt.Fprintf(os.Stderr, "error: %s\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, cfg *config.Config) error {
	logger, err := zap.NewProduction()
	if err != nil {
		return fmt.Errorf("creating logger: %w", err)
	}
	defer logger.Sync() //nolint:errcheck

	// Read GCP credentials into memory
	credBytes, err := os.ReadFile(cfg.GCPCredentialsFile)
	if err != nil {
		return fmt.Errorf("reading credentials file: %w", err)
	}
	logger.Info("loaded GCP credentials", zap.String("file", cfg.GCPCredentialsFile))

	// Create GCP KMS client
	kmsClient, err := gcpkms.NewKeyManagementClient(ctx, option.WithCredentialsJSON(credBytes))
	if err != nil {
		return fmt.Errorf("creating GCP KMS client: %w", err)
	}
	defer func() {
		if err := kmsClient.Close(); err != nil {
			logger.Error("closing KMS client", zap.Error(err))
		}
	}()

	// Create gateway server
	srv := gateway.NewServer(logger, gateway.NewGCPClientWrapper(kmsClient), cfg.KMSKeyName)

	// Create gRPC server (no TLS — local network only)
	s := grpc.NewServer()
	kms.RegisterKMSServiceServer(s, srv)
	health.Register(s)

	// Listen
	lis, err := net.Listen("tcp", cfg.ListenAddress)
	if err != nil {
		return fmt.Errorf("listening on %s: %w", cfg.ListenAddress, err)
	}
	logger.Info("KMS gateway listening",
		zap.String("address", cfg.ListenAddress),
		zap.String("kms_key", cfg.KMSKeyName),
	)

	// Serve with graceful shutdown
	eg, ctx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		return s.Serve(lis)
	})
	eg.Go(func() error {
		<-ctx.Done()
		logger.Info("shutting down gRPC server")
		s.GracefulStop()
		return nil
	})

	return eg.Wait()
}
