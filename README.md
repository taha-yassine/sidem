<h1 align="center">sidem - <u>si</u>mple <u>d</u>otenv <u>m</u>anager</h1>

<p align="center">
TUI app that helps simplify the management of <i>.env</i> configuration files.<br>
It lets you toggle variables on or off and select from multiple predefined values.
</p>

![demo](./demo.gif)

## Setup

### Go Install

```bash
go install github.com/taha-yassine/sidem/cmd/sidem@latest
```

### Nix flake

```bash
nix run github:taha-yassine/sidem
```

### Build from source

```bash
git clone https://github.com/taha-yassine/sidem.git
cd sidem
go build -o sidem ./cmd/sidem
```

## Usage

Run the application from your terminal. By default, it looks for a `.env` file in the current directory. You can optionally specify a path to a different file:

```bash
sidem [path/to/your/.env]
```

## License

MIT
