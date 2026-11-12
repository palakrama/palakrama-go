# palakrama-go

Go library shared by the nine Palakrama services (auth, blog, comment,
communication, media, order, rsvp, site, stats). It is a module, not a service:
it has no `main`, database, or configuration, and only the delivery and
infrastructure layers of a service import it. Domain and application code stay
in each service.

| Package | What |
|---|---|
| `middleware` | `Operational(service, maintenanceMode, next)`: wraps a service's whole handler. Adds `X-Service-Name` and `X-Request-ID` (a valid incoming id, 1–64 of `A-Za-z0-9-_`, is kept), logs requests (every GET and POST, server errors, anything slower than 100 ms), turns a panic into a 500 `SERVICE_ERROR`, and with `MAINTENANCE_MODE` on (1, true, yes, on) answers everything but `/health` with 503 `SERVICE_MAINTENANCE` and `Retry-After: 60`; `RequireKey(header, reject, next, keys...)`: lets a request through when the header holds one of the keys (constant-time; an empty key never matches) and records which one (`KeyOf`, 1-based); otherwise `reject(w, configured)` answers: `RejectText`, `RejectTextUnconfigured(msg)` (500 when no key is set), `RejectJSON(body)`, or the service's own. `MatchKey` is the comparison alone |
| `httpx` | `WriteJSON(w, status, value)`: a JSON answer (`Content-Type: application/json`). `DecodeJSON(r, target)`: exactly one JSON value from the body, unknown fields and anything after the value refused (`ErrMultipleValues`); the size limit stays with the service (`http.MaxBytesHandler`). Error bodies are not here: each service keeps its own shape |
| `client` | `New(baseURL, keyHeader, key, timeout)`: a JSON client for one service. `Post`/`Get`/`Do(ctx, path, body, result, header...)` send the key header (and `Content-Type: application/json` with a body), decode a 2xx answer into `result` (at most 1 MiB; nil skips), and return a `*StatusError` (method, path, status, headers, up to 64 KiB of body) for any other status; network and timeout errors read `POST /path: <cause>`. Mapping a status to a domain error, and skipping calls when a target is not configured, stay in the service's adapter |

Each service keeps a small wrapper with its own header and rejection, and its
own error body (their answers differ and callers rely on them). Calls to
outside providers (Midtrans, Google, email) are not here.

## Using it

A service requires a tagged version (`require github.com/palakrama/palakrama-go v0.1.0`).
The repository is private:

- **Locally**, `go.work` at the workspace root (`Projects/`, outside every
  repository) uses this folder and every service, and replaces the required
  version with this folder, so `./dev.sh` and `go build` see local changes at
  once. When a service requires a new version, update the version in the
  `replace` line too.
- **CI** of each service sets `GOPRIVATE=github.com/palakrama/*` and reads the
  module with the `REPOSITORY_READ_TOKEN` secret.

The workspace file, in case it is lost:

```
go 1.27.1

use (
	./auth
	./blog
	./comment
	./communication
	./media
	./order
	./palakrama-go
	./rsvp
	./site
	./stats
)

replace github.com/palakrama/palakrama-go v0.1.0 => ./palakrama-go
```

## Releasing

1. Change and test here (`go vet ./... && go test ./...`), add `docs/CHANGELOG/<version>.md`.
2. Commit, tag (`git tag v0.2.0`), and push the tag.
3. In each service: `go mod edit -require=github.com/palakrama/palakrama-go@v0.2.0`, then
   `GOWORK=off go mod tidy` (fills `go.sum`; needs read access to the repository), and
   bump the `replace` version in `go.work`.

Keep the API stable: services move to a new version one by one.

Latest changes: [`docs/CHANGELOG/v0.1.0.md`](./docs/CHANGELOG/v0.1.0.md).
