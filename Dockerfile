FROM --platform=$BUILDPLATFORM golang:1.25-trixie AS builder

ARG TARGETOS=linux
ARG TARGETARCH=arm64

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .

RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -ldflags="-s -w" -o /kms-gateway ./cmd/kms-gateway/

FROM gcr.io/distroless/static-debian13:nonroot

COPY --from=builder /kms-gateway /kms-gateway

EXPOSE 4050

ENTRYPOINT ["/kms-gateway"]
