# syntax=docker/dockerfile:1

ARG GO_VERSION=1.27.1

FROM golang:${GO_VERSION} AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -trimpath \
    -ldflags="-s -w -X github.com/taagomag/m-generator/cmd.version=${VERSION}" \
    -o /out/metrics-generator ./cmd/metrics-generator

FROM gcr.io/distroless/static:nonroot
COPY --from=build /out/metrics-generator /usr/local/bin/metrics-generator
ENTRYPOINT ["/usr/local/bin/metrics-generator"]
CMD ["generate", "--config", "/etc/metrics-generator/config.yaml"]
