# DevKit (brew + local app) — starter pack

A lightweight developer toolbox you can ship via **Homebrew**. Users install with `brew tap yourorg/devkit && brew install devkit` and run either as a CLI or a background local service with `brew services`.

---

## Goals
- Bundled utilities: JSON/XML formatters, diffs, converters, markup preview.
- Single signed binary for macOS (Apple Silicon + Intel) running a small HTTP service on `localhost`.
- CLI UX for piping/one-off ops; optional minimal web UI (Tauri/Electron) hitting the same local API.
- Distributed via a custom Homebrew tap. Optional: run as a login service using `brew services`.

---

## High-level architecture
```
+---------------------------+      +-----------------+
| CLI (Cobra)               |      | Optional GUI    |
| devkit format|diff|start  |      | (Tauri/Electron)|
+---------------------------+      +-----------------+
               |                           |
               v                           v
         +---------------------------------------+
         | Local HTTP Service (Go)               |
         |  - /v1/format/json, /xml              |
         |  - /v1/diff/json,  /xml              |
         |  - /v1/convert/...                    |
         |  - /v1/preview/markup                 |
         +---------------------------------------+
               |
               v
        +-----------------+
        | Core libs (Go)  |
        | json, xml, diff |
        +-----------------+
```

Why Go?
- Great for static single-binary distribution; easy Homebrew formula; fast start; good stdlib for JSON/XML; cross-compile universal2 via `-buildmode=pie` + `lipo`.

---

## Repo layout
```
devkit/
  cmd/devkit/main.go         # CLI + HTTP server entrypoint (Cobra)
  internal/format/json.go    # JSON format/validate
  internal/format/xml.go     # XML format/validate
  internal/diff/jsondiff.go  # JSON diff
  internal/diff/xmldiff.go   # XML diff
  internal/preview/markup.go # Markdown/Asciidoc -> HTML
  webui/                     # (optional) Tauri/Electron app hitting localhost
  Makefile                   # build, package, release
  brew/devkit.rb             # Homebrew formula (for your tap repo)
  README.md
```

---

## CLI & HTTP server (Go + Cobra)

> **Single binary** supports both CLI one-shots and running a local API (`devkit start`).

