# GeoIP Server

GeoIP Server is a local distribution point for MaxMind GeoIP databases, designed to avoid external service dependencies and authentication headaches. It downloads, updates, and serves both legacy (v1) and modern (v2) GeoIP databases over HTTP.

## Features
- Downloads and updates MaxMind v1 and v2 databases on a schedule
- Serves GeoIP files via HTTP (using Gin)
- Conditional HTTP fetching (If-Modified-Since, Last-Modified)
- Structured logging with log levels and color
- Docker and distroless support
- Healthcheck CLI for container monitoring
- GitHub Actions for CI/CD, CodeQL, and Dependabot

## Environment Variables
- `PORT`: HTTP server port (default: 7502)
- `LOGLEVEL`: Logging level (default: warn)
- `UPDATE_INTERVAL`: Update interval in seconds (default: 86400)
- `GEOIPV1_URLS`: Comma-separated v1 database URLs
- `MAXMIND_ACCOUNT_ID`: MaxMind account ID (required for v2)
- `MAXMIND_LICENSE_KEY`: MaxMind license key (required for v2)

## Usage

### Build and Run Locally
```sh
git clone https://github.com/user00265/geoip-server.git
cd geoip-server
go build -o geoip-server main.go
./geoip-server
```

### Docker
```sh
docker build -t geoip-server .
docker run -e PORT=7502 -p 7502:7502 geoip-server
```

### Docker Compose
```sh
docker-compose up --build
```

### Healthcheck
```sh
./geoip-server healthcheck
```

## HTTP Endpoints

The following endpoints are registered for fetching the GeoIP files obtained
by this daemon.

### GeoIP (v1)
 - `/GeoIP.dat`
 - `/GeoIPv6.dat`
 - `/GeoIPCity.dat`
 - `/GeoIPCityv6.dat`
 - `/GeoIPASNum.dat`
 - `/GeoIPASNumv6.dat`
 - `/GeoIPISP.dat`
 - `/GeoIPISPv6.dat`
 - `/GeoIPOrg.dat`
 - `/GeoIPOrgv6.dat`

### GeoIP (v2)
 - `/GeoLite2-ASN.mmdb`
 - `/GeoLite2-City.mmdb`
 - `/GeoLite2-Country.mmdb`

## MIT License

Copyright (c) 2025 Elisamuel Resto Donate

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.MIT
