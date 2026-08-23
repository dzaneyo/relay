# Relay

Relay is a local-first manager for Hosts, Databases, and Notes, with SSH connection support. It provides a web UI and CLI for managing connection details, jump-host routes, and work notes.

## Current Capabilities

- **Web UI**: Manage HOST, DATABASE, and NOTE records with search, favorites, editing, and deletion.
- **CLI**: Supports `list`, `search`, `show`, `connect`, and `web`.
- **HOST routing**: Hosts can connect directly or through routes with multiple jump hosts. `connect` resolves and displays the route plan; SSH session launch is not yet enabled.
- **DATABASE**: Store MySQL and PostgreSQL connection details and invoke the corresponding clients through the CLI.
- **NOTE**: Notes are managed only in the Web UI and are not included in CLI record listings or search results.

## Quick Start

The project uses mise to manage Go, Node.js, and pnpm. After installing and enabling mise, run the following from the project root:

```bash
mise run setup
mise run build
mise run run
```

Then open <http://127.0.0.1:17321>.

For development, you can also start the backend and frontend separately:

```bash
# Terminal 1
mise run dev-backend

# Terminal 2
mise run dev-frontend
```

The development UI is available at <http://127.0.0.1:5173>. By default, data is stored in `~/.relay`; you can also set `RELAY_HOME` to use a different directory.
