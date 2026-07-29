# ccache-storage-redis

A [ccache remote storage helper](https://ccache.dev/storage-helpers.html) for
Redis.

## Overview

This is a storage helper for [ccache] that enables caching compilation results
on Redis servers. It implements the [ccache remote storage helper protocol].

[ccache]: https://ccache.dev
[ccache remote storage helper protocol]: https://github.com/ccache/ccache/blob/master/doc/remote_storage_helper_spec.md

## Features

- Supports the Redis protocol over unencrypted or TLS/SSL network connections,
  as well as over unencrypted over local Unix domain socket connections
- Cross-platform: Linux, macOS, Windows
- Optional debug logging


## Installation

The helper should be installed in a [location where ccache searches for helper
programs]. Install it as:

- `ccache-storage-redis` for unencrypted Redis
- `ccache-storage-rediss` for Redis TLS/SSL
- `ccache-storage-redis+unix` for Redis on local Unix domain socket

[location where ccache searches for helper programs]: https://github.com/ccache/ccache/blob/master/doc/manual.adoc#storage-helper-process

### URL formats

- Unencrypted: `redis://[[USERNAME:]PASSWORD@]HOST[:PORT][/DBNUMBER]`
- TLS/SSL-encrypted: `rediss://[[USERNAME:]PASSWORD@]HOST[:PORT][/DBNUMBER]`
- Unix domain socket: `redis+unix:SOCKET_PATH[?db=DBNUMBER]` or `redis+unix://[[USERNAME:]PASSWORD@localhost]SOCKET_PATH[?db=DBNUMBER]`

### Using a prebuilt binary

Grab a prebuilt binary from
[Releases](https://github.com/ccache/ccache-storage-redis/releases) and place it
in a suitable directory as described above.

### Building from source

```bash
# Clone the repository:
git clone https://github.com/ccache/ccache-storage-redis
cd ccache-storage-redis

# On Windows:
go mod download
go build -ldflags="-s -w" -trimpath -o ccache-storage-redis.exe .

# On Linux/macOS and similar:
make

# Install ccache-storage-redis plus ccache-storage-rediss and
# ccache-storage-redis+unix symlinks in /usr/local/bin:
make install

# Install ccache-storage-redis  plus ccache-storage-rediss and
# ccache-storage-redis+unix symlinks in /example/dir:
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

### Configuration

Example ccache configuration:

```
remote_storage = redis://cache.example.com
```

Or as an environment variable:

```bash
export CCACHE_REMOTE_STORAGE="redis://cache.example.com"
```

## Optional debug logging

You can set the `CRSH_LOGFILE` environment variable to enable debug logging to a
file:

```bash
export CRSH_LOGFILE=/path/to/debug.log
```

Note: The helper process is spawned by ccache, so the environment variable must
be set before ccache is invoked.

Warning: The debug log is not redacted and may contain secrets such as
credentials. Only enable it for troubleshooting and protect or delete the log
file afterwards.

## Contributing

Contributions are welcome! Please submit pull requests or open issues on GitHub.
