# kverr

Package `kverr` attaches structured key-value context to errors so that
high-cardinality data (user IDs, request IDs, etc.) travels with the error
through the call stack and can be extracted at the logging site.

The core idea is to separate message cardinality from context:

```
// high-cardinality — bad for alerting
"unable to delete user 180: mysql has gone away"

// structured — cardinality of 1 per error type, context in fields
msg="unable to delete user" error="mysql has gone away" user_id=180
```

KV pairs accumulate through any number of wrapping layers, including
`fmt.Errorf("%w", ...)` layers in between. `errors.Is` and `errors.As` work
correctly through a kverr boundary — sentinel errors remain detectable.

## Installation

```
go get github.com/sethgrid/kverr
```

Requires Go 1.21+.

## Quick start

```go
func queryDB(userID string) error {
    // Typed attrs (compile-time safe):
    return kverr.New(sql.ErrNoRows, slog.String("user_id", userID), slog.String("table", "users"))
    // Or the convenient string/any form:
    // return kverr.New(sql.ErrNoRows, "user_id", userID, "table", "users")
}

func fetchUser(userID, requestID string) error {
    if err := queryDB(userID); err != nil {
        // Adds context at this layer; inner KV pairs are preserved automatically.
        return kverr.New(err, "request_id", requestID)
    }
    return nil
}

func handler(w http.ResponseWriter, r *http.Request) {
    err := fetchUser("u-123", "req-abc")
    if err != nil {
        slog.Error("request failed", kverr.Args(err)...)
        // → msg="request failed" user_id=u-123 table=users request_id=req-abc

        // errors.Is still works through the kverr boundary.
        if errors.Is(err, sql.ErrNoRows) { ... }
    }
}
```

## API

### `New(err error, args ...any) *Error`

Wraps `err` with additional KV context. `args` may be `slog.Attr` values,
alternating `string`/`any` pairs, or a mix:

```go
kverr.New(err, "user_id", uid)
kverr.New(err, slog.Int("user_id", uid))
kverr.New(err, slog.Int("user_id", uid), "req_id", rid)  // mixed
```

### `Args(err error, extra ...any) []any`

Extracts all KV pairs as a flat `[]any` slice for slog's variadic API:

```go
slog.Error("msg", kverr.Args(err)...)
slog.Error("msg", kverr.Args(err, "extra_key", "extra_val")...)
```

### `Map(err error) map[string]any`

Returns all KV pairs as `map[string]any`. Useful for zap, zerolog, or
direct key access:

```go
for k, v := range kverr.Map(err) {
    fields = append(fields, zap.Any(k, v))
}
```

### `(*Error).LogValue() slog.Value`

Implements `slog.LogValuer`. Pass a `*kverr.Error` directly as a slog
argument and its KV pairs are emitted as a structured group automatically:

```go
slog.Error("msg", "err", err)
// → msg="msg" err.user_id=u-123 err.table=users
```

## Logger examples

See the [`examples/`](examples/) directory for complete, runnable programs
using **slog**, **zap**, and **zerolog**.
