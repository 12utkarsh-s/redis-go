# redis-go

`redis-go` is a simplified, in-memory key-value store that partially implements the Redis protocol. It's written in Go and uses a non-blocking, asynchronous TCP server to handle multiple client connections concurrently.

## Features

*   **In-memory storage:** Key-value pairs are stored in a `map`.
*   **Asynchronous I/O:** Uses `kqueue` (on macOS) for efficient handling of multiple clients.
*   **Supported Redis commands:**
    *   `PING`
    *   `SET` (with optional `EX` for expiration)
    *   `GET`
    *   `TTL`
    *   `DEL`
    *   `EXPIRE`
*   **Key expiration:** Automatically deletes expired keys.

## Getting Started

### Prerequisites

*   Go 1.18 or higher

### Installation

1.  Clone the repository:
    ```sh
    git clone https://github.com/your-username/redis-go.git
    ```
2.  Go to the project directory:
    ```sh
    cd redis-go
    ```
3.  Run the server:
    ```sh
    go run main.go
    ```

By default, the server will start on `0.0.0.0:7379`. You can change the host and port using the `-host` and `-port` flags:

```sh
go run main.go -host=127.0.0.1 -port=6379
```

### Usage

You can use any Redis client to connect to the server. For example, using `redis-cli`:

```sh
redis-cli

127.0.0.1:7379> PING
PONG
127.0.0.1:7379> SET mykey "Hello"
OK
127.0.0.1:7379> GET mykey
"Hello"
```
