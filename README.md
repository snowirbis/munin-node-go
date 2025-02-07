# Munin Node in Go

This repository contains a lightweight Munin node implementation written in Go. It serves as an alternative to the standard Munin node while maintaining compatibility with the Munin monitoring system.

## Features

- Written in Go for improved performance and reduced dependencies.
- Secure execution of Munin plugins with path validation.
- IP-based access control using regex patterns.
- Environment variable support for plugins via configuration.
- Reads plugin configurations dynamically.
- Implements core Munin node commands: `cap`, `list`, `nodes`, `config`, `fetch`, `version`, `quit`.

## Installation

### Prerequisites

- Go 1.16 or later

### Steps

1. Clone the repository:
   ```sh
   git clone https://github.com/snowirbis/munin-node-go.git
   cd munin-node-go
   ```
2. Build the binary:
   ```sh
   go build -o munin-node
   ```
3. Run the node:
   ```sh
   ./munin-node
   ```

## Configuration

The node reads its configuration from `node.conf`. The following options are supported:

- `host_name`: The hostname of the node.
- `allow`: List of allowed IP addresses or regex patterns.
- `host`: The IP address to listen on (use `*` for all interfaces).
- `port`: The port number to listen on.
- `plugins`: The directory containing Munin plugins.
- `plugins_config`: The file containing plugin environment variable configurations.

### Example `node.conf`

```conf
host_name example-node
allow 192.168.1.0/24
allow 10.0.0.*
host *
port 4949
plugins /etc/munin/plugins
plugins_config /etc/munin/plugin-conf.d
```

## Usage

The node listens for incoming Munin requests and processes the following commands:

- `list` – Lists available plugins.
- `fetch <plugin>` – Retrieves data from a plugin.
- `config <plugin>` – Displays plugin configuration.
- `version` – Displays the Munin node version.
- `nodes` – Returns the node hostname.
- `cap` – Displays supported capabilities.
- `quit` – Closes the connection.

### Example Commands

#### Get Version
```sh
echo -e "version" | nc localhost 4949
```

#### List Available Plugins
```sh
echo -e "list" | nc localhost 4949
```

#### Fetch Data from a Plugin
```sh
echo -e "fetch cpu" | nc localhost 4949
```

## Security

- Plugins must be located within the configured plugin directory.
- Symbolic links are not allowed.
- Access is restricted based on allowed IPs or regex patterns.

## Logging

This implementation uses the `go-simple-log` package for logging. Errors and execution details are logged to standard output.

## License

This project is licensed under the MIT License.

