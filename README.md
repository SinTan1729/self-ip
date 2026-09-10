# Self IP

A small self-hosted IP geolocation and port checking API written in Go.

The server determines the client's IP address or accepts an IP address through
a query parameter, then returns geolocation and ASN information using the
[MaxMind GeoLite2](https://dev.maxmind.com/geoip/geolite2-free-geolocation-data) databases.

It can also check for whether a port is open on a given IP address.

It is highly recommended that you use it behind a reverse proxy e.g.
[Caddy](https://caddyserver.com).

## Features

- Self-hosted HTTP API
- IPv4 and IPv6 support
- Automatic database updates from [GitHub releases](https://github.com/P3TERX/GeoLite.mmdb)
- Mandatory API key authentication
- Multiple response modes
- Configurable IP lookup through a query parameter

## Database Updates

On startup, the server checks the latest release of the
[P3TERX/GeoLite.mmdb](https://github.com/P3TERX/GeoLite.mmdb) repository.
Then, they're checked once a day around 7am.

## Deployment

The recommended method of installation is using containers e.g. Docker or Podman.
Example `docker compose` and `podman quadlet` files are provided in the
[`deploy`](./deploy) directory.

**Remember to change the API Key hash before deploying.** The provided hash is gibberish.

## Building Locally

Clone the repo, and use `make run` to build and run the binary. Make sure you create
a `.env` file with `SELF_IP_API_KEY` variable containing a valid `Argon2id` hash.

## API

All requests require the `X-API-Key` header. The API Key has to be
[`Argon2id`](https://www.argon2.com) encrypted. Check the deployment
files for some notes. The following can be used to generate an API key.

```bash
openssl rand -hex 32
```

### Basic request

```bash
curl \
  -H "X-API-Key: your-secret-api-key" \
  http://localhost:3213
```

By default, the API looks up the IP address of the requesting client.

### Look up a specific IP

Use the `ip` query parameter:

```bash
curl \
  -H "X-API-Key: your-secret-api-key" \
  "http://localhost:3213?ip=8.8.8.8"
```

## Allowed paths

The paths `/`, `/json`, `/api` are used for IP geolocation check. They return the same data.
The path `/portcheck` is used for `port` probing.
Any other paths will return:

```text
404 Page Not Found
```

## Response Modes

### Default mode

The default response returns a compact JSON representation containing information such as:

```json
{
  "ip": "1.2.3.4",
  "city": "Example City",
  "region": {
    "name": "Example Region",
    "iso": "EX"
  },
  "country": {
    "name": "Example Country",
    "iso": "EX"
  },
  "location": {
    "lat": 25.42,
    "long": -12.5561,
    "postal": "21231"
  },
  "tz": "Asia/Kolkata",
  "org": "A12345 Example Org."
}
```

Example:

```bash
curl \
  -H "X-API-Key: your-secret-api-key" \
  "http://localhost:3213?ip=8.8.8.8"
```

### IP-only mode

Return only the queried IP address:

```bash
curl \
  -H "X-API-Key: your-secret-api-key" \
  "http://localhost:3213?ip=8.8.8.8&mode=ip_only"
```

The response content type is `text/plain` e.g. `1.2.3.4`.

### `echoip` mode

This mode mimics the output of [`echoip`](https://github.com/mpolden/echoip).

```bash
curl \
  -H "X-API-Key: your-secret-api-key" \
  "http://localhost:3213?ip=8.8.8.8&mode=echoip"
```

Check [ifconfig.co](https://ifconfig.co/json) for the reply schema.

The response content type is `application/json`. It can be useful as a drop-in
replacement for `ifconfig.co`.

### Full mode

Return the complete geolocation record:

```bash
curl \
  -H "X-API-Key: your-secret-api-key" \
  "http://localhost:3213/?ip=8.8.8.8&mode=full"
```

Take a look at the [full schema here](./internal/structs.go).

The response content type is `application/json`.

## Port checker

Check the status of a port:

```bash
curl \
  -H "X-API-Key: your-secret-api-key" \
  "http://localhost:3213/portcheck?ip=8.8.8.8&port=53"
```

If not provided, `port` defaults to `443`.

Example response:

```json
{
  "ip": "8.8.8.8",
  "port": 53,
  "reachable": true,
  "status": "open"
}
```

## Query Parameters

| Parameter | Description                                                                         |
| --------- | ----------------------------------------------------------------------------------- |
| `ip`      | Optional IP address to look up. If omitted, the client's IP address is used.        |
| `mode`    | Response mode: default, `ip_only`, `echoip` or `full`.                              |
| `port`    | Optional port to probe. If ommitted, defaults to `443`. Only works in `/portcheck`. |

## Authentication

Every request must include the configured API key:

```text
X-API-Key: your-secret-api-key
```

Requests without a valid key receive:

```text
401 Unauthorized
```
