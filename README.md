# ccache-storage-redis-go

A [ccache remote storage helper](https://ccache.dev/storage-helpers.html) for
Redis/Redis-TLS, written in **Go**.

## Overview

This is a storage helper for [ccache] that enables caching compilation results
on Redis/Redis-TLS servers. It implements the [ccache remote storage helper
protocol].

This project aims to:

1. Provide a high-performance, production-ready Redis(s) ccache storage helper.
2. Serve as an example implementation of a ccache storage helper in **Go**.
   Feel free to use it as a starting point for implementing helpers for other
   storage service protocols.

[ccache]: https://ccache.dev
[ccache remote storage helper protocol]: https://github.com/ccache/ccache/blob/master/doc/remote_storage_helper_spec.md

## Features

- Supports Redis and Redis-TLS
- High-performance concurrent request handling
- Redis context for efficient connection reuse
- Cross-platform: Linux, macOS, Windows
- Bearer token authentication support
- Optional debug logging
- [netrc](https://everything.curl.dev/usingcurl/netrc.html) support


## Installation

The helper should be installed in a [location where ccache searches for helper
programs]. Install it as the name `ccache-storage-redis` for Redis support and/or
`ccache-storage-rediss` for Redis-TLS support.

[location where ccache searches for helper programs]: https://github.com/ccache/ccache/blob/master/doc/manual.adoc#storage-helper-process

### Using a prebuilt binary

Grab a prebuilt binary from
[Releases](https://github.com/ccache/ccache-storage-redis-go/releases) and place
it in a suitable directory as described above. Rename `ccache-storage-redis` to
`ccache-storage-rediss` (or copy or make a symlink) to support TLS.

### Building from source

```bash
# Clone the repository:
git clone https://github.com/ccache/ccache-storage-redis-go
cd ccache-storage-redis-go

# On Windows:
go mod download
go build -ldflags="-s -w" -trimpath -o ccache-storage-redis.exe .

# On Linux/macOS and similar:
make

# Install ccache-storage-redis and a ccache-storage-rediss symlink in /usr/local/bin:
make install

# Install ccache-storage-redis and a ccache-storage-rediss symlink in /example/dir:
make install INSTALL_DIR=/example/dir
```

## Configuration

The helper is configured via ccache's [`remote_storage` configuration]. The
binary is automatically invoked by ccache when needed.

For example:

```bash
# Set the CCACHE_REMOTE_STORAGE environment variable:
export CCACHE_REMOTE_STORAGE="redis://cache.example.com"

# Or set remote_storage in ccache's configuration file:
ccache -o remote_storage="redis://cache.example.com"
```

[`remote_storage` configuration]: https://github.com/ccache/ccache/blob/master/doc/manual.adoc#remote-storage-backends

See also the [Redis storage wiki page] for tips on how to set up a storage server.

[Redis storage wiki page]: https://github.com/ccache/ccache/wiki/Redis-storage

### Configuration attributes

The helper supports the following custom attributes:

- `@bearer-token`: Bearer token for `Authorization` header
- `@use-netrc`: Enable [netrc](https://everything.curl.dev/usingcurl/netrc.html) authentication
- `@netrc-file`: Path to custom [netrc](https://everything.curl.dev/usingcurl/netrc.html) file (implies `@use-netrc`)

Example:

```bash
export CCACHE_REMOTE_STORAGE="redis://cache.example.com @header=Content-Type=application/octet-stream"
```

## Optional debug logging

You can set the `CRSH_LOGFILE` environment variable to enable debug logging to a
file:

```bash
export CRSH_LOGFILE=/path/to/debug.log
```

Note: The helper process is spawned by ccache, so the environment variable must
be set before ccache is invoked.

Warning: The debug log is not redacted and may contain secrets such as bearer
tokens and other credentials. Only enable it for troubleshooting and protect or
delete the log file afterwards.

## Contributing

Contributions are welcome! Please submit pull requests or open issues on GitHub.