```go
// cmd/devkit/main.go
package main

import (
    "bytes"
    "encoding/json"
    "encoding/xml"
    "fmt"
    "log"
    "net/http"
    "os"
    "strings"

    "github.com/spf13/cobra"
)

var (
    listenAddr string
)

func main() {
    root := &cobra.Command{Use: "devkit", Short: "Developer toolbox (CLI + local API)"}

    // start server
    startCmd := &cobra.Command{Use: "start", Short: "Start local HTTP service", RunE: func(cmd *cobra.Command, args []string) error {
        mux := http.NewServeMux()

        // JSON format
        mux.HandleFunc("/v1/format/json", func(w http.ResponseWriter, r *http.Request) {
            defer r.Body.Close()
            var raw any
            dec := json.NewDecoder(r.Body)
            dec.UseNumber()
            if err := dec.Decode(&raw); err != nil {
                http.Error(w, fmt.Sprintf("invalid JSON: %v", err), http.StatusBadRequest)
                return
            }
            buf := &bytes.Buffer{}
            enc := json.NewEncoder(buf)
            enc.SetIndent("", "  ")
            enc.SetEscapeHTML(false)
            if err := enc.Encode(raw); err != nil {
                http.Error(w, err.Error(), http.StatusInternalServerError)
                return
            }
            w.Header().Set("Content-Type", "application/json; charset=utf-8")
            w.Write(buf.Bytes())
        })

        // XML format
        mux.HandleFunc("/v1/format/xml", func(w http.ResponseWriter, r *http.Request) {
            defer r.Body.Close()
            // naive pretty-print: decode/encode to indent (sufficient for MVP)
            b, err := io.ReadAll(r.Body)
            if err != nil { http.Error(w, err.Error(), 500); return }
            var v any
            if err := xml.Unmarshal(b, &v); err != nil {
                http.Error(w, fmt.Sprintf("invalid XML: %v", err), http.StatusBadRequest)
                return
            }
            out, err := xml.MarshalIndent(v, "", "  ")
            if err != nil { http.Error(w, err.Error(), 500); return }
            w.Header().Set("Content-Type", "application/xml; charset=utf-8")
            w.Write([]byte(xml.Header))
            w.Write(out)
        })

        // JSON diff (very simple structural diff for MVP)
        mux.HandleFunc("/v1/diff/json", func(w http.ResponseWriter, r *http.Request) {
            type payload struct{ A json.RawMessage `json:"a"`; B json.RawMessage `json:"b"` }
            var p payload
            if err := json.NewDecoder(r.Body).Decode(&p); err != nil { http.Error(w, "bad payload", 400); return }
            var a, b any
            if err := json.Unmarshal(p.A, &a); err != nil { http.Error(w, "bad a", 400); return }
            if err := json.Unmarshal(p.B, &b); err != nil { http.Error(w, "bad b", 400); return }
            diff := naiveJSONDiff(a, b)
            w.Header().Set("Content-Type", "application/json")
            json.NewEncoder(w).Encode(diff)
        })

        // Markup preview (Markdown -> HTML)
        mux.HandleFunc("/v1/preview/markdown", func(w http.ResponseWriter, r *http.Request) {
            b, _ := io.ReadAll(r.Body)
            html := renderMarkdown(string(b))
            w.Header().Set("Content-Type", "text/html; charset=utf-8")
            w.Write([]byte(html))
        })

        log.Printf("devkit listening on %s", listenAddr)
        return http.ListenAndServe(listenAddr, secureHeaders(mux))
    }})
    startCmd.Flags().StringVar(&listenAddr, "addr", "127.0.0.1:7123", "listen address")

    // CLI format subcommands (stdin -> stdout)
    formatCmd := &cobra.Command{Use: "format", Short: "Format data"}
    formatJSON := &cobra.Command{Use: "json", RunE: func(cmd *cobra.Command, args []string) error {
        in, _ := io.ReadAll(os.Stdin)
        var v any
        dec := json.NewDecoder(bytes.NewReader(in))
        dec.UseNumber()
        if err := dec.Decode(&v); err != nil { return err }
        enc := json.NewEncoder(os.Stdout); enc.SetIndent("", "  "); enc.SetEscapeHTML(false)
        return enc.Encode(v)
    }}
    formatXML := &cobra.Command{Use: "xml", RunE: func(cmd *cobra.Command, args []string) error {
        in, _ := io.ReadAll(os.Stdin)
        var v any
        if err := xml.Unmarshal(in, &v); err != nil { return err }
        out, err := xml.MarshalIndent(v, "", "  ")
        if err != nil { return err }
        fmt.Println(xml.Header + string(out))
        return nil
    }}
    formatCmd.AddCommand(formatJSON, formatXML)

    root.AddCommand(startCmd, formatCmd)
    if err := root.Execute(); err != nil { os.Exit(1) }
}

// --- helpers below ---

// naiveJSONDiff produces a basic diff {onlyInA, onlyInB, changed}
func naiveJSONDiff(a, b any) map[string]any { /* impl left minimal for brevity */ return map[string]any{"todo": true} }

func renderMarkdown(md string) string {
    // Placeholder: swap in goldmark/commonmark lib later. MVP very basic.
    return "<pre>" + html.EscapeString(md) + "</pre>"
}

func secureHeaders(next http.Handler) http.Handler { return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request){
    w.Header().Set("X-Content-Type-Options", "nosniff")
    w.Header().Set("X-Frame-Options", "DENY")
    w.Header().Set("Referrer-Policy", "no-referrer")
    next.ServeHTTP(w, r)
})}
```

> For production, replace placeholders:
> - Use a real Markdown renderer (e.g., `github.com/yuin/goldmark`).
> - Use a real JSON diff lib (e.g., `github.com/wI2L/jsondiff`).
> - Add XML diff (e.g., normalize to DOM and compare node-by-node).

