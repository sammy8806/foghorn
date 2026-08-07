package provider

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const (
	// maxResponseBytes caps how much of a response body is read before decoding.
	// A hostile (or simply broken) source can otherwise stream gigabytes inside
	// the client timeout and exhaust local memory.
	maxResponseBytes = 16 << 20 // 16 MiB

	// maxErrorBodyBytes caps the slice of an error response quoted back in
	// messages and logs.
	maxErrorBodyBytes = 8 << 10 // 8 KiB
)

// decodeJSONResponse decodes resp.Body into out, reading at most
// maxResponseBytes.
func decodeJSONResponse(resp *http.Response, out any) error {
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxResponseBytes)).Decode(out); err != nil {
		return err
	}
	return nil
}

// readLimited reads a response body up to limit bytes, ignoring read errors —
// it is used for error/diagnostic paths where a partial body is still useful.
func readLimited(r io.Reader, limit int64) []byte {
	body, _ := io.ReadAll(io.LimitReader(r, limit))
	return body
}

// errorBody returns a trimmed, size-capped rendering of an error response body.
func errorBody(resp *http.Response) string {
	return strings.TrimSpace(string(readLimited(resp.Body, maxErrorBodyBytes)))
}

// readBodyLimited reads a full response body up to maxResponseBytes, reporting
// read errors to the caller.
func readBodyLimited(resp *http.Response) ([]byte, error) {
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}
	return body, nil
}
