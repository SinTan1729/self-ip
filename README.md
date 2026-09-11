# Self IP

A small self-hosted IP geolocation and port checking API written in Go.

The server determines the client's IP address or accepts an IP address through
a query parameter, then returns geolocation and ASN information using
[MaxMind GeoLite2](https://dev.maxmind.com/geoip/geolite2-free-geolocation-data) databases.

It can also check for whether a port is open on a given IP address.

It is highly recommended that you use it behind a reverse proxy e.g.
[Caddy](https://caddyserver.com).

**Make sure you add a `SELF_IP_TRUSTED_PROXIES` environment variable with your reverse proxy's
IP subnet, so that `self-ip` gets the correct client IP.**

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
Then, they're checked once a day around 7am. When found, new releases
of the databases are automatically downloaded.

## Deployment

The recommended method of installation is using containers e.g. Docker or Podman.
Example `docker compose` and `podman quadlet` files are provided in the
[`deploy`](./deploy) directory.

**Remember to change the API Key hash before deploying.** The provided hash is gibberish.

## Environment Variables

| Variable name                 | Description                                                                                                                                                                                                                 |
| ----------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `SELF_IP_API_KEY`             | Argon2 encrypted API Key. (Mandatory)                                                                                                                                                                                       |
| `SELF_IP_TRUSTED_PROXIES`     | List of trusted proxy IPs/subnet. If the request comes from one of these, the proxy headers will be used to figure out the client's real IP. Useful when the server if behind a reverse proxy, which is highly recommended. |
| `SELF_IP_LISTEN_ADDR`         | The address the server listens to. Defaults to empty i.e. all addresses.                                                                                                                                                    |
| `SELF_IP_LISTEN_PORT`         | The port the server listens to. Defaults to `3213`.                                                                                                                                                                         |
| `SELF_IP_ENABLE_PORT_CHECKER` | Enables port checker when set to `True`.                                                                                                                                                                                    |
| `SELF_IP_ENABLE_HOSTNAME`     | Enables hostname resolution in full mode when set to `True`.                                                                                                                                                                |

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
The path `/portcheck` is used for `port` probing. There's a `/healthz` path listening exclusively
to `127.0.0.1:1729`. Only use it for internal healthchecks.
Any other paths will return:

```text
404 Page Not Found
```

## Response Modes

### Default mode

The default response.

```bash
curl \
  -H "X-API-Key: your-secret-api-key" \
  "http://localhost:3213?ip=8.8.8.8"
```

The default response returns a JSON representation in the following format.

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
  "tz": "Example/Timezone",
  "org": "A12345 Example Org."
}
```

### IP-only mode

Return only the queried IP address. It disregards the provided `ip` query string.

```bash
curl \
  -H "X-API-Key: your-secret-api-key" \
  "http://localhost:3213?ip=8.8.8.8&mode=ip_only"
```

The response content type is `text/plain` e.g. `1.2.3.4`.

### Short mode

Return only essential information.

```bash
curl \
  -H "X-API-Key: your-secret-api-key" \
  "http://localhost:3213?ip=8.8.8.8&mode=short"
```

The default response returns a flat JSON representation in the following format.

```json
{
  "ip": "1.2.3.4",
  "city": "Example City",
  "region": "Example Region",
  "country": "Example Country",
  "tz": "Example/Timezone"
}
```

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

Return the complete geolocation record.

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

_Note: Private IPs, loopback IPs etc. are automatically blocked to prevent abuse._

It returns information in the following format.

```json
{
  "ip": "8.8.8.8",
  "port": 53,
  "reachable": true,
  "status": "open"
}
```

## Query Parameters

| Parameter | Description                                                                                             |
| --------- | ------------------------------------------------------------------------------------------------------- |
| `ip`      | Optional IP address to look up. If omitted, the client's IP address is used. Ignored in `ip_only` mode. |
| `mode`    | Response mode: default, `ip_only`, `echoip`, `short` or `full`.                                         |
| `port`    | Optional port to probe. If ommitted, defaults to `443`. Only works in `/portcheck`.                     |

## Authentication

Every request must include the configured API key:

```text
X-API-Key: your-secret-api-key
```

Requests without a valid key receive:

```text
401 Unauthorized
```
