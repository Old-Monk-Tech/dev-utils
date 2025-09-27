package main

import (
	"bytes"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

var (
	listenAddr string
	port       int
)

func main() {
	root := &cobra.Command{Use: "devkit", Short: "Developer toolbox (CLI + local API)"}

	// start server
	startCmd := &cobra.Command{
		Use:   "start",
		Short: "Start local HTTP service",
		RunE: func(cmd *cobra.Command, args []string) error {
			if port != 0 {
				listenAddr = "127.0.0.1:" + strconv.Itoa(port)
			}

			mux := http.NewServeMux()

			// Serve web UI
			mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/" {
					w.Header().Set("Content-Type", "text/html; charset=utf-8")
					w.Write([]byte(webUI))
					return
				}
				http.NotFound(w, r)
			})

			// === EDITING & INSPECTION ===

			// JSON format
			mux.HandleFunc("/v1/format/json", handleJSONFormat)

			// XML format
			mux.HandleFunc("/v1/format/xml", handleXMLFormat)

			// JSON diff
			mux.HandleFunc("/v1/diff/json", handleJSONDiff)

			// Text diff
			mux.HandleFunc("/v1/diff/text", handleTextDiff)

			// Markdown preview
			mux.HandleFunc("/v1/preview/markdown", handleMarkdownPreview)

			// Regex test
			mux.HandleFunc("/v1/regex/test", handleRegexTest)

			// === SECURITY & ENCODINGS ===

			// JWT decode
			mux.HandleFunc("/v1/jwt/decode", handleJWTDecode)

			// Hash generation
			mux.HandleFunc("/v1/hash/generate", handleHashGenerate)

			// UUID generation
			mux.HandleFunc("/v1/uuid/generate", handleUUIDGenerate)

			// Base64 encode/decode
			mux.HandleFunc("/v1/base64/encode", handleBase64Encode)
			mux.HandleFunc("/v1/base64/decode", handleBase64Decode)

			// URL encode/decode
			mux.HandleFunc("/v1/url/encode", handleURLEncode)
			mux.HandleFunc("/v1/url/decode", handleURLDecode)

			// Hex encode/decode
			mux.HandleFunc("/v1/hex/encode", handleHexEncode)
			mux.HandleFunc("/v1/hex/decode", handleHexDecode)

			// === UTILITIES ===

			// Timestamp conversion
			mux.HandleFunc("/v1/timestamp/convert", handleTimestampConvert)

			// Color conversion
			mux.HandleFunc("/v1/color/convert", handleColorConvert)

			// Lorem ipsum generator
			mux.HandleFunc("/v1/lorem/generate", handleLoremGenerate)

			// Text transformations
			mux.HandleFunc("/v1/text/transform", handleTextTransform)

			log.Printf("devkit listening on %s", listenAddr)
			log.Printf("Web UI available at http://%s", listenAddr)
			return http.ListenAndServe(listenAddr, secureHeaders(mux))
		},
	}
	startCmd.Flags().StringVar(&listenAddr, "addr", "127.0.0.1:7123", "listen address")
	startCmd.Flags().IntVar(&port, "port", 0, "listen port (overrides addr)")

	// CLI format subcommands
	formatCmd := &cobra.Command{Use: "format", Short: "Format data"}
	formatJSON := &cobra.Command{
		Use: "json",
		RunE: func(cmd *cobra.Command, args []string) error {
			in, _ := io.ReadAll(os.Stdin)
			var v any
			dec := json.NewDecoder(bytes.NewReader(in))
			dec.UseNumber()
			if err := dec.Decode(&v); err != nil {
				return err
			}
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			enc.SetEscapeHTML(false)
			return enc.Encode(v)
		},
	}
	formatXML := &cobra.Command{
		Use: "xml",
		RunE: func(cmd *cobra.Command, args []string) error {
			in, _ := io.ReadAll(os.Stdin)
			var v any
			if err := xml.Unmarshal(in, &v); err != nil {
				return err
			}
			out, err := xml.MarshalIndent(v, "", "  ")
			if err != nil {
				return err
			}
			fmt.Println(xml.Header + string(out))
			return nil
		},
	}
	formatCmd.AddCommand(formatJSON, formatXML)

	root.AddCommand(startCmd, formatCmd)
	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

// === EDITING & INSPECTION HANDLERS ===

func handleJSONFormat(w http.ResponseWriter, r *http.Request) {
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
}

func handleXMLFormat(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	b, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	var v any
	if err := xml.Unmarshal(b, &v); err != nil {
		http.Error(w, fmt.Sprintf("invalid XML: %v", err), http.StatusBadRequest)
		return
	}
	out, err := xml.MarshalIndent(v, "", "  ")
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.Write([]byte(xml.Header))
	w.Write(out)
}

func handleJSONDiff(w http.ResponseWriter, r *http.Request) {
	type payload struct {
		A json.RawMessage `json:"a"`
		B json.RawMessage `json:"b"`
	}
	var p payload
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		http.Error(w, "bad payload", 400)
		return
	}

	aStr := strings.TrimSpace(string(p.A))
	bStr := strings.TrimSpace(string(p.B))

	if aStr == bStr {
		json.NewEncoder(w).Encode(map[string]any{"equal": true, "message": "Objects are identical"})
		return
	}

	// Simple line-by-line diff
	aLines := strings.Split(aStr, "\n")
	bLines := strings.Split(bStr, "\n")

	diff := map[string]any{
		"equal": false,
		"a_lines": len(aLines),
		"b_lines": len(bLines),
		"changes": generateSimpleDiff(aLines, bLines),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(diff)
}

func handleTextDiff(w http.ResponseWriter, r *http.Request) {
	type payload struct {
		A string `json:"a"`
		B string `json:"b"`
	}
	var p payload
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		http.Error(w, "bad payload", 400)
		return
	}

	if p.A == p.B {
		json.NewEncoder(w).Encode(map[string]any{"equal": true, "message": "Texts are identical"})
		return
	}

	aLines := strings.Split(p.A, "\n")
	bLines := strings.Split(p.B, "\n")

	diff := map[string]any{
		"equal": false,
		"a_lines": len(aLines),
		"b_lines": len(bLines),
		"changes": generateSimpleDiff(aLines, bLines),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(diff)
}

func handleMarkdownPreview(w http.ResponseWriter, r *http.Request) {
	b, _ := io.ReadAll(r.Body)
	html := renderMarkdown(string(b))
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(html))
}

