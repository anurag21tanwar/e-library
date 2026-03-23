# e-Library REST API

A lightweight, thread-safe RESTful API built in Go for managing e-book loans.

## Features
- **In-Memory Storage**: Uses a thread-safe repository with `sync.RWMutex`.
- **Concurrent-Safe**: Handles multiple simultaneous borrow/return requests.
- **Modern Routing**: Utilizes Go 1.22+ enhanced `http.ServeMux`.
- **Unit Tested**: Includes functional tests using `net/http/httptest`.

## Prerequisites
- Go 1.22 or higher

## Getting Started

1. **Clone the repository** (or navigate to the folder).
2. **Run the application**:
   ```bash
   go run main.go
3. The application will start and listen on port **3000**.
4. Run Tests to verify the logic and coverage:
    ```bash
   go test -v
   go test -coverprofile=coverage.out
   ```
5. To view the HTML report 
   ```bash
   go tool cover -html=coverage.out
   
# API Endpoints

### 1. Search for a Book

Retrieve details and available copies of a specific book title.

- **Endpoint:** `GET /Book`
- **Query Param:** `title` (string)
- **Example:**
    ```bash
    curl "http://localhost:3000/Book?title=Clean+Code"

### 2. Borrow a Book

Borrow a book for a standard loan period of 4 weeks.

- **Endpoint:** `POST /Borrow`
- **Body:** `{"name": "string", "title": "string"}`
- **Example:**
    ```bash 
  curl -X POST http://localhost:3000/Borrow -H "Content-Type: application/json" -d '{"name": "Anurag", "title": "Clean Code"}'

### 3. Extend a Loan

Extend an existing loan by an additional 3 weeks from the current return date.

- **Endpoint:** `POST /Extend`
- **Body:** `{"name": "string", "title": "string"}`
- **Example:**
    ```bash 
    curl -X POST http://localhost:3000/Extend -H "Content-Type: application/json" -d '{"name": "Anurag", "title": "Clean Code"}'

### 4. Return a Book

Return a borrowed book to the library inventory.

- **Endpoint:** `POST /Return`
- **Body:** `{"name": "string", "title": "string"}`
- **Example:**
    ```bash 
  curl -X POST http://localhost:3000/Return -H "Content-Type: application/json" -d '{"name": "Anurag", "title": "Clean Code"}'

# Design Considerations

## Concurrency Model

Since the storage is in-memory, I used a `sync.RWMutex`.

- **RLock (Read Lock):** Used for the `/Book` endpoint to allow multiple simultaneous readers, maximizing performance.
- **Lock (Write Lock):** Used for `Borrow`, `Extend`, and `Return` operations to ensure atomicity and prevent race conditions (e.g., two users borrowing the last copy of a book at once).

## Slice Manipulation
To maintain a high-performance in-memory store, loan records are removed from the internal slice using the idiomatic Go approach: `append(s[:i], s[i+1:]...)`. This avoids unnecessary allocations.

## Error Handling & HTTP Status
The API adheres to REST standards:

- `201 Created`: Successfully created a loan.
- `404 Not Found`: Book or Loan record does not exist.
- `409 Conflict`: Request valid, but book is currently out of stock.
- `400 Bad Request`: Invalid JSON or missing required fields.

## Seeding
Upon initialization, the library is seeded with:

- "The Go Programming Language" (3 copies)

- "Clean Code" (1 copy)

## Future Improvements

#### 1. Implement persistent storage, such as a Postgres database
- To implement the Persistent Storage bonus task, I have structured the LibraryStore as a dependency. This allows for a seamless migration to PostgreSQL by implementing a Repository Interface, moving from in-memory mutexes to ACID-compliant database transactions.
- I would define a Repository interface with methods like `GetBook(title string)`, `UpdateStock(title string, change int)`, and `CreateLoan(loan LoanDetail)`.
- Currently, the `Handler` depends on the `LibraryStore` struct. I would change the `Handler` to depend on that `Repository` interface instead. This allows me to swap the `In-Memory` implementation for a `Postgres` implementation using `database/sql` or an `ORM` like `Gorm` without changing a single line of code in the HTTP handlers."

#### 2. Incorporate logging for API requests and responses
- I have included `log.Fatal` for server startup and `fmt.Printf` for basic request tracking. In a real production app, I would  use a middleware (like `rs/zerolog` or `zap`).