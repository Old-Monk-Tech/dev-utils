# DevKit - Developer Toolbox

A lightweight developer toolbox offering CLI tools and a local web UI for common developer tasks.

## Features
- JSON/XML formatter and validator
- JSON and text diff
- Markdown preview
- JWT decoder, hashing (md5/sha1/sha256)
- Base64/URL/Hex encode/decode
- UUID generator, timestamp/color/text utilities
- Favorites and Most Used quick-access on the welcome page

## Install

### From Source (local)
```bash
make build-simple
./dist/devkit --help
```

### Homebrew (tap this repo, build from source)
```bash
brew tap Old-Monk-Tech/dev-utils
brew install --HEAD Old-Monk-Tech/dev-utils/devkit
```

If you prefer a separate tap with bottled releases, use a repo like `Old-Monk-Tech/homebrew-devkit` and publish release artifacts; then install with `brew tap Old-Monk-Tech/homebrew-devkit && brew install devkit`.

## Usage

### Start Web Server
```bash
devkit start                 # Default 127.0.0.1:7123
devkit start --port 9109     # Custom port
devkit start --addr 0.0.0.0:8080
```
Then open http://localhost:9109.

### CLI Examples
```bash
echo '{"name":"John","age":30}' | devkit format json
echo '<root><item>value</item></root>' | devkit format xml
```

## Development
```bash
make build-simple            # Build for current platform
make build                   # Create universal macOS binary
make package                 # Build + checksum
make dev                     # Clean build and run on port 9109
```

## Homebrew Formula in this repo
- `Formula/devkit.rb` supports tapping this repo and building from source via `--HEAD`.

## License
MIT
