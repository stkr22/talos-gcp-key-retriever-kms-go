package gateway

import (
	"context"

	"cloud.google.com/go/kms/apiv1/kmspb"
	"github.com/siderolabs/kms-client/api/kms"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// KMSClient defines the subset of the GCP KMS API used by the gateway.
type KMSClient interface {
	Encrypt(ctx context.Context, req *kmspb.EncryptRequest) (*kmspb.EncryptResponse, error)
	Decrypt(ctx context.Context, req *kmspb.DecryptRequest) (*kmspb.DecryptResponse, error)
}

// Server implements the Talos KMSServiceServer by delegating to Google Cloud KMS.
type Server struct {
	kms.UnimplementedKMSServiceServer
	logger    *zap.Logger
	kmsClient KMSClient
	keyName   string
}

// NewServer creates a new KMS gateway server.
func NewServer(logger *zap.Logger, kmsClient KMSClient, keyName string) *Server {
	return &Server{
		logger:    logger,
		kmsClient: kmsClient,
		keyName:   keyName,
	}
}

// Seal encrypts the provided data using Google Cloud KMS.
// The node_uuid is used as Additional Authenticated Data (AAD) to bind the
// ciphertext to a specific node.
func (s *Server) Seal(ctx context.Context, req *kms.Request) (*kms.Response, error) {
	s.logger.Info("seal request",
		zap.String("node_uuid", req.GetNodeUuid()),
		zap.Int("data_len", len(req.GetData())),
	)

	if err := validateRequest(req); err != nil {
		return nil, err
	}

	resp, err := s.kmsClient.Encrypt(ctx, &kmspb.EncryptRequest{
		Name:                        s.keyName,
		Plaintext:                   req.GetData(),
		AdditionalAuthenticatedData: []byte(req.GetNodeUuid()),
	})
	if err != nil {
		s.logger.Error("GCP KMS encrypt failed",
			zap.String("node_uuid", req.GetNodeUuid()),
			zap.Error(err),
		)
		return nil, status.Errorf(codes.Internal, "encryption failed: %v", err)
	}

	s.logger.Info("seal successful",
		zap.String("node_uuid", req.GetNodeUuid()),
		zap.Int("ciphertext_len", len(resp.GetCiphertext())),
	)

	return &kms.Response{Data: resp.GetCiphertext()}, nil
}

// Unseal decrypts the provided ciphertext using Google Cloud KMS.
// The node_uuid must match the AAD used during Seal.
func (s *Server) Unseal(ctx context.Context, req *kms.Request) (*kms.Response, error) {
	s.logger.Info("unseal request",
		zap.String("node_uuid", req.GetNodeUuid()),
		zap.Int("data_len", len(req.GetData())),
	)

	if err := validateRequest(req); err != nil {
		return nil, err
	}

	resp, err := s.kmsClient.Decrypt(ctx, &kmspb.DecryptRequest{
		Name:                        s.keyName,
		Ciphertext:                  req.GetData(),
		AdditionalAuthenticatedData: []byte(req.GetNodeUuid()),
	})
	if err != nil {
		s.logger.Error("GCP KMS decrypt failed",
			zap.String("node_uuid", req.GetNodeUuid()),
			zap.Error(err),
		)
		return nil, status.Errorf(codes.Internal, "decryption failed: %v", err)
	}

	s.logger.Info("unseal successful",
		zap.String("node_uuid", req.GetNodeUuid()),
	)

	return &kms.Response{Data: resp.GetPlaintext()}, nil
}

func validateRequest(req *kms.Request) error {
	if len(req.GetData()) == 0 {
		return status.Error(codes.InvalidArgument, "data is empty")
	}
	if req.GetNodeUuid() == "" {
		return status.Error(codes.InvalidArgument, "node_uuid is empty")
	}
	return nil
}
