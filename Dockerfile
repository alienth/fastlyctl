FROM golang:1.14 AS build

WORKDIR /workspace

COPY go.mod go.sum ./
RUN go mod download

COPY ./ ./

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
  go build -ldflags '-extldflags "-static"' \
  -o /build/fastlyctl ./cmd/fastlyctl

FROM ubuntu

COPY --from=build /build/fastlyctl /
ENTRYPOINT ["/fastlyctl"]
