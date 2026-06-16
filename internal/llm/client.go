package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"time"
)

const defaultDialTimeout = 30 * time.Second

type Client struct {
	socketPath string
	profile    string
	timeout    time.Duration
}

type Request struct {
	Profile string `json:"profile"`
	Prompt  string `json:"prompt"`
	JSON    bool   `json:"json"`
}

type Response struct {
	Text string          `json:"text"`
	JSON json.RawMessage `json:"json,omitempty"`
}

func NewClient(socketPath, profile string) (Client, error) {
	if strings.TrimSpace(socketPath) == "" {
		return Client{}, errors.New("yllmd socket path is required")
	}
	if strings.TrimSpace(profile) == "" {
		return Client{}, errors.New("yllmd profile is required")
	}
	return Client{
		socketPath: socketPath,
		profile:    profile,
		timeout:    defaultDialTimeout,
	}, nil
}

func (c Client) Generate(ctx context.Context, prompt string, jsonMode bool) (Response, error) {
	if strings.TrimSpace(prompt) == "" {
		return Response{}, errors.New("prompt is required")
	}

	dialer := net.Dialer{Timeout: c.timeout}
	conn, err := dialer.DialContext(ctx, "unix", c.socketPath)
	if err != nil {
		return Response{}, fmt.Errorf("connect to yllmd socket: %w", err)
	}
	defer conn.Close()

	request := Request{
		Profile: c.profile,
		Prompt:  prompt,
		JSON:    jsonMode,
	}
	if err := json.NewEncoder(conn).Encode(request); err != nil {
		return Response{}, fmt.Errorf("write yllmd request: %w", err)
	}

	scanner := bufio.NewScanner(conn)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return Response{}, fmt.Errorf("read yllmd response: %w", err)
		}
		return Response{}, errors.New("read yllmd response: connection closed")
	}

	var response Response
	decoder := json.NewDecoder(bytes.NewReader(scanner.Bytes()))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&response); err != nil {
		return Response{}, fmt.Errorf("decode yllmd response: %w", err)
	}
	if len(response.JSON) == 0 && response.Text == "" {
		return Response{}, errors.New("yllmd response is empty")
	}
	return response, nil
}

func StrictJSON[T any](response Response) (T, error) {
	var value T
	payload := response.JSON
	if len(payload) == 0 {
		payload = []byte(response.Text)
	}
	if len(bytes.TrimSpace(payload)) == 0 {
		return value, errors.New("strict json response is empty")
	}

	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&value); err != nil {
		return value, fmt.Errorf("decode strict json: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return value, errors.New("decode strict json: trailing data")
	}
	return value, nil
}
