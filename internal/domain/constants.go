package domain

import "time"

const (
	// MaxRequestBodySize is the maximum size of an incoming webhook payload (10MB).
	MaxRequestBodySize = 10 * 1024 * 1024
	// MaxResponseBodySize is the maximum size of a delivery response body to read (10KB).
	MaxResponseBodySize = 10 * 1024
	// SQLiteBusyTimeout is the SQLite busy timeout in milliseconds.
	SQLiteBusyTimeout = 5000
	// DefaultPaginationLimit is the default number of items per page.
	DefaultPaginationLimit = 50
	// MaxRedirects is the maximum number of HTTP redirects to follow.
	MaxRedirects = 3
	// ShutdownTimeout is the maximum time to wait for graceful shutdown.
	ShutdownTimeout = 30 * time.Second
	// ReadTimeout is the HTTP server read timeout.
	ReadTimeout = 15 * time.Second
	// WriteTimeout is the HTTP server write timeout.
	WriteTimeout = 60 * time.Second
	// IdleTimeout is the HTTP server idle timeout.
	IdleTimeout = 60 * time.Second
	// ResponseBodyTruncation is the maximum length of stored response bodies.
	ResponseBodyTruncation = 1000
	// RetryBackoffBase is the default base delay in seconds for exponential backoff.
	RetryBackoffBase = 30
)