---

## HTTP API (MVP)

```
POST /v1/format/json       body: raw JSON         -> 200 formatted JSON
POST /v1/format/xml        body: raw XML          -> 200 formatted XML
POST /v1/diff/json         body: {"a":...,"b":...}-> 200 diff JSON
POST /v1/preview/markdown  body: text/markdown    -> 200 text/html
```

Auth: none (localhost-only). Consider a random token in `~/Library/Application Support/devkit/config.json` and require `Authorization: Bearer <token>` for extra safety.

---

## Makefile (universal macOS build)

```make
APP=devkit
VERSION?=$(shell git describe --tags --always --dirty)

.PHONY: build build-macos package

build:
	GOOS=darwin GOARCH=arm64 go build -trimpath -ldflags "-s -w -X main.version=$(VERSION)" -o dist/$(APP)-darwin-arm64 ./cmd/devkit
	GOOS=darwin GOARCH=amd64 go build -trimpath -ldflags "-s -w -X main.version=$(VERSION)" -o dist/$(APP)-darwin-amd64 ./cmd/devkit
	lipo -create -output dist/$(APP) dist/$(APP)-darwin-arm64 dist/$(APP)-darwin-amd64

package: build
	shasum -a 256 dist/$(APP) > dist/$(APP).sha256
```

Codesigning & notarization (optional but recommended):
```
codesign --force --options runtime --sign "Developer ID Application: Your Org" dist/devkit
xcrun notarytool submit dist/devkit --keychain-profile "AC_NOTARY" --wait
```

---

## Homebrew formula (tap)

Place this in a separate repo users will `tap`, e.g., `yourorg/homebrew-devkit`. File: `Formula/devkit.rb`.

```ruby
class Devkit < Formula
  desc "Developer toolbox (CLI + local API)"
  homepage "https://github.com/yourorg/devkit"
  version "0.1.0"
  url "https://github.com/yourorg/devkit/releases/download/v0.1.0/devkit"
  sha256 "<fill-from-dist/devkit.sha256>"

  def install
    bin.install "devkit"
  end

  service do
    run [opt_bin/"devkit", "start", "--addr", "127.0.0.1:7123"]
    keep_alive true
    log_path var/"log/devkit.log"
    error_log_path var/"log/devkit.err.log"
  end
end
```

Usage:
```
brew tap yourorg/devkit
brew install devkit
# CLI
cat data.json | devkit format json
# Service
brew services start devkit
curl -s http://127.0.0.1:7123/v1/format/json -d '{"x":1}'
```

---

## Optional GUI (Tauri)

- Create a tiny SPA (Svelte/React) that calls `http://127.0.0.1:7123` endpoints.
- Package with Tauri for native macOS app (fast, small). Ship separately via DMG or via Brew Cask.
- Cask example (in the same tap) for `devkit-gui` that bundles the Tauri app.

---

## Security & sandboxing
- Bind to `127.0.0.1` by default, not `0.0.0.0`.
- Optionally require a random bearer token.
- Consider per-request size limits and timeouts; deny directory traversal for any file endpoints.

---

## Roadmap
- [ ] Real JSON diff & XML diff libraries
- [ ] More converters (YAML↔JSON, TOML↔JSON)
- [ ] Markup engines (CommonMark, Asciidoc)
- [ ] Configurable hotkeys / OS service integration
- [ ] Plugin model (exec adapters): discover `~/.config/devkit/plugins/*.toml`
- [ ] Auto-update check (GitHub Releases)

---

## Quickstart for you
1. Copy the `cmd/devkit/main.go` and `Makefile` into a new repo and run `make package` on macOS (Go 1.22+).
2. Create `yourorg/homebrew-devkit` with the formula above; commit & push.
3. Create a GitHub Release with the `dist/devkit` binary and `.sha256`.
4. `brew tap yourorg/devkit && brew install devkit` and test on a clean machine.

Need me to expand any piece (e.g., real JSON diff, token auth, or a Tauri skeleton)? I can drop in fuller code next.

