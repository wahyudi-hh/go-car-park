# Go Car Park Availability Service

This project is a backend service developed as part of my journey to learn **Golang**. It provides an API to find the nearest available carparks in Singapore based on user coordinates.

The project follows **Clean Architecture** principles (Controller-Service-Repository) and focuses on writing idiomatic, testable, and performant Go code.

## 🚀 Purpose
The main goal of this project was to transition from a Java-based background to Go, specifically focusing on:
- **Dependency Injection**: Managing service layers without external frameworks.
- **Concurrency**: Using Go's native features for efficient data handling.
- **Testing**: Implementing full Integration Tests using `httptest` and `testify`.
- **Project Structure**: Following the `internal/` and `cmd/` directory patterns.

## 🛠 Features
- **Nearest Carpark Search**: Calculates distances between user coordinates and carpark locations provided via CSV.
- **Real-time Availability**: Fetches live data from the Singapore Government's Carpark Availability API.
- **Data Transformation**: Maps SVY21 coordinate systems to readable formats for distance calculation.
- **Robust Testing**: 85%+ code coverage using mock servers and JSON test data.

---

## 📖 API Specification

### 1. Get Nearest Carparks
Returns a paginated list of the closest carparks with their current available lots.

**Endpoint:** `GET /car-parks/nearest`

**Query Parameters:**
| Parameter | Type | Required | Description |
| :--- | :--- | :--- | :--- |
| `user_x`  | float | Yes      | User's X coordinate (SVY21 format) |
| `user_y`  | float | Yes      | User's Y coordinate (SVY21 format) |
| `page`    | int   | No       | Page number (default: 1) |
| `size`    | int   | No       | Items per page (default: 10) |
| `lot_type`| string| No       | Specific lot type to be search |

**Sample Response:**
```json
{
  "total_elements": 1,
  "pages": 1,
  "content": [
    {
      "car_park_no": "ACB",
      "address": "BLK 456 JALAN JALAN",
      "total_lots": "100",
      "lots_available": "50"
    }
  ]
}

## 🚦 Getting Started

### Prerequisites

- Go 1.21 or higher
- A `.env` file (see `.env` sample below)

```bash
API_TIMEOUT_SEC=10
CACHE_REFRESH_PERIOD_SEC=60
CSV_PATH=./data/HDBCarparkInformation.csv
LIVE_CARPARK_API_URL=https://api.data.gov.sg/v1/transport/carpark-availability
USE_REDIS=false
REDIS_ADDR=127.0.0.1:6379
REDIS_PASS=
```

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
go test -v "-coverprofile=coverage.out" "-coverpkg=./internal/..." ./test/...

go tool cover -html coverage.out
```

## 🏗 Architecture

- `cmd/`: Entry point for the application.
- `internal/handler/`: Gin HTTP handlers (controllers) that process incoming requests.
- `internal/service/`: Core business logic, including distance calculation and result sorting.
- `internal/client/`: External API consumers for real-time Government API data.
- `internal/model/`: Shared domain entities, structs, and JSON DTOs.
- `test/testdata/`: JSON files used for mocking external API responses in integration tests.
