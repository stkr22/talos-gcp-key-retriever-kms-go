package gateway

import (
	"context"
	"fmt"
	"testing"

	"cloud.google.com/go/kms/apiv1/kmspb"
	"github.com/siderolabs/kms-client/api/kms"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type mockKMSClient struct {
	encryptFn func(ctx context.Context, req *kmspb.EncryptRequest) (*kmspb.EncryptResponse, error)
	decryptFn func(ctx context.Context, req *kmspb.DecryptRequest) (*kmspb.DecryptResponse, error)
}

func (m *mockKMSClient) Encrypt(ctx context.Context, req *kmspb.EncryptRequest) (*kmspb.EncryptResponse, error) {
	return m.encryptFn(ctx, req)
}

func (m *mockKMSClient) Decrypt(ctx context.Context, req *kmspb.DecryptRequest) (*kmspb.DecryptResponse, error) {
	return m.decryptFn(ctx, req)
}

const testKeyName = "projects/test/locations/global/keyRings/test/cryptoKeys/test"

func newTestServer(mock *mockKMSClient) *Server {
	logger := zap.NewNop()
	return NewServer(logger, mock, testKeyName)
}

func TestSealSuccess(t *testing.T) {
	mock := &mockKMSClient{
		encryptFn: func(_ context.Context, req *kmspb.EncryptRequest) (*kmspb.EncryptResponse, error) {
			return &kmspb.EncryptResponse{Ciphertext: []byte("encrypted-" + string(req.Plaintext))}, nil
		},
	}
	srv := newTestServer(mock)

	resp, err := srv.Seal(context.Background(), &kms.Request{
		NodeUuid: "node-1",
		Data:     []byte("secret-key"),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(resp.GetData()) != "encrypted-secret-key" {
		t.Fatalf("unexpected response: %s", resp.GetData())
	}
}

func TestUnsealSuccess(t *testing.T) {
	mock := &mockKMSClient{
		decryptFn: func(_ context.Context, req *kmspb.DecryptRequest) (*kmspb.DecryptResponse, error) {
			return &kmspb.DecryptResponse{Plaintext: []byte("decrypted-" + string(req.Ciphertext))}, nil
		},
	}
	srv := newTestServer(mock)

	resp, err := srv.Unseal(context.Background(), &kms.Request{
		NodeUuid: "node-1",
		Data:     []byte("sealed-blob"),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(resp.GetData()) != "decrypted-sealed-blob" {
		t.Fatalf("unexpected response: %s", resp.GetData())
	}
}

func TestSealPassesAAD(t *testing.T) {
	var capturedAAD []byte
	mock := &mockKMSClient{
		encryptFn: func(_ context.Context, req *kmspb.EncryptRequest) (*kmspb.EncryptResponse, error) {
			capturedAAD = req.AdditionalAuthenticatedData
			return &kmspb.EncryptResponse{Ciphertext: []byte("ct")}, nil
		},
	}
	srv := newTestServer(mock)

	_, err := srv.Seal(context.Background(), &kms.Request{
		NodeUuid: "node-abc-123",
		Data:     []byte("data"),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(capturedAAD) != "node-abc-123" {
		t.Fatalf("expected AAD 'node-abc-123', got '%s'", capturedAAD)
	}
}

func TestUnsealPassesAAD(t *testing.T) {
	var capturedAAD []byte
	mock := &mockKMSClient{
		decryptFn: func(_ context.Context, req *kmspb.DecryptRequest) (*kmspb.DecryptResponse, error) {
			capturedAAD = req.AdditionalAuthenticatedData
			return &kmspb.DecryptResponse{Plaintext: []byte("pt")}, nil
		},
	}
	srv := newTestServer(mock)

	_, err := srv.Unseal(context.Background(), &kms.Request{
		NodeUuid: "node-xyz-789",
		Data:     []byte("data"),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(capturedAAD) != "node-xyz-789" {
		t.Fatalf("expected AAD 'node-xyz-789', got '%s'", capturedAAD)
	}
}

func TestSealPassesKeyName(t *testing.T) {
	var capturedName string
	mock := &mockKMSClient{
		encryptFn: func(_ context.Context, req *kmspb.EncryptRequest) (*kmspb.EncryptResponse, error) {
			capturedName = req.Name
			return &kmspb.EncryptResponse{Ciphertext: []byte("ct")}, nil
		},
	}
	srv := newTestServer(mock)

	_, err := srv.Seal(context.Background(), &kms.Request{
		NodeUuid: "node-1",
		Data:     []byte("data"),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedName != testKeyName {
		t.Fatalf("expected key name %q, got %q", testKeyName, capturedName)
	}
}

func TestSealEmptyData(t *testing.T) {
	srv := newTestServer(&mockKMSClient{})

	_, err := srv.Seal(context.Background(), &kms.Request{
		NodeUuid: "node-1",
		Data:     nil,
	})
	if err == nil {
		t.Fatal("expected error for empty data")
	}
	if s, ok := status.FromError(err); !ok || s.Code() != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v", err)
	}
}

func TestUnsealEmptyData(t *testing.T) {
	srv := newTestServer(&mockKMSClient{})

	_, err := srv.Unseal(context.Background(), &kms.Request{
		NodeUuid: "node-1",
		Data:     nil,
	})
	if err == nil {
		t.Fatal("expected error for empty data")
	}
	if s, ok := status.FromError(err); !ok || s.Code() != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v", err)
	}
}

func TestSealEmptyNodeUUID(t *testing.T) {
	srv := newTestServer(&mockKMSClient{})

	_, err := srv.Seal(context.Background(), &kms.Request{
		NodeUuid: "",
		Data:     []byte("data"),
	})
	if err == nil {
		t.Fatal("expected error for empty node_uuid")
	}
	if s, ok := status.FromError(err); !ok || s.Code() != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v", err)
	}
}

func TestSealGCPError(t *testing.T) {
	mock := &mockKMSClient{
		encryptFn: func(_ context.Context, _ *kmspb.EncryptRequest) (*kmspb.EncryptResponse, error) {
			return nil, fmt.Errorf("gcp kms unavailable")
		},
	}
	srv := newTestServer(mock)

	_, err := srv.Seal(context.Background(), &kms.Request{
		NodeUuid: "node-1",
		Data:     []byte("data"),
	})
	if err == nil {
		t.Fatal("expected error on GCP failure")
	}
	if s, ok := status.FromError(err); !ok || s.Code() != codes.Internal {
		t.Fatalf("expected Internal error, got %v", err)
	}
}

func TestUnsealGCPError(t *testing.T) {
	mock := &mockKMSClient{
		decryptFn: func(_ context.Context, _ *kmspb.DecryptRequest) (*kmspb.DecryptResponse, error) {
			return nil, fmt.Errorf("gcp kms unavailable")
		},
	}
	srv := newTestServer(mock)

	_, err := srv.Unseal(context.Background(), &kms.Request{
		NodeUuid: "node-1",
		Data:     []byte("data"),
	})
	if err == nil {
		t.Fatal("expected error on GCP failure")
	}
	if s, ok := status.FromError(err); !ok || s.Code() != codes.Internal {
		t.Fatalf("expected Internal error, got %v", err)
	}
}
