# e-Library REST API

A lightweight, thread-safe RESTful API built in Go for managing e-book loans.

## Features

- **In-Memory Storage**: Thread-safe repository using `sync.RWMutex` and Go maps.
- **Concurrent-Safe**: Handles simultaneous borrow/return requests with no data races.
- **Modern Routing**: Go 1.22+ `http.ServeMux` with method-prefixed patterns.
- **Structured Logging**: Every request logs method, path, status, and latency via `log/slog` (JSON output).
- **Request Validation**: Input validated before reaching business logic; consistent `{"error": "..."}` responses.
- **Graceful Shutdown**: Listens for `SIGINT`/`SIGTERM` and drains in-flight requests before exiting.
- **25 Tests**: Unit tests (mock-backed) and integration tests (real in-memory store), including `-race` detector support.

## Prerequisites

- Go 1.22 or higher

## Getting Started

1. **Clone the repository** (or navigate to the folder).
2. **Run the application**:
   ```bash
   go run main.go
   ```
3. The server starts on port **3000** by default. Override with the `PORT` environment variable:
   ```bash
   PORT=8080 go run main.go
   ```
4. **Health check**:
   ```bash
   curl http://localhost:3000/
   # e-Library API is running
   ```
5. **Run tests**:
   ```bash
   go test -race -v ./tests/...
   ```
6. **View coverage report**:
   ```bash
   go test ./tests/... -coverprofile=coverage.out -coverpkg=./...
   go tool cover -html=coverage.out
   ```

## Project Structure

```
.
├── main.go               # Server setup, seeding, graceful shutdown
├── handlers/             # HTTP layer — decodes requests, maps errors to status codes
├── service/              # Business logic — loan periods, validation rules, error mapping
├── repository/           # In-memory store — thread-safe reads/writes
├── middleware/           # Logging middleware — method, path, status, latency
├── routes/               # Router wiring — registers handlers, applies middleware
├── validator/            # Request structs with Validate() methods
├── models/               # Shared domain types (BookDetail, LoanDetail)
├── respond/              # JSON response helpers (respond.JSON, respond.Error)
└── tests/                # All tests — unit (mocks) and integration (real store)
```

## API Endpoints

All responses (success and error) use `Content-Type: application/json`.

### GET /Book — Search for a book

Retrieve details and available copies of a specific book title.

- **Query Param:** `title` (string, required)
- **Example:**
  ```bash
  curl "http://localhost:3000/Book?title=Clean+Code"
  ```
- **Success `200`:**
  ```json
  {"title": "Clean Code", "available_copies": 1}
  ```

### POST /Borrow — Borrow a book

Borrow a book for a standard loan period of **28 days**. A user cannot borrow the same book twice concurrently.

- **Body:** `{"name": "string", "title": "string"}`
- **Example:**
  ```bash
  curl -X POST http://localhost:3000/Borrow \
    -H "Content-Type: application/json" \
    -d '{"name": "Anurag", "title": "Clean Code"}'
  ```
- **Success `201`:**
  ```json
  {
    "name_of_borrower": "Anurag",
    "book_title": "Clean Code",
    "loan_date": "2026-03-25T10:00:00Z",
    "return_date": "2026-04-22T10:00:00Z"
  }
  ```

### POST /Extend — Extend a loan

Extend an existing loan's return date by **21 days**. Each loan may only be extended **once**; a second attempt is rejected.

- **Body:** `{"name": "string", "title": "string"}`
- **Example:**
  ```bash
  curl -X POST http://localhost:3000/Extend \
    -H "Content-Type: application/json" \
    -d '{"name": "Anurag", "title": "Clean Code"}'
  ```
- **Success `200`:** Returns the updated loan record (same shape as Borrow response, with `"extended": true`).

### POST /Return — Return a book

Return a borrowed book; restores one copy to inventory.

- **Body:** `{"name": "string", "title": "string"}`
- **Example:**
  ```bash
  curl -X POST http://localhost:3000/Return \
    -H "Content-Type: application/json" \
    -d '{"name": "Anurag", "title": "Clean Code"}'
  ```
- **Success `200`:**
  ```json
  {"message": "Book returned successfully"}
  ```

### Error Responses

All errors share a consistent JSON shape:

```json
{"error": "<description>"}
```

| Status | Meaning |
|---|---|
| `400 Bad Request` | Invalid or malformed JSON, or missing required fields (`name`/`title`) |
| `404 Not Found` | Book or active loan record does not exist |
| `409 Conflict` | No copies available, user already has an active loan for that title, or loan has already been extended |
| `500 Internal Server Error` | Unexpected server-side failure |

## Design

### Package Architecture

The codebase follows a layered architecture with clear separation of concerns:

```
HTTP Request → middleware → handler → service → repository
                 (log)    (decode/  (business  (store
                           validate)  logic)    read/write)
```

Each layer depends only on interfaces, not concrete types (Dependency Inversion):
- `Handler` depends on `service.BookService` and `service.LoanService` interfaces
- `service.libraryService` depends on the `repository.Store` interface
- Tests inject mock implementations without touching production wiring

### Concurrency Model

All shared states are protected by a `sync.RWMutex` in `LibraryStore`:

- **`RLock`** — `GET /Book`: allows multiple concurrent readers.
- **`Lock`** — `Borrow`, `Extend`, `Return`: exclusive write lock ensures atomicity (e.g. two users borrowing the last copy simultaneously both get the correct outcome).

`GetBook` copies the `BookDetail` value under the lock before releasing, preventing a data race where a pointer could be read after another goroutine modifies the underlying struct.

`CreateLoan` performs a check-then-act (stock check → duplicate check → decrement → store) as a single atomic write, preventing TOCTOU races.

### Loan Storage

Loans are stored in `map[string]LoanDetail` keyed by a `(name, title)` composite key. This gives O(1) lookup, update, and deletion — compared to O(n) for a slice-based approach. A null-byte separator (`\x00`) between name and title prevents key collisions for inputs that share a prefix.

### Request Validation

Each request type (in the `validator` package) owns a `Validate()` method. Validation runs in the handler before any service call, returning `400` immediately on failure. This keeps business logic free of input-sanitisation concerns (SRP).

### Response Safety

`respond.JSON` encodes the response body into a buffer *before* writing headers. This ensures a failed encode returns `500` instead of sending a partial `200` with a broken body.

### Logging

`middleware.Logging` wraps the entire router and logs every request using structured `log/slog` fields: `method`, `path`, `status`, `latency_ms`. Output is JSON-formatted, compatible with log aggregators (Datadog, CloudWatch, etc.).

### Seeding

On startup, the library is seeded with:

- "The Go Programming Language" — three copies
- "Clean Code" — one copy

Title uniqueness is enforced by `AddBook` (returns `ErrDuplicateBook` on collision).

## Future Improvements

#### 1. Persistent storage (e.g. PostgreSQL)
The service depends on the `repository.Store` interface. Swapping the in-memory `LibraryStore` for a `database/sql`-backed implementation requires no changes to the handler or service layers.

#### 2. Authentication / Authorization
Add middleware to verify a caller's identity before allowing borrow/extend/return operations.
