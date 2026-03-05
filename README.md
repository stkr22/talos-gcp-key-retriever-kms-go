# KMS Gateway for Talos Disk Encryption

[![Go](https://img.shields.io/badge/Go-00ADD8?logo=go&logoColor=fff)](#)
[![License: GPL v3](https://img.shields.io/badge/License-GPLv3-blue.svg)](LICENSE)

A stateless gRPC gateway that bridges [Talos Linux](https://www.talos.dev/) KMS disk encryption with [Google Cloud KMS](https://cloud.google.com/kms). Designed to run on a Raspberry Pi as a boot dependency for Talos nodes with encrypted STATE and EPHEMERAL partitions.

## How It Works

```
Talos Node                    Raspberry Pi                  Google Cloud
┌──────────┐    gRPC:4050     ┌──────────────┐    HTTPS     ┌───────────┐
│          │ ──── Seal() ───> │              │ ── Encrypt ─> │           │
│  Boot    │                  │ KMS Gateway  │               │ Cloud KMS │
│  Loader  │ <── ciphertext ─ │  (stateless) │ <─ response ─ │           │
│          │                  │              │               │           │
│          │ ── Unseal() ───> │              │ ── Decrypt ─> │           │
│          │ <── plaintext ── │              │ <─ response ─ │           │
└──────────┘                  └──────────────┘               └───────────┘
```

1. **First boot**: Talos generates a random disk encryption key, calls `Seal(node_uuid, key)` — the gateway encrypts it via GCP Cloud KMS and returns the ciphertext, which is stored in the META partition.
2. **Subsequent boots**: Talos reads the sealed blob from META, calls `Unseal(node_uuid, blob)` — the gateway decrypts it via GCP Cloud KMS and returns the plaintext key to unlock LUKS2 volumes.

The `node_uuid` is passed as Additional Authenticated Data (AAD), binding each sealed key to a specific node.

## Threat Model

The threat is **physical theft of the server**. Without access to the KMS gateway (and its GCP credentials), sealed key blobs in the META partition are useless. The gateway runs without TLS on the local network — network sniffing on a home LAN is not in scope.

## GCP Cloud KMS Setup

This gateway uses **Cloud KMS symmetric encryption keys** — not Secret Manager. The difference matters: with Secret Manager you store and retrieve a passphrase, meaning the secret exists on your server. With Cloud KMS symmetric keys, encryption and decryption happen server-side at Google — the actual cryptographic key never leaves GCP infrastructure. Your server only ever sees ciphertext.

### Create the KMS resources

```bash
PROJECT_ID="your-gcp-project"
LOCATION="europe-west1"  # choose a region close to you

# Create a Key Ring (permanent once created — choose the name carefully)
gcloud kms keyrings create talos \
  --project=$PROJECT_ID \
  --location=$LOCATION

# Create a symmetric encryption key
gcloud kms keys create disk-encryption \
  --project=$PROJECT_ID \
  --location=$LOCATION \
  --keyring=talos \
  --purpose=encryption \
  --protection-level=software
```

### Create a service account

```bash
# Create a dedicated service account
gcloud iam service-accounts create kms-gateway \
  --project=$PROJECT_ID \
  --display-name="Talos KMS Gateway"

# Grant encrypt/decrypt permission on the specific key only
gcloud kms keys add-iam-policy-binding disk-encryption \
  --project=$PROJECT_ID \
  --location=$LOCATION \
  --keyring=talos \
  --member="serviceAccount:kms-gateway@${PROJECT_ID}.iam.gserviceaccount.com" \
  --role="roles/cloudkms.cryptoKeyEncrypterDecrypter"

# Export the service account key JSON
gcloud iam service-accounts keys create credentials.json \
  --iam-account="kms-gateway@${PROJECT_ID}.iam.gserviceaccount.com"
```

The resulting `KMS_KEY_NAME` for your config is:
```
projects/your-gcp-project/locations/europe-west1/keyRings/talos/cryptoKeys/disk-encryption
```

### Cost

Cloud KMS costs ~$0.06/month per active key version + $0.03 per 10,000 operations. For a home Talos cluster this is essentially free.

## Prerequisites

- GCP Cloud KMS resources created as described above
- A Raspberry Pi (arm64) or similar device on the same network as Talos nodes

## Configuration

The gateway is configured via environment variables:

| Variable | Required | Default | Description |
|---|---|---|---|
| `KMS_LISTEN_ADDRESS` | No | `0.0.0.0:4050` | gRPC listen address |
| `KMS_GCP_CREDENTIALS_FILE` | Yes | — | Path to GCP service account JSON key |
| `KMS_KEY_NAME` | Yes | — | Full KMS key resource name |

The key name format is: `projects/PROJECT/locations/LOCATION/keyRings/KEYRING/cryptoKeys/KEY`

GCP credentials are read into memory at startup. The file can reside on tmpfs and be deleted after the service starts.

## Build

```bash
# Build for Raspberry Pi (arm64)
make build

# Build for current platform
make build-local

# Run tests
make test

# Run linter
make lint

# Build Docker image for arm64
make docker
```

## Deployment

### Systemd (bare metal on Raspberry Pi)

```bash
# Copy the binary
sudo cp bin/kms-gateway-linux-arm64 /usr/local/bin/kms-gateway

# Create service user
sudo useradd --system --no-create-home kms-gateway

# Install config
sudo mkdir -p /etc/kms-gateway
sudo cp deploy/kms-gateway.env.example /etc/kms-gateway/kms-gateway.env
# Edit /etc/kms-gateway/kms-gateway.env with your values

# Place GCP credentials (consider using tmpfs)
sudo cp credentials.json /run/kms-gateway/credentials.json

# Install and start the service
sudo cp deploy/kms-gateway.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now kms-gateway
```

### Docker

```bash
docker run -d \
  -p 4050:4050 \
  -v /path/to/credentials.json:/run/kms-gateway/credentials.json:ro \
  -e KMS_GCP_CREDENTIALS_FILE=/run/kms-gateway/credentials.json \
  -e KMS_KEY_NAME=projects/my-project/locations/global/keyRings/talos/cryptoKeys/disk-key \
  ghcr.io/<owner>/talos-gcp-key-retriever-kms-go:latest
```

## Talos Configuration

Point Talos nodes at the gateway in the machine config:

```yaml
machine:
  systemDiskEncryption:
    state:
      provider: kms
      keys:
        - kms:
            endpoint: grpc://<raspberry-pi-ip>:4050
    ephemeral:
      provider: kms
      keys:
        - kms:
            endpoint: grpc://<raspberry-pi-ip>:4050
```

## Health Check

The gateway exposes a standard gRPC health check (`grpc.health.v1.Health`). Use `grpc-health-probe` or any compatible tool:

```bash
grpc-health-probe -addr=localhost:4050
```

## Disaster Recovery

- The KMS gateway is a **critical boot dependency** — if it is down, Talos nodes cannot unlock their disks.
- The gateway is **stateless** — it does not store any keys. Deploy a new Raspberry Pi with the same GCP credentials to restore service.
- Sealed keys in the META partition are portable — the same GCP KMS key will always unseal them.
- Keep a backup of the GCP service account JSON in a secure location (password manager, separate cloud backup).
- For redundancy, run a second instance of the gateway on another device. Talos will retry the KMS endpoint on failure.
