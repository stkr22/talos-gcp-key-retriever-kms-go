package gateway

import (
	"context"

	gcpkms "cloud.google.com/go/kms/apiv1"
	"cloud.google.com/go/kms/apiv1/kmspb"
	gax "github.com/googleapis/gax-go/v2"
)

// gcpKMSClientAPI defines the methods on the real GCP KMS client that we use.
type gcpKMSClientAPI interface {
	Encrypt(ctx context.Context, req *kmspb.EncryptRequest, opts ...gax.CallOption) (*kmspb.EncryptResponse, error)
	Decrypt(ctx context.Context, req *kmspb.DecryptRequest, opts ...gax.CallOption) (*kmspb.DecryptResponse, error)
}

// Verify at compile time that the real client satisfies the interface.
var _ gcpKMSClientAPI = (*gcpkms.KeyManagementClient)(nil)

// GCPClientWrapper wraps a real GCP KMS client to satisfy the KMSClient interface.
type GCPClientWrapper struct {
	client gcpKMSClientAPI
}

// NewGCPClientWrapper creates a wrapper around a real GCP KMS client.
func NewGCPClientWrapper(client *gcpkms.KeyManagementClient) *GCPClientWrapper {
	return &GCPClientWrapper{client: client}
}

// Encrypt delegates to the GCP KMS client.
func (w *GCPClientWrapper) Encrypt(ctx context.Context, req *kmspb.EncryptRequest) (*kmspb.EncryptResponse, error) {
	return w.client.Encrypt(ctx, req)
}

// Decrypt delegates to the GCP KMS client.
func (w *GCPClientWrapper) Decrypt(ctx context.Context, req *kmspb.DecryptRequest) (*kmspb.DecryptResponse, error) {
	return w.client.Decrypt(ctx, req)
}
