ARG GCFLAGS
ARG TARGETARCH

FROM golang:1.18-alpine as builder

WORKDIR /workspace

COPY go.mod go.mod
COPY go.sum go.sum

RUN go mod download

COPY main.go main.go
COPY admission_controller.go admission_controller.go

RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} GO111MODULE=on go build -gcflags "${GCFLAGS}" -a -o webhook .

FROM golang:1.18-alpine as dlv

RUN CGO_ENABLED=0 go install github.com/go-delve/delve/cmd/dlv@latest

WORKDIR /

COPY --from=builder /workspace/webhook .

ENTRYPOINT ["dlv", "--listen=:2345", "--headless=true", "--api-version=2", "--accept-multiclient", "exec", "--", "/webhook"]

FROM gcr.io/distroless/static:nonroot

WORKDIR /

COPY --from=builder /workspace/webhook .

USER nonroot:nonroot

ENTRYPOINT ["/webhook"]