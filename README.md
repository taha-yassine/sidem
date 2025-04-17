<h1 align="center">dotenv-manager</h1>

<p align="center">
TUI app that helps simplify the management of `.env` configuration files. It lets you toggle variables on or off and select from multiple predefined values.
</p>

![demo](./demo.gif)

## Installation

### Go Install

```bash
go install github.com/taha-yassine/dotenv-manager@latest
```

### Nix flake

```bash
nix run github.com:taha-yassine/dotenv-manager
```

### Build from source

```bash
git clone https://github.com/taha-yassine/dotenv-manager.git
cd dotenv-manager
go build -o dotenv-manager ./cmd/dotenv-manager
```

## Usage

Run the application from your terminal. By default, it looks for a `.env` file in the current directory. You can optionally specify a path to a different file:

```bash
dotenv-manager [path/to/your/.env]
```

## License

MIT