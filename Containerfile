# Build stage (shared)
FROM golang:1.26 AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .

# Docker worker build
FROM builder AS docker-worker-build
RUN CGO_ENABLED=0 go build -tags example -o /docker-worker ./examples/docker/worker/

# Function worker build
FROM builder AS function-worker-build
RUN CGO_ENABLED=0 go build -tags example -o /function-worker ./examples/function/worker/

# Trigger build
FROM builder AS trigger-build
RUN CGO_ENABLED=0 go build -tags example -o /trigger ./examples/trigger/

# Runtime: docker worker
FROM gcr.io/distroless/static AS docker-worker
COPY --from=docker-worker-build /docker-worker /docker-worker
ENTRYPOINT ["/docker-worker"]

# Runtime: function worker
FROM gcr.io/distroless/static AS function-worker
COPY --from=function-worker-build /function-worker /function-worker
ENTRYPOINT ["/function-worker"]

# Runtime: trigger
FROM gcr.io/distroless/static AS trigger
COPY --from=trigger-build /trigger /trigger
ENTRYPOINT ["/trigger"]
