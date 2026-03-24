## 🚦 Getting Started

### Prerequisites

- Go 1.21 or higher
- A `.env` file (see `.env.example`)

### Installation

Clone the repository:

```bash
git clone https://github.com/wahyudi-hh/go-car-park.git
```

Install dependencies:

```bash
go mod download
```

Run the server:

```bash
go run cmd/server/main.go
```

### Running Tests

To run the full suite, including integration tests and coverage:

```bash
go test -v -coverprofile=coverage.out ./...

go tool cover -html=coverage.out
```

## 🏗 Architecture

- `cmd/`: Entry point for the application.
- `internal/handler/`: Gin HTTP handlers (controllers) that process incoming requests.
- `internal/service/`: Core business logic, including distance calculation and result sorting.
- `internal/client/`: External API consumers for real-time Government API data.
- `internal/model/`: Shared domain entities, structs, and JSON DTOs.
- `test/testdata/`: JSON files used for mocking external API responses in integration tests.
