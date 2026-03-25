# e-Library REST API

A lightweight, thread-safe RESTful API built in Go for managing e-book loans.

## Features
- **In-Memory Storage**: Thread-safe repository using `sync.RWMutex` and Go maps.
- **Concurrent-Safe**: Handles multiple simultaneous borrow/return requests with no data races.
- **Modern Routing**: Utilizes Go 1.22+ enhanced `http.ServeMux`.
- **Structured Error Responses**: All errors return a consistent JSON envelope `{"error": "..."}`.
- **Unit Tested**: 17 tests covering all endpoints, success and error paths, with `-race` detector.

## Prerequisites
- Go 1.22 or higher

## Getting Started

1. **Clone the repository** (or navigate to the folder).
2. **Run the application**:
   ```bash
   go run main.go
   ```
3. The application will start and listen on port **3000**.
4. Run tests to verify logic and coverage:
   ```bash
   go test -race -v
   go test -coverprofile=coverage.out
   ```
5. To view the HTML coverage report:
   ```bash
   go tool cover -html=coverage.out
   ```

# API Endpoints

All responses (success and error) use `Content-Type: application/json`.

### 1. Search for a Book

Retrieve details and available copies of a specific book title.

- **Endpoint:** `GET /Book`
- **Query Param:** `title` (string, required)
- **Example:**
  ```bash
  curl "http://localhost:3000/Book?title=Clean+Code"
  ```
- **Success `200`:**
  ```json
  {"title": "Clean Code", "available_copies": 1}
  ```

### 2. Borrow a Book

Borrow a book for a standard loan period of 4 weeks. A user cannot borrow the same book twice concurrently.

- **Endpoint:** `POST /Borrow`
- **Body:** `{"name": "string", "title": "string"}`
- **Example:**
  ```bash
  curl -X POST http://localhost:3000/Borrow \
    -H "Content-Type: application/json" \
    -d '{"name": "Anurag", "title": "Clean Code"}'
  ```
- **Success `201`:**
  ```json
  {"name_of_borrower": "Anurag", "book_title": "Clean Code", "loan_date": "...", "return_date": "..."}
  ```

### 3. Extend a Loan

Extend an existing loan by an additional 3 weeks from the current return date.

- **Endpoint:** `POST /Extend`
- **Body:** `{"name": "string", "title": "string"}`
- **Example:**
  ```bash
  curl -X POST http://localhost:3000/Extend \
    -H "Content-Type: application/json" \
    -d '{"name": "Anurag", "title": "Clean Code"}'
  ```
- **Success `200`:** Returns the updated loan record.

### 4. Return a Book

Return a borrowed book to the library inventory.

- **Endpoint:** `POST /Return`
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

All error responses share a consistent JSON shape:

```json
{"error": "<description>"}
```

| Status | Meaning |
|---|---|
| `400 Bad Request` | Invalid JSON or missing required fields |
| `404 Not Found` | Book or loan record does not exist |
| `409 Conflict` | Book out of stock, or user already has an active loan for that book |
| `201 Created` | Loan successfully created |

# Design Considerations

## Concurrency Model

All shared states are protected by a `sync.RWMutex`:

- **RLock (Read Lock):** Used for `GET /Book` to allow multiple simultaneous readers.
- **Lock (Write Lock):** Used for `Borrow`, `Extend`, and `Return` to ensure atomicity (e.g., two users borrowing the last copy simultaneously).

The `GetBook` handler copies the `BookDetail` value before releasing the lock, preventing a data race where the pointer could be read after another goroutine modifies it.

## Loan Storage

Loans are stored in a `map[string]LoanDetail` keyed by a `(name, title)` composite key. This gives O(1) lookup, update, and deletion for `Extend` and `Return` — compared to the O(n) linear scan a slice would require. A null-byte separator (`\x00`) is used between name and title to prevent key collisions.

## Structured Error Handling

All error responses are returned as JSON via a shared `writeError` helper, and all success responses are written via `writeJSON`. The `writeJSON` helper encodes the response body into a buffer before writing any headers — this ensures a failed encode returns `500` instead of silently sending a `200` with a broken body.

## Seeding

Upon initialization, the library is seeded with:

- "The Go Programming Language" (three copies)
- "Clean Code" (one copy)

## Future Improvements

#### 1. Implement persistent storage (e.g. PostgreSQL)
The `Handler` currently depends on the concrete `*LibraryStore`. Extracting a `Store` interface with methods like `GetBook`, `CreateLoan`, and `DeleteLoan` would allow swapping the in-memory implementation for a `database/sql` or `gorm`-backed one without touching handler code.

#### 2. Request logging middleware
Adding a middleware layer (e.g. `rs/zerolog` or `zap`) to log method, path, status code, and latency on every request would improve observability.

