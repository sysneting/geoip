# Sysneting GeoIP Plugin

A middleware plugin that provides country-based access control using GeoIP databases.

## Features

- Country whitelist/blacklist filtering
- CloudFlare, X-Real-IP, and X-Forwarded-For support
- Automatic database updates
- Connection termination for blocked requests
- IPv4 and IPv6 support

## Configuration

```yaml
middleware:
  geoip:
    apiKey: "your-api-key"  # GeoIP database API key
    dbPath: "/etc/geo/geo.mmdb"  # Database file path
    mode: "blacklist"  # or "whitelist"
    countries: ["US", "CA"]  # ISO country codes
    updateInterval: "24h"  # Update frequency
    trustHeaders: true  # Trust proxy headers
```

## Docker Example

```yaml
services:
  web:
    labels:
      - "traefik.http.middlewares.geo-filter.plugin.geoip.apiKey=${GEOIP_LICENSE}"
      - "traefik.http.middlewares.geo-filter.plugin.geoip.mode=blacklist"
      - "traefik.http.middlewares.geo-filter.plugin.geoip.countries=US,CA"
      - "traefik.http.middlewares.geo-filter.plugin.geoip.trustHeaders=true"
```

## License

MIT License - see LICENSE file for details.