# Analytics and metrics history

This note describes the host history graphs and the traffic analytics panel. It sits on the v0.2.0-dev foundation in [phase-1-stacks-monitoring.md](phase-1-stacks-monitoring.md). The panel still reads true state from `/proc`. History is a small set of SQLite rollups, not a second monitoring stack.

There is no Prometheus, no ClickHouse, and no background exporter. The API process samples the host and records requests it already handled.

## Routes

Both routes require a session. Query values other than `range` and `project` are ignored. A bad range or project id returns 400 and does not echo the input.

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/api/metrics/host` | Current host snapshot. Unchanged |
| `GET` | `/api/metrics/host/series?range=` | CPU, memory, disk, and network over time |
| `GET` | `/api/analytics/summary?range=&project=` | Requests, visitors, bandwidth, errors, p95, and breakdowns |

`range` is `live`, `24h`, `7d`, `30d`, or `month` (`months` is accepted as the same window). `project` is optional. Empty means every project. A project id is a short token: letters, digits, `.`, `_`, and `-`.

| Range | Host buckets | Traffic buckets | Window |
| --- | --- | --- | --- |
| Live | 15 seconds | 1 minute | Last 15 minutes |
| 24h | 1 minute | 1 minute | Last 24 hours |
| 7d | 1 hour | 1 hour | Last 7 days |
| 30d | 1 hour | 1 hour | Last 30 days |
| Months | 1 day, grouped by calendar month | 1 day, grouped by calendar month | Current month and the 11 before it |

The disk series follows the mount with the largest total size. Network rates are the sum of interface rates. CPU stays null until a second host sample exists, the same rule as the live snapshot.

## Retention

Older buckets are deleted. They are not archived.

| Bucket | Kept |
| --- | --- |
| 15 seconds (host only) | 2 hours |
| 1 minute | 48 hours |
| 1 hour | 40 days |
| 1 day | 400 days |

Daily visitor hashes follow the same day limit. Hourly visitor hashes follow the minute limit. A write prunes at most once a minute.

Each sample updates the 15-second, minute, hour, and day buckets in place. Counters are summed. Gauges are averaged by the number of samples in the bucket. Months are that daily average (or daily sum, for requests) grouped in the query. The database does not grow with the number of requests beyond the cardinality caps below.

## Traffic collection

`analytics.Recorder.Record` is the ingest hook. It accepts one request event and writes rollups. Two callers ship with the panel:

1. In-process middleware records `/api/` traffic after the response is written. That is real panel traffic, including health checks. It is the source the Analytics page shows when no log file is configured.
2. An optional Combined Log Format tail. It is off unless `ALTAS_ACCESS_LOG` is an absolute path to an existing file. Paths with `..` are refused. The tail starts at the end of the file, so old lines are not imported. Those events are stored as project `proxy` and have no latency, so they do not move p95.

A reverse proxy can call `Record` from a small in-process adapter later. There is no public ingest URL. The panel does not seed demo traffic. Tests insert rows themselves. An empty database returns zero totals and empty lists.

An event stores:

- project, from `X-Altas-Project` when that header is a valid id, otherwise `panel` for middleware
- application, from `X-Altas-App`, otherwise `panel`
- HTTP host, path, status, response bytes, and latency when known
- device class from the User-Agent: `desktop`, `mobile`, `tablet`, `bot`, or `unknown`
- country and city only when a local GeoIP database returns them

Errors are responses with status 500 or higher. p95 is the upper edge of a fixed histogram: 5, 10, 25, 50, 100, 250, 500, 1000, 2500, 5000, and 10000 milliseconds. It is empty when the window has no timed requests.

Paths drop the query string and fragment, and drop `.` and `..` segments, before insert. A day bucket keeps at most 200 paths, 100 hosts, and 100 applications. Further names roll into `other`.

`project` on the summary filters the rollups. Host, application, and path tables are the breakdowns. There is no free-form search.

## IP privacy

The raw client address is not stored.

Before lookup or hashing, IPv4 is truncated to `/24` and IPv6 to `/48`. The GeoIP lookup uses that truncated address. If it misses, country and city stay empty. The panel does not guess a place.

The long-term visitor key is the first 8 bytes of SHA-256 over a per-database salt, the UTC date, and the truncated address, hex-encoded. The salt is created once and kept in `analytics_meta`. The truncated address itself is not written. The hash changes the next UTC day, so a person who returns tomorrow is a new visitor. That is deliberate: the goal is not to keep a stable identifier.

Query strings are not stored, so reset tokens and other secrets in a URL do not land in the rollup tables.

## GeoIP

`ALTAS_GEOIP_DB` may point at a MaxMind City or Country `.mmdb` file. The path must be absolute and must not contain `..`. A missing, relative, or unreadable file disables geo and the API reports `geo_configured: false`. The Analytics page says the fields are empty. It does not fill them with placeholders.

No GeoIP database is shipped in the repository.

## Operator settings

| Variable | Default | Purpose |
| --- | --- | --- |
| `ALTAS_METRICS_INTERVAL` | `15s` | Host sample interval. Values below 5s or above 5m fall back to 15s |
| `ALTAS_GEOIP_DB` | empty | Optional MaxMind `.mmdb`. Empty leaves locations blank |
| `ALTAS_ACCESS_LOG` | empty | Optional Combined log to tail. Empty leaves the tail off |

Host proc paths, the Docker probe, and the command allowlist are unchanged.
