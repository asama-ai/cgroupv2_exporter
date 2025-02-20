VERSION ?= $(shell git describe --tags --always --dirty)
COMMIT  ?= $(shell git rev-parse --short HEAD)

# Build flags for optimization
LDFLAGS := -s -w \
	-X main.version=${VERSION} \
	-X main.commit=${COMMIT} \
	-buildid= \
	'-extldflags=-static-pie -z relro -z now'

# Build flags
GOFLAGS := -trimpath -tags=netgo,osusergo -buildmode=pie

# Use GOAMD64=v1 for better compatibility
GOAMD64 = v1  # Supports older CPUs

.PHONY: build
build: cgroupv2_exporter

cgroupv2_exporter:
	CGO_ENABLED=0 \
	GOAMD64=${GOAMD64} \
	go build \
		-ldflags "$(LDFLAGS)" \
		$(GOFLAGS) \
		-o cgroupv2_exporter \
		cgroupv2_exporter.go


.PHONY: build-small
build-small: cgroupv2_exporter-small

cgroupv2_exporter-small:
	CGO_ENABLED=0 \
	GOAMD64=${GOAMD64} \
	go build \
		-ldflags "$(LDFLAGS)" \
		$(GOFLAGS) \
		-gcflags=all="-l -B" \
		-o cgroupv2_exporter \
		cgroupv2_exporter.go

.PHONY: build-debug
build-debug: host-agent-debug textfile-collector-debug

cgroupv2_exporter-debug:
	CGO_ENABLED=0 \
	GOAMD64=${GOAMD64} \
	go build \
		-ldflags "-X main.version=${VERSION} -X main.commit=${COMMIT}" \
		-gcflags=all="-N -l" \
		-o cgroupv2_exporter \
		cgroupv2_exporter.go

.PHONY: install
install:
	CGO_ENABLED=0 \
	GOAMD64=${GOAMD64} \
	go install \
		-ldflags "$(LDFLAGS)" \
		$(GOFLAGS)

.PHONY: verify
verify:
	go vet ./...
	go test -race ./...
	if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "Warning: golangci-lint not installed, skipping lint checks"; \
	fi

.PHONY: clean
clean:
	rm -f cgroupv2_exporter