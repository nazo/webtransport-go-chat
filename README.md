# webtransport-go-chat

## Requirements

- go 1.21

## Usage

- create `localhost.pem` and `localhost-key.pem`

```
mkcert localhost
```

- (Google Chrome) Enable WebTransport Developer Mode
  - [chrome://flags/#webtransport-developer-mode](chrome://flags/#webtransport-developer-mode)

- run `go run main.go`

- open `index.html`