func handleRegexTest(w http.ResponseWriter, r *http.Request) {
	type payload struct {
		Pattern string `json:"pattern"`
		Text    string `json:"text"`
		Flags   string `json:"flags"`
	}
	var p payload
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		http.Error(w, "bad payload", 400)
		return
	}

	re, err := regexp.Compile(p.Pattern)
	if err != nil {
		json.NewEncoder(w).Encode(map[string]any{
			"valid": false,
			"error": err.Error(),
		})
		return
	}

	matches := re.FindAllStringSubmatch(p.Text, -1)
	indices := re.FindAllStringSubmatchIndex(p.Text, -1)

	result := map[string]any{
		"valid": true,
		"matches": matches,
		"indices": indices,
		"count": len(matches),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// === SECURITY & ENCODINGS HANDLERS ===

func handleJWTDecode(w http.ResponseWriter, r *http.Request) {
	type payload struct {
		Token string `json:"token"`
	}
	var p payload
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		http.Error(w, "bad payload", 400)
		return
	}

	parts := strings.Split(p.Token, ".")
	if len(parts) != 3 {
		json.NewEncoder(w).Encode(map[string]any{
			"valid": false,
			"error": "Invalid JWT format",
		})
		return
	}

	// Decode header
	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		json.NewEncoder(w).Encode(map[string]any{
			"valid": false,
			"error": "Invalid header encoding",
		})
		return
	}

	// Decode payload
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		json.NewEncoder(w).Encode(map[string]any{
			"valid": false,
			"error": "Invalid payload encoding",
		})
		return
	}

	var header, claims map[string]any
	json.Unmarshal(headerBytes, &header)
	json.Unmarshal(payloadBytes, &claims)

	// Check expiry if present
	var expired bool
	if exp, ok := claims["exp"].(float64); ok {
		expired = time.Now().Unix() > int64(exp)
	}

	result := map[string]any{
		"valid": true,
		"header": header,
		"payload": claims,
		"signature": parts[2],
		"expired": expired,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func handleHashGenerate(w http.ResponseWriter, r *http.Request) {
	type payload struct {
		Text string `json:"text"`
		Type string `json:"type"`
	}
	var p payload
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		http.Error(w, "bad payload", 400)
		return
	}

	data := []byte(p.Text)
	var hash string

	switch p.Type {
	case "md5":
		h := md5.Sum(data)
		hash = hex.EncodeToString(h[:])
	case "sha1":
		h := sha1.Sum(data)
		hash = hex.EncodeToString(h[:])
	case "sha256":
		h := sha256.Sum256(data)
		hash = hex.EncodeToString(h[:])
	default:
		http.Error(w, "unsupported hash type", 400)
		return
	}

	result := map[string]any{
		"input": p.Text,
		"type": p.Type,
		"hash": hash,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func handleUUIDGenerate(w http.ResponseWriter, r *http.Request) {
	type payload struct {
		Version string `json:"version"`
		Count   int    `json:"count"`
	}
	var p payload
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		p.Version = "v4"
		p.Count = 1
	}

	if p.Count <= 0 || p.Count > 100 {
		p.Count = 1
	}

	var uuids []string
	for i := 0; i < p.Count; i++ {
		switch p.Version {
		case "v1":
			uuids = append(uuids, uuid.Must(uuid.NewUUID()).String())
		case "v4":
			uuids = append(uuids, uuid.New().String())
		default:
			uuids = append(uuids, uuid.New().String())
		}
	}

	result := map[string]any{
		"version": p.Version,
		"count": len(uuids),
		"uuids": uuids,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func handleBase64Encode(w http.ResponseWriter, r *http.Request) {
	type payload struct {
		Text string `json:"text"`
	}
	var p payload
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		http.Error(w, "bad payload", 400)
		return
	}

	encoded := base64.StdEncoding.EncodeToString([]byte(p.Text))

	result := map[string]any{
		"input": p.Text,
		"encoded": encoded,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func handleBase64Decode(w http.ResponseWriter, r *http.Request) {
	type payload struct {
		Text string `json:"text"`
	}
	var p payload
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		http.Error(w, "bad payload", 400)
		return
	}

	decoded, err := base64.StdEncoding.DecodeString(p.Text)
	if err != nil {
		json.NewEncoder(w).Encode(map[string]any{
			"valid": false,
			"error": err.Error(),
		})
		return
	}

	result := map[string]any{
		"input": p.Text,
		"decoded": string(decoded),
		"valid": true,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func handleURLEncode(w http.ResponseWriter, r *http.Request) {
	type payload struct {
		Text string `json:"text"`
	}
	var p payload
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		http.Error(w, "bad payload", 400)
		return
	}

	encoded := url.QueryEscape(p.Text)

	result := map[string]any{
		"input": p.Text,
		"encoded": encoded,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func handleURLDecode(w http.ResponseWriter, r *http.Request) {
	type payload struct {
		Text string `json:"text"`
	}
	var p payload
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		http.Error(w, "bad payload", 400)
		return
	}

	decoded, err := url.QueryUnescape(p.Text)
	if err != nil {
		json.NewEncoder(w).Encode(map[string]any{
			"valid": false,
			"error": err.Error(),
		})
		return
	}

	result := map[string]any{
		"input": p.Text,
		"decoded": decoded,
		"valid": true,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func handleHexEncode(w http.ResponseWriter, r *http.Request) {
	type payload struct {
		Text string `json:"text"`
	}
	var p payload
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		http.Error(w, "bad payload", 400)
		return
	}

	encoded := hex.EncodeToString([]byte(p.Text))

	result := map[string]any{
		"input": p.Text,
		"encoded": encoded,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func handleHexDecode(w http.ResponseWriter, r *http.Request) {
	type payload struct {
		Text string `json:"text"`
	}
	var p payload
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		http.Error(w, "bad payload", 400)
		return
	}

	decoded, err := hex.DecodeString(p.Text)
	if err != nil {
		json.NewEncoder(w).Encode(map[string]any{
			"valid": false,
			"error": err.Error(),
		})
		return
	}

	result := map[string]any{
		"input": p.Text,
		"decoded": string(decoded),
		"valid": true,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// === UTILITIES HANDLERS ===

func handleTimestampConvert(w http.ResponseWriter, r *http.Request) {
	type payload struct {
		Input  string `json:"input"`
		Format string `json:"format"`
	}
	var p payload
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		http.Error(w, "bad payload", 400)
		return
	}

	now := time.Now()
	var result map[string]any

	if p.Input == "" {
		// Generate current timestamps
		result = map[string]any{
			"unix": now.Unix(),
			"unix_ms": now.UnixMilli(),
			"iso": now.Format(time.RFC3339),
			"rfc3339": now.Format(time.RFC3339),
			"human": now.Format("2006-01-02 15:04:05 MST"),
		}
	} else {
		// Try to parse input
		if unix, err := strconv.ParseInt(p.Input, 10, 64); err == nil {
			var t time.Time
			if unix > 1e10 {
				t = time.UnixMilli(unix)
			} else {
				t = time.Unix(unix, 0)
			}
			result = map[string]any{
				"unix": t.Unix(),
				"unix_ms": t.UnixMilli(),
				"iso": t.Format(time.RFC3339),
				"rfc3339": t.Format(time.RFC3339),
				"human": t.Format("2006-01-02 15:04:05 MST"),
			}
		} else if t, err := time.Parse(time.RFC3339, p.Input); err == nil {
			result = map[string]any{
				"unix": t.Unix(),
				"unix_ms": t.UnixMilli(),
				"iso": t.Format(time.RFC3339),
				"rfc3339": t.Format(time.RFC3339),
				"human": t.Format("2006-01-02 15:04:05 MST"),
			}
		} else {
			result = map[string]any{
				"error": "Invalid timestamp format",
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func handleColorConvert(w http.ResponseWriter, r *http.Request) {
	type payload struct {
		Color  string `json:"color"`
		Format string `json:"format"`
	}
	var p payload
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		http.Error(w, "bad payload", 400)
		return
	}

	// Simple color conversion (basic implementation)
	result := map[string]any{
		"input": p.Color,
		"hex": "#FF0000",
		"rgb": "rgb(255, 0, 0)",
		"hsl": "hsl(0, 100%, 50%)",
		"message": "Color conversion - basic implementation",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func handleLoremGenerate(w http.ResponseWriter, r *http.Request) {
	type payload struct {
		Type  string `json:"type"`
		Count int    `json:"count"`
	}
	var p payload
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		p.Type = "paragraphs"
		p.Count = 3
	}

	if p.Count <= 0 || p.Count > 20 {
		p.Count = 3
	}

	lorem := "Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat."

	var text []string
	for i := 0; i < p.Count; i++ {
		text = append(text, lorem)
	}

	result := map[string]any{
		"type": p.Type,
		"count": p.Count,
		"text": strings.Join(text, "\n\n"),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func handleTextTransform(w http.ResponseWriter, r *http.Request) {
	type payload struct {
		Text      string `json:"text"`
		Transform string `json:"transform"`
	}
	var p payload
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		http.Error(w, "bad payload", 400)
		return
	}

	var transformed string
	switch p.Transform {
	case "uppercase":
		transformed = strings.ToUpper(p.Text)
	case "lowercase":
		transformed = strings.ToLower(p.Text)
	case "title":
		transformed = strings.Title(p.Text)
	case "trim":
		transformed = strings.TrimSpace(p.Text)
	case "reverse":
		runes := []rune(p.Text)
		for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
			runes[i], runes[j] = runes[j], runes[i]
		}
		transformed = string(runes)
	default:
		transformed = p.Text
	}

	result := map[string]any{
		"input": p.Text,
		"transform": p.Transform,
		"output": transformed,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// === HELPER FUNCTIONS ===

func generateSimpleDiff(aLines, bLines []string) []map[string]any {
	changes := []map[string]any{}
	maxLen := len(aLines)
	if len(bLines) > maxLen {
		maxLen = len(bLines)
	}

	for i := 0; i < maxLen; i++ {
		var aLine, bLine string
		if i < len(aLines) {
			aLine = aLines[i]
		}
		if i < len(bLines) {
			bLine = bLines[i]
		}

		if aLine != bLine {
			changes = append(changes, map[string]any{
				"line": i + 1,
				"a": aLine,
				"b": bLine,
				"type": "change",
			})
		}
	}

	return changes
}

func renderMarkdown(md string) string {
	lines := strings.Split(md, "\n")
	var result strings.Builder

	result.WriteString("<div style='font-family: -apple-system, BlinkMacSystemFont, sans-serif; max-width: 800px; margin: 0 auto; padding: 20px;'>")

	inList := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			if inList {
				result.WriteString("</ul>")
				inList = false
			}
			result.WriteString("<br>")
			continue
		}

		if strings.HasPrefix(trimmed, "# ") {
			if inList {
				result.WriteString("</ul>")
				inList = false
			}
			result.WriteString("<h1>" + html.EscapeString(trimmed[2:]) + "</h1>")
		} else if strings.HasPrefix(trimmed, "## ") {
			if inList {
				result.WriteString("</ul>")
				inList = false
			}
			result.WriteString("<h2>" + html.EscapeString(trimmed[3:]) + "</h2>")
		} else if strings.HasPrefix(trimmed, "### ") {
			if inList {
				result.WriteString("</ul>")
				inList = false
			}
			result.WriteString("<h3>" + html.EscapeString(trimmed[4:]) + "</h3>")
		} else if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") {
			if !inList {
				result.WriteString("<ul>")
				inList = true
			}
			result.WriteString("<li>" + html.EscapeString(trimmed[2:]) + "</li>")
		} else {
			if inList {
				result.WriteString("</ul>")
				inList = false
			}
			// Handle **bold** and *italic*
			text := html.EscapeString(trimmed)
			text = regexp.MustCompile(`\*\*(.*?)\*\*`).ReplaceAllString(text, "<strong>$1</strong>")
			text = regexp.MustCompile(`\*(.*?)\*`).ReplaceAllString(text, "<em>$1</em>")
			result.WriteString("<p>" + text + "</p>")
		}
	}

	if inList {
		result.WriteString("</ul>")
	}

	result.WriteString("</div>")
	return result.String()
}

func secureHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(w, r)
	})
}

const webUI = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>DevKit - Developer Toolkit</title>
    <style>
        :root {
            --bg: #ffffff;
            --bg-muted: #fafafa;
            --text: #1a1a1a;
            --text-muted: #666666;
            --accent: #2563eb;
            --border: #e5e7eb;
            --sidebar-bg: #ffffff;
        }

        [data-theme="dark"] {
            --bg: #0a0a0a;
            --bg-muted: #1a1a1a;
            --text: #ffffff;
            --text-muted: #a3a3a3;
            --accent: #3b82f6;
            --border: #262626;
            --sidebar-bg: #0a0a0a;
        }

        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }

        body {
            font-family: -apple-system, BlinkMacSystemFont, sans-serif;
            background: var(--bg);
            color: var(--text);
            line-height: 1.5;
            transition: all 0.2s ease;
        }

        .app-container {
            display: flex;
            height: 100vh;
        }

        .sidebar {
            width: 240px;
            background: var(--sidebar-bg);
            border-right: 1px solid var(--border);
            padding: 24px 0;
            position: fixed;
            height: 100vh;
        }

        .sidebar-header {
            padding: 0 24px 24px;
            border-bottom: 1px solid var(--border);
            margin-bottom: 24px;
        }

        .logo {
            font-size: 20px;
            font-weight: 600;
            color: var(--text);
            margin-bottom: 4px;
        }

        .subtitle {
            font-size: 13px;
            color: var(--text-muted);
        }

        .theme-toggle {
            position: absolute;
            top: 24px;
            right: 24px;
            background: none;
            border: 1px solid var(--border);
            border-radius: 6px;
            padding: 6px;
            cursor: pointer;
            color: var(--text);
            transition: all 0.15s ease;
        }

        .theme-toggle:hover {
            background: var(--bg-muted);
        }

        .nav-item {
            display: flex;
            align-items: center;
            padding: 12px 24px;
            cursor: pointer;
            border: none;
            background: none;
            width: 100%;
            text-align: left;
            color: var(--text-muted);
            font-size: 14px;
            transition: all 0.15s ease;
        }

        .nav-item:hover {
            background: var(--bg-muted);
            color: var(--text);
        }

        .nav-item.active {
            background: var(--accent);
            color: white;
        }

        .nav-group {
            margin-bottom: 24px;
        }

        .nav-group-title {
            font-size: 12px;
            font-weight: 600;
            color: var(--text-muted);
            padding: 8px 24px;
            margin-bottom: 8px;
            text-transform: uppercase;
            letter-spacing: 0.5px;
        }

        .nav-icon {
            width: 16px;
            height: 16px;
            margin-right: 12px;
        }

        .main-content {
            margin-left: 240px;
            padding: 48px;
            width: calc(100% - 240px);
            min-height: 100vh;
            display: flex;
            flex-direction: column;
            align-items: center;
        }

        .tool-container {
            display: none;
            max-width: 800px;
            width: 100%;
        }

        .tool-container.active {
            display: block;
        }

        .tool-header {
            margin-bottom: 32px;
        }

        .tool-title {
            font-size: 24px;
            font-weight: 600;
            color: var(--text);
            margin-bottom: 8px;
        }

        .tool-description {
            font-size: 16px;
            color: var(--text-muted);
        }

        .tool-card {
            background: var(--bg-muted);
            border-radius: 8px;
            padding: 32px;
            border: 1px solid var(--border);
        }

        .form-group {
            margin-bottom: 24px;
        }

        label {
            display: block;
            margin-bottom: 8px;
            font-weight: 500;
            color: var(--text);
            font-size: 14px;
        }

        textarea, input, select {
            width: 100%;
            padding: 12px;
            border: 1px solid var(--border);
            border-radius: 6px;
            font-family: 'SF Mono', Monaco, monospace;
            font-size: 14px;
            resize: vertical;
            transition: border-color 0.15s ease;
            background: var(--bg);
            color: var(--text);
        }

        textarea:focus, input:focus, select:focus {
            outline: none;
            border-color: var(--accent);
        }

        textarea {
            min-height: 120px;
        }

        /* Improve dropdown appearance */
        select {
            appearance: none;
            -webkit-appearance: none;
            -moz-appearance: none;
            background-image: linear-gradient(45deg, transparent 50%, var(--text-muted) 50%),
                              linear-gradient(135deg, var(--text-muted) 50%, transparent 50%);
            background-position: calc(100% - 18px) calc(1em + 2px), calc(100% - 13px) calc(1em + 2px);
            background-size: 5px 5px, 5px 5px;
            background-repeat: no-repeat;
        }

        .btn {
            background: var(--accent);
            color: white;
            border: none;
            padding: 12px 24px;
            border-radius: 6px;
            font-size: 14px;
            font-weight: 500;
            cursor: pointer;
            transition: all 0.15s ease;
            width: 100%;
        }

        .btn:hover {
            opacity: 0.9;
        }

        /* Prevent overlapping when two buttons are adjacent */
        .btn + .btn { margin-top: 12px; }
        .btn-row { display: flex; gap: 12px; flex-wrap: wrap; }
        .btn-row .btn { width: auto; flex: 1 1 160px; }

        .output {
            margin-top: 24px;
            padding: 16px;
            background: var(--bg);
            border: 1px solid var(--border);
            border-radius: 6px;
            font-family: 'SF Mono', Monaco, monospace;
            font-size: 13px;
            white-space: pre-wrap;
            max-height: 300px;
            overflow-y: auto;
        }

        .output.error {
            border-color: #ef4444;
            color: #ef4444;
        }

        .output.success {
            border-color: #22c55e;
            color: var(--text);
        }

        .welcome-container {
            text-align: center;
            padding: 80px 32px;
            max-width: 1000px;
            width: 100%;
        }

        .welcome-title {
            font-size: 32px;
            font-weight: 600;
            color: var(--text);
            margin-bottom: 16px;
        }

        .welcome-subtitle {
            font-size: 18px;
            color: var(--text-muted);
            margin-bottom: 24px;
        }

        .contact-info {
            background: var(--bg-muted);
            border: 1px solid var(--border);
            border-radius: 8px;
            padding: 16px;
            margin: 24px 0;
            font-size: 16px;
            color: var(--text);
        }

        .feature-grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(160px, 1fr));
            gap: 24px;
        }

        .feature-card {
            background: var(--bg-muted);
            padding: 24px;
            border-radius: 8px;
            border: 1px solid var(--border);
            text-align: center;
            transition: all 0.2s ease;
            position: relative;
        }

        .feature-card.clickable-card {
            cursor: pointer;
        }

        .feature-card.clickable-card:hover {
            transform: translateY(-2px);
            box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1);
            border-color: var(--accent);
            background: var(--bg);
        }

        [data-theme="dark"] .feature-card.clickable-card:hover {
            box-shadow: 0 4px 12px rgba(255, 255, 255, 0.1);
        }

        .feature-icon {
            font-size: 24px;
            margin-bottom: 12px;
        }

        .feature-card h3 {
            font-size: 14px;
            font-weight: 600;
            margin-bottom: 8px;
            color: var(--text);
        }

        .feature-card p {
            font-size: 13px;
            color: var(--text-muted);
        }

        /* Favorites / Most Used quick access */
        .quick-access { margin-bottom: 40px; }
        .quick-header { display: flex; align-items: center; justify-content: space-between; margin-bottom: 12px; }
        .quick-tabs { display: flex; gap: 8px; }
        .quick-tab { background: var(--bg-muted); border: 1px solid var(--border); color: var(--text); padding: 6px 12px; border-radius: 999px; cursor: pointer; font-size: 13px; }
        .quick-tab.active { background: var(--accent); color: #fff; border-color: var(--accent); }
        .quick-edit-btn { background: none; border: 1px solid var(--border); border-radius: 6px; padding: 6px 10px; color: var(--text); cursor: pointer; }
        .quick-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(160px, 1fr)); gap: 12px; }
        .quick-card { padding: 14px; border: 1px solid var(--border); border-radius: 8px; background: var(--bg-muted); cursor: pointer; transition: background .15s ease; }
        .quick-card:hover { background: var(--bg); }
        .quick-card-title { font-size: 14px; font-weight: 600; }
        .quick-card-sub { font-size: 12px; color: var(--text-muted); }

        .favorite-toggle { position: absolute; top: 10px; right: 10px; background: none; border: 1px solid var(--border); border-radius: 999px; padding: 4px 8px; cursor: pointer; color: var(--text-muted); }
        .favorite-toggle.active { color: var(--accent); border-color: var(--accent); }

        @media (max-width: 768px) {
            .sidebar {
                transform: translateX(-100%);
                transition: transform 0.2s ease;
            }

            .sidebar.open {
                transform: translateX(0);
            }

            .main-content {
                margin-left: 0;
                width: 100%;
                padding: 24px;
                align-items: stretch;
            }

            .tool-container {
                max-width: none;
            }

            .welcome-container {
                padding: 40px 16px;
            }
        }
    </style>
</head>
<body>
    <div class="app-container">
        <div class="sidebar">
            <div class="sidebar-header">
                <div class="logo">DevKit</div>
                <div class="subtitle">Developer Toolkit</div>
            </div>

            <button class="nav-item active" onclick="showTool('welcome')">
                <svg class="nav-icon" fill="currentColor" viewBox="0 0 24 24">
                    <path d="M12 2L2 7v10c0 5.55 3.84 9.74 9 11 5.16-1.26 9-5.45 9-11V7l-10-5z"/>
                </svg>
                Welcome
            </button>

            <div class="nav-group">
                <div class="nav-group-title">Editing & Inspection</div>

                <button class="nav-item" onclick="showTool('compare')">
                    <svg class="nav-icon" fill="currentColor" viewBox="0 0 24 24">
                        <path d="M9 3H7v2H5v2h2v10H5v2h2v2h2v-2h10v-2h2v-2h-2V5h2V3h-2V1h-2v2H9z"/>
                    </svg>
                    Compare/Diff
                </button>

                <button class="nav-item" onclick="showTool('formatter')">
                    <svg class="nav-icon" fill="currentColor" viewBox="0 0 24 24">
                        <path d="M5 3h2v2H5v5a2 2 0 0 1-2 2 2 2 0 0 1 2 2v5h2v2H5c-1.07-.27-2-.9-2-2v-4a2 2 0 0 0-2-2H0v-2h1a2 2 0 0 0 2-2V5a2 2 0 0 1 2-2zm14 0a2 2 0 0 1 2 2v4a2 2 0 0 0 2 2h1v2h-1a2 2 0 0 0-2 2v4a2 2 0 0 1-2 2h-2v-2h2v-5a2 2 0 0 1 2-2 2 2 0 0 1-2-2V5h-2V3h2z"/>
                    </svg>
                    Formatter
                </button>

                <button class="nav-item" onclick="showTool('regex')">
                    <svg class="nav-icon" fill="currentColor" viewBox="0 0 24 24">
                        <path d="M16 6l2.29 2.29-4.88 4.88-4-4L2 16.59 3.41 18l6-6 4 4 6.3-6.29L22 12V6z"/>
                    </svg>
                    Regex Tester
                </button>

                <button class="nav-item" onclick="showTool('markdown')">
                    <svg class="nav-icon" fill="currentColor" viewBox="0 0 24 24">
                        <path d="M22.46 6c-.77.35-1.6.58-2.46.69.88-.53 1.56-1.37 1.88-2.38-.83.5-1.75.85-2.72 1.05C18.37 4.5 17.26 4 16 4c-2.35 0-4.27 1.92-4.27 4.29 0 .34.04.67.11.98C8.28 9.09 5.11 7.38 3 4.79c-.37.63-.58 1.37-.58 2.15 0 1.49.75 2.81 1.91 3.56-.71 0-1.37-.2-1.95-.5v.03c0 2.08 1.48 3.82 3.44 4.21a4.22 4.22 0 0 1-1.93.07 4.28 4.28 0 0 0 4 2.98 8.521 8.521 0 0 1-5.33 1.84c-.34 0-.68-.02-1.02-.06C3.44 20.29 5.7 21 8.12 21 16 21 20.33 14.46 20.33 8.79c0-.19 0-.37-.01-.56.84-.6 1.56-1.36 2.14-2.23z"/>
                    </svg>
                    Markdown
                </button>
            </div>

            <div class="nav-group">
                <div class="nav-group-title">Security & Encodings</div>

                <button class="nav-item" onclick="showTool('jwt')">
                    <svg class="nav-icon" fill="currentColor" viewBox="0 0 24 24">
                        <path d="M12 1L3 5v6c0 5.55 3.84 9.74 9 11 5.16-1.26 9-5.45 9-11V5l-9-4z"/>
                    </svg>
                    JWT Decoder
                </button>

                <button class="nav-item" onclick="showTool('hash')">
                    <svg class="nav-icon" fill="currentColor" viewBox="0 0 24 24">
                        <path d="M5.5 7A1.5 1.5 0 017 5.5h10A1.5 1.5 0 0118.5 7v10a1.5 1.5 0 01-1.5 1.5H7A1.5 1.5 0 015.5 17V7zm4.19 4.84L9 12.5l4.5 4.5h2l-3.31-3.31zm0 0L9 12.5l4.5 4.5h2l-3.31-3.31z"/>
                    </svg>
                    Hash/UUID
                </button>

                <button class="nav-item" onclick="showTool('encoding')">
                    <svg class="nav-icon" fill="currentColor" viewBox="0 0 24 24">
                        <path d="M9.4 16.6L4.8 12l4.6-4.6L8 6l-6 6 6 6 1.4-1.4zm5.2 0L19.2 12l-4.6-4.6L16 6l6 6-6 6-1.4-1.4z"/>
                    </svg>
                    Encoding
                </button>
            </div>

            <div class="nav-group">
                <div class="nav-group-title">Utilities</div>

                <button class="nav-item" onclick="showTool('timestamp')">
                    <svg class="nav-icon" fill="currentColor" viewBox="0 0 24 24">
                        <path d="M12 2C6.5 2 2 6.5 2 12s4.5 10 10 10 10-4.5 10-10S17.5 2 12 2zm4.2 14.2L11 13V7h1.5v5.2l4.5 2.7-.8 1.3z"/>
                    </svg>
                    Timestamp
                </button>

                <button class="nav-item" onclick="showTool('color')">
                    <svg class="nav-icon" fill="currentColor" viewBox="0 0 24 24">
                        <path d="M12 3c-4.97 0-9 4.03-9 9s4.03 9 9 9c.83 0 1.5-.67 1.5-1.5 0-.39-.15-.74-.39-1.01-.23-.26-.38-.61-.38-.99 0-.83.67-1.5 1.5-1.5H16c2.76 0 5-2.24 5-5 0-4.42-4.03-8-9-8z"/>
                    </svg>
                    Color Tools
                </button>

                <button class="nav-item" onclick="showTool('lorem')">
                    <svg class="nav-icon" fill="currentColor" viewBox="0 0 24 24">
                        <path d="M14 2H6c-1.1 0-1.99.9-1.99 2L4 20c0 1.1.89 2 2 2h8c1.1 0 2-.9 2-2V8l-6-6zm2 16H8v-2h8v2zm0-4H8v-2h8v2zm-3-5V3.5L18.5 9H13z"/>
                    </svg>
                    Lorem & Random
                </button>

                <button class="nav-item" onclick="showTool('text')">
                    <svg class="nav-icon" fill="currentColor" viewBox="0 0 24 24">
                        <path d="M5 4v3h5.5v12h3V7H19V4z"/>
                    </svg>
                    Text Tools
                </button>
            </div>
        </div>

        <div class="main-content">
            <button class="theme-toggle" onclick="toggleTheme()">
                <span id="theme-icon">🌙</span>
            </button>

            <div id="welcome" class="tool-container active">
                <div class="welcome-container">
                    <h1 class="welcome-title">DevKit</h1>
                    <p class="welcome-subtitle">Your all-in-one developer toolkit for formatting, comparing, and previewing</p>

                    <!-- Quick Access: Favorites / Most Used -->
                    <div class="quick-access">
                        <div class="quick-header">
                            <div class="quick-tabs">
                                <button id="tab-fav" class="quick-tab active" onclick="switchQuick('fav')">Favorites</button>
                                <button id="tab-used" class="quick-tab" onclick="switchQuick('used')">Most Used</button>
                            </div>
                            <button class="quick-edit-btn" onclick="toggleFavoritesEditor()">Edit Favorites</button>
                        </div>
                        <div id="quick-fav" class="quick-grid"></div>
                        <div id="quick-used" class="quick-grid" style="display:none"></div>

                        <div id="quick-editor" style="display:none; margin-top:12px; padding:12px; border:1px solid var(--border); border-radius:8px;">
                            <div style="display:flex; gap:16px; flex-wrap:wrap;" id="quick-editor-list"></div>
                            <div style="margin-top:12px; text-align:right;">
                                <button class="btn" style="width:auto;" onclick="saveFavoritesEditor()">Save</button>
                            </div>
                        </div>
                    </div>

                    <div class="feature-grid">
                        <div class="feature-card clickable-card" onclick="showTool('formatter')">
                            <button class="favorite-toggle" onclick="toggleFavorite(event, 'formatter')" title="Toggle favorite">★</button>
                            <div class="feature-icon">{ }</div>
                            <h3>JSON & XML Tools</h3>
                            <p>Format and validate JSON/XML data</p>
                        </div>
                        <div class="feature-card clickable-card" onclick="showTool('compare')">
                            <button class="favorite-toggle" onclick="toggleFavorite(event, 'compare')" title="Toggle favorite">★</button>
                            <div class="feature-icon">⚡</div>
                            <h3>Compare & Diff</h3>
                            <p>Compare JSON objects and text files</p>
                        </div>
                        <div class="feature-card clickable-card" onclick="showTool('markdown')">
                            <button class="favorite-toggle" onclick="toggleFavorite(event, 'markdown')" title="Toggle favorite">★</button>
                            <div class="feature-icon">📝</div>
                            <h3>Markdown Tools</h3>
                            <p>Preview and render markdown</p>
                        </div>
                        <div class="feature-card clickable-card" onclick="showTool('jwt')">
                            <button class="favorite-toggle" onclick="toggleFavorite(event, 'jwt')" title="Toggle favorite">★</button>
                            <div class="feature-icon">🔐</div>
                            <h3>Security Tools</h3>
                            <p>JWT decode, hashing, encoding</p>
                        </div>
                        <div class="feature-card clickable-card" onclick="showTool('regex')">
                            <button class="favorite-toggle" onclick="toggleFavorite(event, 'regex')" title="Toggle favorite">★</button>
                            <div class="feature-icon">🔍</div>
                            <h3>Regex Tester</h3>
                            <p>Test regular expressions</p>
                        </div>
                        <div class="feature-card clickable-card" onclick="showTool('timestamp')">
                            <button class="favorite-toggle" onclick="toggleFavorite(event, 'timestamp')" title="Toggle favorite">★</button>
                            <div class="feature-icon">⏰</div>
                            <h3>Utilities</h3>
                            <p>Timestamps, colors, text tools</p>
                        </div>
                    </div>

                    <!-- Contact Form - Only on Welcome Page -->
                    <div class="contact-form-section" style="margin-top: 80px; padding: 40px 0; border-top: 1px solid var(--border);">
                        <div class="tool-header">
                            <h2 class="tool-title">Contact Us</h2>
                            <p class="tool-description">Have feedback or need additional tools? Get in touch!</p>
                        </div>
                        <div class="tool-card">
                            <div class="form-group">
                                <label for="contact-name">Name:</label>
                                <input type="text" id="contact-name" placeholder="Your name">
                            </div>
                            <div class="form-group">
                                <label for="contact-email">Email:</label>
                                <input type="email" id="contact-email" placeholder="your.email@example.com">
                            </div>
                            <div class="form-group">
                                <label for="contact-message">Message:</label>
                                <textarea id="contact-message" placeholder="Tell us about your feedback or tool requests..." rows="4"></textarea>
                            </div>
                            <button class="btn" onclick="submitContact()">Send Message</button>
                            <div id="contact-output" class="output" style="display: none;"></div>
                        </div>
                    </div>
                </div>
            </div>

            <div id="json-format" class="tool-container">
                <div class="tool-header">
                    <h1 class="tool-title">JSON Formatter</h1>
                    <p class="tool-description">Format and validate JSON data with proper indentation</p>
                </div>
                <div class="tool-card">
                    <div class="form-group">
                        <label for="json-input">Raw JSON:</label>
                        <textarea id="json-input" placeholder='{"name": "John", "age": 30, "city": "New York"}'></textarea>
                    </div>
                    <button class="btn" onclick="formatJSON()">Format JSON</button>
                    <div id="json-output" class="output" style="display: none;"></div>
                </div>
            </div>

            <div id="xml-format" class="tool-container">
                <div class="tool-header">
                    <h1 class="tool-title">XML Formatter</h1>
                    <p class="tool-description">Pretty-print XML documents with proper structure</p>
                </div>
                <div class="tool-card">
                    <div class="form-group">
                        <label for="xml-input">Raw XML:</label>
                        <textarea id="xml-input" placeholder='<root><user><name>John</name><age>30</age></user></root>'></textarea>
                    </div>
                    <button class="btn" onclick="formatXML()">Format XML</button>
                    <div id="xml-output" class="output" style="display: none;"></div>
                </div>
            </div>

            <div id="json-diff" class="tool-container">
                <div class="tool-header">
                    <h1 class="tool-title">JSON Diff</h1>
                    <p class="tool-description">Compare two JSON objects and see the differences</p>
                </div>
                <div class="tool-card">
                    <div class="form-group">
                        <label for="json-a">JSON A:</label>
                        <textarea id="json-a" placeholder='{"name": "John", "age": 30, "city": "New York"}'></textarea>
                    </div>
                    <div class="form-group">
                        <label for="json-b">JSON B:</label>
                        <textarea id="json-b" placeholder='{"name": "John", "age": 31, "city": "San Francisco"}'></textarea>
                    </div>
                    <button class="btn" onclick="diffJSON()">Compare JSON</button>
                    <div id="diff-output" class="output" style="display: none;"></div>
                </div>
            </div>

            <div id="markdown-preview" class="tool-container">
                <div class="tool-header">
                    <h1 class="tool-title">Markdown Preview</h1>
                    <p class="tool-description">Convert Markdown text to HTML and preview the result</p>
                </div>
                <div class="tool-card">
                    <div class="form-group">
                        <label for="markdown-input">Markdown:</label>
                        <textarea id="markdown-input" placeholder="# Hello World

This is **bold** text and this is *italic* text.

- List item 1
- List item 2
- List item 3"></textarea>
                    </div>
                    <button class="btn" onclick="previewMarkdown()">Preview Markdown</button>
                    <div id="markdown-output" class="output" style="display: none;"></div>
                </div>
            </div>

            <div id="compare" class="tool-container">
                <div class="tool-header">
                    <h1 class="tool-title">Compare/Diff</h1>
                    <p class="tool-description">Compare JSON objects and text files to see differences</p>
                </div>
                <div class="tool-card">
                    <div class="form-group">
                        <label for="compare-a">Text A:</label>
                        <textarea id="compare-a" placeholder="Enter first text or JSON to compare"></textarea>
                    </div>
                    <div class="form-group">
                        <label for="compare-b">Text B:</label>
                        <textarea id="compare-b" placeholder="Enter second text or JSON to compare"></textarea>
                    </div>
                    <button class="btn" onclick="compareTexts()">Compare</button>
                    <div id="compare-output" class="output" style="display: none;"></div>
                </div>
            </div>

            <div id="formatter" class="tool-container">
                <div class="tool-header">
                    <h1 class="tool-title">Code Formatter</h1>
                    <p class="tool-description">Format JSON, XML and other data formats</p>
                </div>
                <div class="tool-card">
                    <div class="form-group">
                        <label for="format-input">Code to Format:</label>
                        <textarea id="format-input" placeholder="Paste your JSON, XML or other code here"></textarea>
                    </div>
                    <div class="form-group">
                        <label for="format-type">Format Type:</label>
                        <select id="format-type">
                            <option value="json">JSON</option>
                            <option value="xml">XML</option>
                        </select>
                    </div>
                    <button class="btn" onclick="formatCode()">Format</button>
                    <div id="format-output" class="output" style="display: none;"></div>
                </div>
            </div>

            <div id="regex" class="tool-container">
                <div class="tool-header">
                    <h1 class="tool-title">Regex Tester</h1>
                    <p class="tool-description">Test regular expressions against text</p>
                </div>
                <div class="tool-card">
                    <div class="form-group">
                        <label for="regex-pattern">Regular Expression:</label>
                        <input type="text" id="regex-pattern" placeholder="Enter regex pattern">
                    </div>
                    <div class="form-group">
                        <label for="regex-text">Test Text:</label>
                        <textarea id="regex-text" placeholder="Enter text to test against regex"></textarea>
                    </div>
                    <button class="btn" onclick="testRegex()">Test Regex</button>
                    <div id="regex-output" class="output" style="display: none;"></div>
                </div>
            </div>

            <div id="markdown" class="tool-container">
                <div class="tool-header">
                    <h1 class="tool-title">Markdown Tools</h1>
                    <p class="tool-description">Convert and preview Markdown documents</p>
                </div>
                <div class="tool-card">
                    <div class="form-group">
                        <label for="md-input">Markdown Text:</label>
                        <textarea id="md-input" placeholder="# Your Markdown Here"></textarea>
                    </div>
                    <button class="btn" onclick="renderMarkdown()">Render</button>
                    <div id="md-output" class="output" style="display: none;"></div>
                </div>
            </div>

            <div id="jwt" class="tool-container">
                <div class="tool-header">
                    <h1 class="tool-title">JWT Decoder</h1>
                    <p class="tool-description">Decode and analyze JSON Web Tokens</p>
                </div>
                <div class="tool-card">
                    <div class="form-group">
                        <label for="jwt-token">JWT Token:</label>
                        <textarea id="jwt-token" placeholder="Paste JWT token here"></textarea>
                    </div>
                    <button class="btn" onclick="decodeJWT()">Decode JWT</button>
                    <div id="jwt-output" class="output" style="display: none;"></div>
                </div>
            </div>

            <div id="hash" class="tool-container">
                <div class="tool-header">
                    <h1 class="tool-title">Hash & UUID Generator</h1>
                    <p class="tool-description">Generate hashes and UUIDs</p>
                </div>
                <div class="tool-card">
                    <div class="form-group">
                        <label for="hash-input">Text to Hash:</label>
                        <textarea id="hash-input" placeholder="Enter text to generate hash"></textarea>
                    </div>
                    <div class="form-group">
                        <label for="hash-type">Hash Type:</label>
                        <select id="hash-type">
                            <option value="md5">MD5</option>
                            <option value="sha1">SHA1</option>
                            <option value="sha256">SHA256</option>
                        </select>
                    </div>
                    <div class="btn-row">
                        <button class="btn" onclick="generateHash()">Generate Hash</button>
                        <button class="btn" onclick="generateUUID()">Generate UUID</button>
                    </div>
                    <div id="hash-output" class="output" style="display: none;"></div>
                </div>
            </div>

            <div id="encoding" class="tool-container">
                <div class="tool-header">
                    <h1 class="tool-title">Encoding & Decoding</h1>
                    <p class="tool-description">Encode and decode Base64, URL, and Hex</p>
                </div>
                <div class="tool-card">
                    <div class="form-group">
                        <label for="encode-input">Text to Encode/Decode:</label>
                        <textarea id="encode-input" placeholder="Enter text or encoded data"></textarea>
                    </div>
                    <div class="form-group">
                        <label for="encode-type">Encoding Type:</label>
                        <select id="encode-type">
                            <option value="base64">Base64</option>
                            <option value="url">URL</option>
                            <option value="hex">Hex</option>
                        </select>
                    </div>
                    <div class="btn-row">
                        <button class="btn" onclick="encodeText()">Encode</button>
                        <button class="btn" onclick="decodeText()">Decode</button>
                    </div>
                    <div id="encode-output" class="output" style="display: none;"></div>
                </div>
            </div>

            <div id="timestamp" class="tool-container">
                <div class="tool-header">
                    <h1 class="tool-title">Timestamp Converter</h1>
                    <p class="tool-description">Convert between different timestamp formats</p>
                </div>
                <div class="tool-card">
                    <div class="form-group">
                        <label for="timestamp-input">Timestamp (leave empty for current time):</label>
                        <input type="text" id="timestamp-input" placeholder="Unix timestamp, ISO date, or leave empty">
                    </div>
                    <button class="btn" onclick="convertTimestamp()">Convert</button>
                    <div id="timestamp-output" class="output" style="display: none;"></div>
                </div>
            </div>

            <div id="color" class="tool-container">
                <div class="tool-header">
                    <h1 class="tool-title">Color Tools</h1>
                    <p class="tool-description">Convert between color formats</p>
                </div>
                <div class="tool-card">
                    <div class="form-group">
                        <label for="color-input">Color Value:</label>
                        <input type="text" id="color-input" placeholder="#FF0000, rgb(255,0,0), or hsl(0,100%,50%)">
                    </div>
                    <button class="btn" onclick="convertColor()">Convert</button>
                    <div id="color-output" class="output" style="display: none;"></div>
                </div>
            </div>

            <div id="lorem" class="tool-container">
                <div class="tool-header">
                    <h1 class="tool-title">Lorem Ipsum Generator</h1>
                    <p class="tool-description">Generate placeholder text for your projects</p>
                </div>
                <div class="tool-card">
                    <div class="form-group">
                        <label for="lorem-count">Number of Paragraphs:</label>
                        <input type="number" id="lorem-count" value="3" min="1" max="20">
                    </div>
                    <button class="btn" onclick="generateLorem()">Generate Lorem Ipsum</button>
                    <div id="lorem-output" class="output" style="display: none;"></div>
                </div>
            </div>

            <div id="text" class="tool-container">
                <div class="tool-header">
                    <h1 class="tool-title">Text Transformation</h1>
                    <p class="tool-description">Transform text case and format</p>
                </div>
                <div class="tool-card">
                    <div class="form-group">
                        <label for="text-input">Text to Transform:</label>
                        <textarea id="text-input" placeholder="Enter text to transform"></textarea>
                    </div>
                    <div class="form-group">
                        <label for="text-transform">Transformation:</label>
                        <select id="text-transform">
                            <option value="uppercase">UPPERCASE</option>
                            <option value="lowercase">lowercase</option>
                            <option value="title">Title Case</option>
                            <option value="trim">Trim Whitespace</option>
                            <option value="reverse">Reverse</option>
                        </select>
                    </div>
                    <button class="btn" onclick="transformText()">Transform</button>
                    <div id="text-output" class="output" style="display: none;"></div>
                </div>
            </div>

        </div>
    </div>

    <script>
        // Theme management
        function toggleTheme() {
            const html = document.documentElement;
            const currentTheme = html.getAttribute('data-theme');
            const newTheme = currentTheme === 'dark' ? 'light' : 'dark';

            html.setAttribute('data-theme', newTheme);
            localStorage.setItem('theme', newTheme);

            const themeIcon = document.getElementById('theme-icon');
            themeIcon.textContent = newTheme === 'dark' ? '☀️' : '🌙';
        }

        // Initialize theme from localStorage
        function initTheme() {
            const savedTheme = localStorage.getItem('theme') || 'light';
            document.documentElement.setAttribute('data-theme', savedTheme);

            const themeIcon = document.getElementById('theme-icon');
            themeIcon.textContent = savedTheme === 'dark' ? '☀️' : '🌙';
        }

        // Tool registry for quick access and favorites
        const DEVKIT_TOOLS = [
            { id: 'formatter', name: 'JSON & XML Tools' },
            { id: 'compare', name: 'Compare / Diff' },
            { id: 'markdown', name: 'Markdown Tools' },
            { id: 'jwt', name: 'JWT Decoder' },
            { id: 'hash', name: 'Hash & UUID' },
            { id: 'encoding', name: 'Encoding' },
            { id: 'regex', name: 'Regex Tester' },
            { id: 'timestamp', name: 'Timestamp' },
            { id: 'color', name: 'Color Tools' },
            { id: 'lorem', name: 'Lorem & Random' },
            { id: 'text', name: 'Text Tools' },
        ];

        function getFavorites() {
            try { return JSON.parse(localStorage.getItem('devkit:favorites') || '[]'); } catch { return []; }
        }
        function setFavorites(list) {
            localStorage.setItem('devkit:favorites', JSON.stringify(list));
        }
        function isFavorite(id) { return getFavorites().includes(id); }
        function toggleFavorite(evt, id) {
            if (evt && evt.stopPropagation) evt.stopPropagation();
            const favs = new Set(getFavorites());
            if (favs.has(id)) favs.delete(id); else favs.add(id);
            setFavorites(Array.from(favs));
            renderQuickAccess();
            // Toggle button state if present
            if (evt && evt.target) {
                evt.target.classList.toggle('active', favs.has(id));
            }
        }

        function getUsage() {
            try { return JSON.parse(localStorage.getItem('devkit:usage') || '{}'); } catch { return {}; }
        }
        function bumpUsage(id) {
            const usage = getUsage();
            usage[id] = (usage[id] || 0) + 1;
            localStorage.setItem('devkit:usage', JSON.stringify(usage));
        }

        function switchQuick(tab) {
            const fav = document.getElementById('quick-fav');
            const used = document.getElementById('quick-used');
            const tabFav = document.getElementById('tab-fav');
            const tabUsed = document.getElementById('tab-used');
            if (!fav || !used) return;
            if (tab === 'fav') {
                fav.style.display = '';
                used.style.display = 'none';
                if (tabFav) tabFav.classList.add('active');
                if (tabUsed) tabUsed.classList.remove('active');
            } else {
                fav.style.display = 'none';
                used.style.display = '';
                if (tabFav) tabFav.classList.remove('active');
                if (tabUsed) tabUsed.classList.add('active');
            }
        }

        function renderQuickAccess() {
            const favWrap = document.getElementById('quick-fav');
            const usedWrap = document.getElementById('quick-used');
            if (!favWrap || !usedWrap) return;
            const favs = getFavorites();
            favWrap.innerHTML = '';
            favs.forEach(id => {
                const meta = DEVKIT_TOOLS.find(t => t.id === id);
                if (!meta) return;
                const d = document.createElement('div');
                d.className = 'quick-card';
                d.onclick = () => showTool(id);
                d.innerHTML = '<div class="quick-card-title">' + meta.name + '</div><div class="quick-card-sub">Quick launch</div>';
                favWrap.appendChild(d);
            });

            const usage = getUsage();
            const sorted = DEVKIT_TOOLS
                .map(t => ({...t, count: usage[t.id] || 0}))
                .filter(t => t.count > 0)
                .sort((a,b) => b.count - a.count)
                .slice(0, 8);
            usedWrap.innerHTML = '';
            sorted.forEach(t => {
                const d = document.createElement('div');
                d.className = 'quick-card';
                d.onclick = () => showTool(t.id);
                d.innerHTML = '<div class="quick-card-title">' + t.name + '</div><div class="quick-card-sub">Used ' + t.count + 'x</div>';
                usedWrap.appendChild(d);
            });

            // Update star buttons on feature cards
            document.querySelectorAll('.feature-card .favorite-toggle').forEach(btn => {
                const id = (btn.getAttribute('onclick') || '').match(/'([a-z]+)'/);
                if (id && id[1]) btn.classList.toggle('active', isFavorite(id[1]));
            });
        }

        function toggleFavoritesEditor() {
            const p = document.getElementById('quick-editor');
            if (!p) return; p.style.display = p.style.display === 'none' || p.style.display === '' ? 'block' : 'none';
            if (p.style.display === 'block') buildFavoritesEditor();
        }
        function buildFavoritesEditor() {
            const wrap = document.getElementById('quick-editor-list');
            if (!wrap) return;
            const favs = new Set(getFavorites());
            wrap.innerHTML = DEVKIT_TOOLS.map(function(t) {
                var checked = favs.has(t.id) ? 'checked' : '';
                return '<label style="display:flex; align-items:center; gap:6px; border:1px solid var(--border); padding:6px 10px; border-radius:6px;">' +
                    '<input type="checkbox" value="' + t.id + '" ' + checked + '>' +
                    '<span>' + t.name + '</span>' +
                    '</label>';
            }).join('');
        }

        function saveFavoritesEditor() {
            const wrap = document.getElementById('quick-editor-list');
            if (!wrap) return;
            const ids = Array.from(wrap.querySelectorAll('input[type="checkbox"]:checked')).map(i => i.value);
            setFavorites(ids);
            document.getElementById('quick-editor').style.display = 'none';
            renderQuickAccess();
        }

        // Navigation
        function showTool(toolId) {
            // Hide all tool containers
            document.querySelectorAll('.tool-container').forEach(container => {
                container.classList.remove('active');
            });

            // Show selected tool (with null check to prevent errors)
            const toolContainer = document.getElementById(toolId);
            if (toolContainer) {
                toolContainer.classList.add('active');
            }

            // Track usage for quick access (ignore welcome)
            if (toolId && toolId !== 'welcome' && DEVKIT_TOOLS.find(t => t.id === toolId)) {
                bumpUsage(toolId);
                renderQuickAccess();
            }

            // Update nav active state
            document.querySelectorAll('.nav-item').forEach(item => {
                item.classList.remove('active');
            });
            if (typeof event !== 'undefined' && event && event.target) {
                event.target.classList.add('active');
            }
        }

        // API functions
        async function formatJSON() {
            const input = document.getElementById('json-input').value;
            const output = document.getElementById('json-output');

            try {
                const response = await fetch('/v1/format/json', {
                    method: 'POST',
                    headers: {'Content-Type': 'application/json'},
                    body: input
                });

                if (response.ok) {
                    const formatted = await response.text();
                    output.textContent = formatted;
                    output.className = 'output success';
                } else {
                    const error = await response.text();
                    output.textContent = 'Error: ' + error;
                    output.className = 'output error';
                }
                output.style.display = 'block';
            } catch (err) {
                output.textContent = 'Error: ' + err.message;
                output.className = 'output error';
                output.style.display = 'block';
            }
        }

        async function formatXML() {
            const input = document.getElementById('xml-input').value;
            const output = document.getElementById('xml-output');

            try {
                const response = await fetch('/v1/format/xml', {
                    method: 'POST',
                    headers: {'Content-Type': 'application/xml'},
                    body: input
                });

                if (response.ok) {
                    const formatted = await response.text();
                    output.textContent = formatted;
                    output.className = 'output success';
                } else {
                    const error = await response.text();
                    output.textContent = 'Error: ' + error;
                    output.className = 'output error';
                }
                output.style.display = 'block';
            } catch (err) {
                output.textContent = 'Error: ' + err.message;
                output.className = 'output error';
                output.style.display = 'block';
            }
        }

        async function diffJSON() {
            const a = document.getElementById('json-a').value;
            const b = document.getElementById('json-b').value;
            const output = document.getElementById('diff-output');

            try {
                const payload = {a: JSON.parse(a), b: JSON.parse(b)};
                const response = await fetch('/v1/diff/json', {
                    method: 'POST',
                    headers: {'Content-Type': 'application/json'},
                    body: JSON.stringify(payload)
                });

                if (response.ok) {
                    const diff = await response.json();
                    output.textContent = JSON.stringify(diff, null, 2);
                    output.className = 'output success';
                } else {
                    const error = await response.text();
                    output.textContent = 'Error: ' + error;
                    output.className = 'output error';
                }
                output.style.display = 'block';
            } catch (err) {
                output.textContent = 'Error: ' + err.message;
                output.className = 'output error';
                output.style.display = 'block';
            }
        }

        async function previewMarkdown() {
            const input = document.getElementById('markdown-input').value;
            const output = document.getElementById('markdown-output');

            try {
                const response = await fetch('/v1/preview/markdown', {
                    method: 'POST',
                    headers: {'Content-Type': 'text/plain'},
                    body: input
                });

                if (response.ok) {
                    const html = await response.text();
                    output.innerHTML = html;
                    output.className = 'output success';
                } else {
                    const error = await response.text();
                    output.textContent = 'Error: ' + error;
                    output.className = 'output error';
                }
                output.style.display = 'block';
            } catch (err) {
                output.textContent = 'Error: ' + err.message;
                output.className = 'output error';
                output.style.display = 'block';
            }
        }

        // Additional API functions for new tools
        async function compareTexts() {
            const a = document.getElementById('compare-a').value;
            const b = document.getElementById('compare-b').value;
            const output = document.getElementById('compare-output');

            try {
                const payload = {a: a, b: b};
                const response = await fetch('/v1/diff/text', {
                    method: 'POST',
                    headers: {'Content-Type': 'application/json'},
                    body: JSON.stringify(payload)
                });

                if (response.ok) {
                    const diff = await response.json();
                    output.textContent = JSON.stringify(diff, null, 2);
                    output.className = 'output success';
                } else {
                    const error = await response.text();
                    output.textContent = 'Error: ' + error;
                    output.className = 'output error';
                }
                output.style.display = 'block';
            } catch (err) {
                output.textContent = 'Error: ' + err.message;
                output.className = 'output error';
                output.style.display = 'block';
            }
        }

        async function formatCode() {
            const input = document.getElementById('format-input').value;
            const type = document.getElementById('format-type').value;
            const output = document.getElementById('format-output');

            try {
                const endpoint = type === 'json' ? '/v1/format/json' : '/v1/format/xml';
                const response = await fetch(endpoint, {
                    method: 'POST',
                    headers: {'Content-Type': type === 'json' ? 'application/json' : 'application/xml'},
                    body: input
                });

                if (response.ok) {
                    const formatted = await response.text();
                    output.textContent = formatted;
                    output.className = 'output success';
                } else {
                    const error = await response.text();
                    output.textContent = 'Error: ' + error;
                    output.className = 'output error';
                }
                output.style.display = 'block';
            } catch (err) {
                output.textContent = 'Error: ' + err.message;
                output.className = 'output error';
                output.style.display = 'block';
            }
        }

        async function testRegex() {
            const pattern = document.getElementById('regex-pattern').value;
            const text = document.getElementById('regex-text').value;
            const output = document.getElementById('regex-output');

            try {
                const payload = {pattern: pattern, text: text, flags: ''};
                const response = await fetch('/v1/regex/test', {
                    method: 'POST',
                    headers: {'Content-Type': 'application/json'},
                    body: JSON.stringify(payload)
                });

                if (response.ok) {
                    const result = await response.json();
                    output.textContent = JSON.stringify(result, null, 2);
                    output.className = 'output success';
                } else {
                    const error = await response.text();
                    output.textContent = 'Error: ' + error;
                    output.className = 'output error';
                }
                output.style.display = 'block';
            } catch (err) {
                output.textContent = 'Error: ' + err.message;
                output.className = 'output error';
                output.style.display = 'block';
            }
        }

        async function renderMarkdown() {
            const input = document.getElementById('md-input').value;
            const output = document.getElementById('md-output');

            try {
                const response = await fetch('/v1/preview/markdown', {
                    method: 'POST',
                    headers: {'Content-Type': 'text/plain'},
                    body: input
                });

                if (response.ok) {
                    const html = await response.text();
                    output.innerHTML = html;
                    output.className = 'output success';
                } else {
                    const error = await response.text();
                    output.textContent = 'Error: ' + error;
                    output.className = 'output error';
                }
                output.style.display = 'block';
            } catch (err) {
                output.textContent = 'Error: ' + err.message;
                output.className = 'output error';
                output.style.display = 'block';
            }
        }

        async function decodeJWT() {
            const token = document.getElementById('jwt-token').value;
            const output = document.getElementById('jwt-output');

            try {
                const payload = {token: token};
                const response = await fetch('/v1/jwt/decode', {
                    method: 'POST',
                    headers: {'Content-Type': 'application/json'},
                    body: JSON.stringify(payload)
                });

                if (response.ok) {
                    const result = await response.json();
                    output.textContent = JSON.stringify(result, null, 2);
                    output.className = 'output success';
                } else {
                    const error = await response.text();
                    output.textContent = 'Error: ' + error;
                    output.className = 'output error';
                }
                output.style.display = 'block';
            } catch (err) {
                output.textContent = 'Error: ' + err.message;
                output.className = 'output error';
                output.style.display = 'block';
            }
        }

        async function generateHash() {
            const text = document.getElementById('hash-input').value;
            const type = document.getElementById('hash-type').value;
            const output = document.getElementById('hash-output');

            try {
                const payload = {text: text, type: type};
                const response = await fetch('/v1/hash/generate', {
                    method: 'POST',
                    headers: {'Content-Type': 'application/json'},
                    body: JSON.stringify(payload)
                });

                if (response.ok) {
                    const result = await response.json();
                    output.textContent = JSON.stringify(result, null, 2);
                    output.className = 'output success';
                } else {
                    const error = await response.text();
                    output.textContent = 'Error: ' + error;
                    output.className = 'output error';
                }
                output.style.display = 'block';
            } catch (err) {
                output.textContent = 'Error: ' + err.message;
                output.className = 'output error';
                output.style.display = 'block';
            }
        }

        async function generateUUID() {
            const output = document.getElementById('hash-output');

            try {
                const payload = {version: 'v4', count: 1};
                const response = await fetch('/v1/uuid/generate', {
                    method: 'POST',
                    headers: {'Content-Type': 'application/json'},
                    body: JSON.stringify(payload)
                });

                if (response.ok) {
                    const result = await response.json();
                    output.textContent = JSON.stringify(result, null, 2);
                    output.className = 'output success';
                } else {
                    const error = await response.text();
                    output.textContent = 'Error: ' + error;
                    output.className = 'output error';
                }
                output.style.display = 'block';
            } catch (err) {
                output.textContent = 'Error: ' + err.message;
                output.className = 'output error';
                output.style.display = 'block';
            }
        }

        async function encodeText() {
            const text = document.getElementById('encode-input').value;
            const type = document.getElementById('encode-type').value;
            const output = document.getElementById('encode-output');

            try {
                const payload = {text: text};
                const endpoint = '/v1/' + type + '/encode';
                const response = await fetch(endpoint, {
                    method: 'POST',
                    headers: {'Content-Type': 'application/json'},
                    body: JSON.stringify(payload)
                });

                if (response.ok) {
                    const result = await response.json();
                    output.textContent = JSON.stringify(result, null, 2);
                    output.className = 'output success';
                } else {
                    const error = await response.text();
                    output.textContent = 'Error: ' + error;
                    output.className = 'output error';
                }
                output.style.display = 'block';
            } catch (err) {
                output.textContent = 'Error: ' + err.message;
                output.className = 'output error';
                output.style.display = 'block';
            }
        }

        async function decodeText() {
            const text = document.getElementById('encode-input').value;
            const type = document.getElementById('encode-type').value;
            const output = document.getElementById('encode-output');

            try {
                const payload = {text: text};
                const endpoint = '/v1/' + type + '/decode';
                const response = await fetch(endpoint, {
                    method: 'POST',
                    headers: {'Content-Type': 'application/json'},
                    body: JSON.stringify(payload)
                });

                if (response.ok) {
                    const result = await response.json();
                    output.textContent = JSON.stringify(result, null, 2);
                    output.className = 'output success';
                } else {
                    const error = await response.text();
                    output.textContent = 'Error: ' + error;
                    output.className = 'output error';
                }
                output.style.display = 'block';
            } catch (err) {
                output.textContent = 'Error: ' + err.message;
                output.className = 'output error';
                output.style.display = 'block';
            }
        }

        async function convertTimestamp() {
            const input = document.getElementById('timestamp-input').value;
            const output = document.getElementById('timestamp-output');

            try {
                const payload = {input: input, format: ''};
                const response = await fetch('/v1/timestamp/convert', {
                    method: 'POST',
                    headers: {'Content-Type': 'application/json'},
                    body: JSON.stringify(payload)
                });

                if (response.ok) {
                    const result = await response.json();
                    output.textContent = JSON.stringify(result, null, 2);
                    output.className = 'output success';
                } else {
                    const error = await response.text();
                    output.textContent = 'Error: ' + error;
                    output.className = 'output error';
                }
                output.style.display = 'block';
            } catch (err) {
                output.textContent = 'Error: ' + err.message;
                output.className = 'output error';
                output.style.display = 'block';
            }
        }

        async function convertColor() {
            const color = document.getElementById('color-input').value;
            const output = document.getElementById('color-output');

            try {
                const payload = {color: color, format: ''};
                const response = await fetch('/v1/color/convert', {
                    method: 'POST',
                    headers: {'Content-Type': 'application/json'},
                    body: JSON.stringify(payload)
                });

                if (response.ok) {
                    const result = await response.json();
                    output.textContent = JSON.stringify(result, null, 2);
                    output.className = 'output success';
                } else {
                    const error = await response.text();
                    output.textContent = 'Error: ' + error;
                    output.className = 'output error';
                }
                output.style.display = 'block';
            } catch (err) {
                output.textContent = 'Error: ' + err.message;
                output.className = 'output error';
                output.style.display = 'block';
            }
        }

        async function generateLorem() {
            const count = document.getElementById('lorem-count').value;
            const output = document.getElementById('lorem-output');

            try {
                const payload = {type: 'paragraphs', count: parseInt(count)};
                const response = await fetch('/v1/lorem/generate', {
                    method: 'POST',
                    headers: {'Content-Type': 'application/json'},
                    body: JSON.stringify(payload)
                });

                if (response.ok) {
                    const result = await response.json();
                    output.textContent = result.text;
                    output.className = 'output success';
                } else {
                    const error = await response.text();
                    output.textContent = 'Error: ' + error;
                    output.className = 'output error';
                }
                output.style.display = 'block';
            } catch (err) {
                output.textContent = 'Error: ' + err.message;
                output.className = 'output error';
                output.style.display = 'block';
            }
        }

        async function transformText() {
            const text = document.getElementById('text-input').value;
            const transform = document.getElementById('text-transform').value;
            const output = document.getElementById('text-output');

            try {
                const payload = {text: text, transform: transform};
                const response = await fetch('/v1/text/transform', {
                    method: 'POST',
                    headers: {'Content-Type': 'application/json'},
                    body: JSON.stringify(payload)
                });

                if (response.ok) {
                    const result = await response.json();
                    output.textContent = result.output;
                    output.className = 'output success';
                } else {
                    const error = await response.text();
                    output.textContent = 'Error: ' + error;
                    output.className = 'output error';
                }
                output.style.display = 'block';
            } catch (err) {
                output.textContent = 'Error: ' + err.message;
                output.className = 'output error';
                output.style.display = 'block';
            }
        }

        function submitContact() {
            const name = document.getElementById('contact-name').value;
            const email = document.getElementById('contact-email').value;
            const message = document.getElementById('contact-message').value;
            const output = document.getElementById('contact-output');

            if (!name || !email || !message) {
                output.textContent = 'Please fill in all fields.';
                output.className = 'output error';
                output.style.display = 'block';
                return;
            }

            // Create mailto link with form data
            const subject = encodeURIComponent('DevKit Feedback from ' + name);
            const body = encodeURIComponent('Name: ' + name + '\nEmail: ' + email + '\n\nMessage:\n' + message);
            const mailtoLink = 'mailto:arpandey@groupon.com?subject=' + subject + '&body=' + body;

            // Open email client
            window.location.href = mailtoLink;

            // Show success message
            output.textContent = 'Opening your email client to send the message...';
            output.className = 'output success';
            output.style.display = 'block';

            // Clear form after a short delay
            setTimeout(function() {
                document.getElementById('contact-name').value = '';
                document.getElementById('contact-email').value = '';
                document.getElementById('contact-message').value = '';
                output.style.display = 'none';
            }, 3000);
        }

        // Initialize on page load
        initTheme();
        renderQuickAccess();
    </script>
</body>
</html>`
