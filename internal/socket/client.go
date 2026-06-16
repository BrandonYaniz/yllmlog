package socket

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"time"
)

const defaultDialTimeout = 5 * time.Second

// Do sends one request to the daemon socket and waits for one response.
func Do(ctx context.Context, socketPath string, request Request) (Response, error) {
	dialer := net.Dialer{Timeout: defaultDialTimeout}
	conn, err := dialer.DialContext(ctx, "unix", socketPath)
	if err != nil {
		return Response{}, fmt.Errorf("connect to daemon socket: %w", err)
	}
	defer conn.Close()

	if err := json.NewEncoder(conn).Encode(request); err != nil {
		return Response{}, fmt.Errorf("write request: %w", err)
	}

	scanner := bufio.NewScanner(conn)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return Response{}, fmt.Errorf("read response: %w", err)
		}
		return Response{}, fmt.Errorf("read response: connection closed")
	}

	var response Response
	if err := json.Unmarshal(scanner.Bytes(), &response); err != nil {
		return Response{}, fmt.Errorf("decode response: %w", err)
	}
	return response, nil
}

// DecodeResult decodes a successful response result into a concrete type.
func DecodeResult[T any](response Response) (T, error) {
	var value T
	if !response.OK {
		return value, errors.New(response.Error)
	}
	if len(response.Result) == 0 {
		return value, errors.New("response result is empty")
	}
	if err := json.Unmarshal(response.Result, &value); err != nil {
		return value, err
	}
	return value, nil
}
