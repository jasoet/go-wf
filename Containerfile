# Build stage (shared)
FROM golang:1.26 AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .

# Container worker build
FROM builder AS container-worker-build
RUN CGO_ENABLED=0 go build -tags example -o /container-worker ./examples/container/worker/

# Function worker build
FROM builder AS function-worker-build
RUN CGO_ENABLED=0 go build -tags example -o /function-worker ./examples/function/worker/

# Trigger build
FROM builder AS trigger-build
RUN CGO_ENABLED=0 go build -tags example -o /trigger ./examples/trigger/

# Runtime: container worker
FROM gcr.io/distroless/static AS container-worker
COPY --from=container-worker-build /container-worker /container-worker
ENTRYPOINT ["/container-worker"]

# Runtime: function worker
FROM gcr.io/distroless/static AS function-worker
COPY --from=function-worker-build /function-worker /function-worker
ENTRYPOINT ["/function-worker"]

# Runtime: trigger
FROM gcr.io/distroless/static AS trigger
COPY --from=trigger-build /trigger /trigger
ENTRYPOINT ["/trigger"]
