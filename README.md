# gateway

A gateway service that provides HTTP endpoints for authentication, proxying requests to an upstream gRPC authentication service.

## Features

- HTTP API for authentication (login, register, refresh, revoke)
- Protected routes with JWT-based authentication middleware
- Cookie and header-based token management
- Integration with gRPC authentication service

## Testing

The project includes comprehensive integration tests for all authentication HTTP handlers. Tests cover:

- Successful authentication flow
- Failed authentication with invalid credentials
- User registration
- Token refresh behavior
- Token revocation
- Protected route access with valid tokens
- Protected route rejection with missing/invalid/expired tokens

### Running Tests

Run all tests with:

```bash
go test ./...
```

Run only the authentication handler integration tests:

```bash
go test -v ./internal/http/handlers/...
```

### Test Setup

The integration tests use:
- **httptest**: To spin up test HTTP servers
- **testify**: For assertions and test utilities
- **Mock gRPC clients**: To simulate the authentication service without external dependencies

All tests are deterministic, isolated, and require no additional setup or external services to run.

## Development

### Prerequisites

- Go 1.24.2 or later
- Access to gRPC authentication service (for production)

### Building

```bash
go build ./cmd
```

### Running

Set the required environment variables and run:

```bash
export HTTP_ADDR=":8080"
export GRPC_ADDR="localhost:50051"
go run ./cmd
```

Or use flags:

```bash
go run ./cmd -http=":8080" -grpc="localhost:50051"
```
