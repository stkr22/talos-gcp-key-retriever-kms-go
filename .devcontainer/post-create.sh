#!/usr/bin/env bash
set -euo pipefail

echo ">>> Installing Go tools..."

go install golang.org/x/tools/gopls@latest
go install golang.org/x/tools/cmd/goimports@latest
go install github.com/go-delve/delve/cmd/dlv@latest
go install gotest.tools/gotestsum@latest
go install github.com/air-verse/air@latest          # live reload
go install golang.org/x/vuln/cmd/govulncheck@latest

# golangci-lint (recommended install method)
curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh \
  | sh -s -- -b "$(go env GOPATH)/bin" latest

echo ">>> Go tools installed successfully."

# Download module dependencies if go.mod exists
if [ -f "go.mod" ]; then
  echo ">>> Downloading Go modules..."
  go mod download
fi

echo ">>> Post-create setup complete."