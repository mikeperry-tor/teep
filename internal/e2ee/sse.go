package e2ee

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
)

const (
	// sseScannerBufSize is the bufio.Scanner buffer for SSE parsing.
	// Encrypted chunks can be large; 1 MiB is sufficient.
	sseScannerBufSize = 1 << 20 // 1 MiB
)

// sseScannerBufPool reuses 1 MiB scanner buffers across SSE requests.
var sseScannerBufPool = sync.Pool{
	New: func() any {
		buf := make([]byte, sseScannerBufSize)
		return &buf
	},
}

// FinishSSE reads to EOF after the application end marker. Only bounded empty
// lines and SSE comments may follow it. Reading through the scanner preserves
// buffered bytes and propagates errors from an underlying authenticated reader.
func FinishSSE(scanner *bufio.Scanner) error {
	const maxTrailingBytes = 64 << 10
	remaining := maxTrailingBytes
	for scanner.Scan() {
		line := scanner.Text()
		remaining -= len(line) + 1
		if remaining < 0 {
			return errors.New("SSE data after end marker exceeds limit")
		}
		if line != "" && !strings.HasPrefix(line, ":") {
			return errors.New("unexpected SSE data after end marker")
		}
	}
	return scanner.Err()
}

// newSSEScanner creates a bufio.Scanner backed by a pooled 1 MiB buffer.
// The caller must call the returned cleanup function (defer it) to return
// the buffer to the pool.
func newSSEScanner(body io.Reader) (scanner *bufio.Scanner, cleanup func()) {
	scanner = bufio.NewScanner(body)
	bufp, ok := sseScannerBufPool.Get().(*[]byte)
	if !ok {
		panic("sseScannerBufPool: unexpected type")
	}
	scanner.Buffer((*bufp)[:cap(*bufp)], sseScannerBufSize)
	return scanner, func() {
		clear(*bufp)
		sseScannerBufPool.Put(bufp)
	}
}

// WriteSSEError writes an SSE error event and flushes. Used when streaming
// has already started and we can't use http.Error.
func WriteSSEError(w http.ResponseWriter, flusher http.Flusher, msg string) {
	fmt.Fprintf(w, "event: error\ndata: {\"error\":{\"message\":%q,\"type\":\"decryption_error\"}}\n\n", msg)
	flusher.Flush()
}

// SafePrefix returns up to n characters of s for safe use in log messages.
func SafePrefix(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
