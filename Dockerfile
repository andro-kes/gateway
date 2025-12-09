FROM golang:1.24.2-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ENV CGO_ENABLED=0
RUN GOOS=linux GOARCH=amd64 go build -o /bin/app ./cmd/server/

FROM alpine:3.21
COPY --from=builder /bin/app /bin/app
EXPOSE 8080
CMD ["/bin/app"]
