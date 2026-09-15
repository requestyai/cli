package oauth

import (
	"html/template"
	"net/http"
)

// callbackPage is what the browser shows once the redirect has landed. It
// mirrors the dashboard's consent card so the hand-off feels like one flow.
type callbackPage struct {
	Success bool
	Title   string
	Message string
	Hint    string
}

func writePage(w http.ResponseWriter, page callbackPage) {
	_ = pageTemplate.Execute(w, page)
}

// pageTemplate is self-contained: no scripts, fonts, or images are fetched, so
// it renders the same offline and never leaks the callback URL to a third
// party. Colours follow the dashboard's Tailwind palette in both schemes.
var pageTemplate = template.Must(template.New("callback").Parse(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.Title}} · Requesty CLI</title>
<style>
  :root {
    --page: #ffffff;
    --card: #ffffff;
    --border: #e5e7eb;
    --fg: #111827;
    --muted: #6b7280;
    --tile-blue-bg: #dbeafe;
    --tile-blue-fg: #2563eb;
    --tile-gray-bg: #f3f4f6;
    --tile-gray-fg: #4b5563;
  }
  @media (prefers-color-scheme: dark) {
    :root {
      --page: #0a0d14;
      --card: #111827;
      --border: #1f2937;
      --fg: #f9fafb;
      --muted: #9ca3af;
      --tile-blue-bg: #1e3a8a;
      --tile-blue-fg: #60a5fa;
      --tile-gray-bg: #1f2937;
      --tile-gray-fg: #9ca3af;
    }
  }
  * { box-sizing: border-box; }
  html, body { height: 100%; margin: 0; }
  body {
    display: flex;
    align-items: center;
    justify-content: center;
    background: var(--page);
    color: var(--fg);
    font-family: ui-sans-serif, system-ui, -apple-system, "Segoe UI", Roboto, sans-serif;
    -webkit-font-smoothing: antialiased;
  }
  .card {
    width: 500px;
    max-width: calc(100% - 2rem);
    background: var(--card);
    border: 2px solid var(--border);
    border-radius: 0.75rem;
    box-shadow: 0 20px 25px -5px rgb(0 0 0 / 0.1), 0 8px 10px -6px rgb(0 0 0 / 0.1);
    overflow: hidden;
  }
  .header {
    padding: 1.5rem;
    text-align: center;
    border-bottom: 1px solid var(--border);
  }
  .tiles {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 1rem;
  }
  .tile {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 3rem;
    height: 3rem;
    border-radius: 0.5rem;
    font-size: 1.25rem;
    font-weight: 700;
  }
  .tile.blue { background: var(--tile-blue-bg); color: var(--tile-blue-fg); }
  .tile.gray { background: var(--tile-gray-bg); color: var(--tile-gray-fg); }
  .link {
    position: relative;
    width: 3rem;
    height: 2px;
    background: var(--border);
  }
  .dot {
    position: absolute;
    top: 50%;
    left: 50%;
    transform: translate(-50%, -50%);
    display: flex;
    align-items: center;
    justify-content: center;
    width: 1.25rem;
    height: 1.25rem;
    border-radius: 9999px;
    color: #ffffff;
    font-size: 9px;
    font-weight: 500;
    line-height: 1;
  }
  .dot.ok { background: #2da44e; }
  .dot.bad { background: #dc2626; }
  h1 {
    margin: 1rem 0 0;
    font-size: 1.25rem;
    font-weight: 600;
  }
  .body {
    padding: 1.5rem;
    text-align: center;
  }
  .message {
    margin: 0;
    font-size: 0.875rem;
    line-height: 1.5;
    overflow-wrap: anywhere;
  }
  .hint {
    margin: 1rem 0 0;
    font-size: 0.75rem;
    color: var(--muted);
  }
</style>
</head>
<body>
  <main class="card" role="status">
    <div class="header">
      <div class="tiles">
        <div class="tile blue">R</div>
        <div class="link"><div class="dot {{if .Success}}ok{{else}}bad{{end}}">{{if .Success}}✓{{else}}✕{{end}}</div></div>
        <div class="tile gray">R</div>
      </div>
      <h1>{{.Title}}</h1>
    </div>
    <div class="body">
      <p class="message">{{.Message}}</p>
      <p class="hint">{{.Hint}}</p>
    </div>
  </main>
</body>
</html>
`))
