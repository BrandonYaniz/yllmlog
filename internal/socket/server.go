package socket

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Handler processes a decoded socket request.
type Handler func(context.Context, Request) (any, error)

// Server accepts local Unix socket requests.
type Server struct {
	socketPath string
	handler    Handler
	listener   net.Listener
	done       chan struct{}
	once       sync.Once
}

// NewServer creates a Unix socket server.
func NewServer(socketPath string, handler Handler) (*Server, error) {
	if strings.TrimSpace(socketPath) == "" {
		return nil, errors.New("socket path is required")
	}
	if handler == nil {
		return nil, errors.New("socket handler is required")
	}
	return &Server{
		socketPath: socketPath,
		handler:    handler,
		done:       make(chan struct{}),
	}, nil
}

// Listen creates the socket and starts accepting connections.
func (s *Server) Listen() error {
	if err := prepareSocket(s.socketPath); err != nil {
		return err
	}

	listener, err := net.Listen("unix", s.socketPath)
	if err != nil {
		return fmt.Errorf("listen on unix socket: %w", err)
	}
	if err := os.Chmod(s.socketPath, 0o660); err != nil {
		listener.Close()
		return fmt.Errorf("set socket permissions: %w", err)
	}

	s.listener = listener
	go s.accept()
	return nil
}

// Close stops the server and removes its socket file.
func (s *Server) Close() error {
	var err error
	s.once.Do(func() {
		if s.listener != nil {
			err = s.listener.Close()
		}
		close(s.done)
		if removeErr := os.Remove(s.socketPath); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) && err == nil {
			err = removeErr
		}
	})
	return err
}

func (s *Server) accept() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.done:
				return
			default:
				continue
			}
		}
		go s.handleConn(conn)
	}
}

func (s *Server) handleConn(conn net.Conn) {
	defer conn.Close()

	scanner := bufio.NewScanner(conn)
	encoder := json.NewEncoder(conn)
	for scanner.Scan() {
		var request Request
		if err := json.Unmarshal(scanner.Bytes(), &request); err != nil {
			writeResponse(encoder, Response{OK: false, Error: "invalid request json"})
			continue
		}

		result, err := s.handler(context.Background(), request)
		response := Response{ID: request.ID, OK: err == nil}
		if err != nil {
			response.Error = err.Error()
			writeResponse(encoder, response)
			continue
		}

		if result != nil {
			payload, err := json.Marshal(result)
			if err != nil {
				writeResponse(encoder, Response{ID: request.ID, OK: false, Error: "encode response result"})
				continue
			}
			response.Result = payload
		}
		writeResponse(encoder, response)
	}
}

func writeResponse(encoder *json.Encoder, response Response) {
	_ = encoder.Encode(response)
}

func prepareSocket(socketPath string) error {
	if err := os.MkdirAll(filepath.Dir(socketPath), 0o750); err != nil {
		return fmt.Errorf("create socket directory: %w", err)
	}

	info, err := os.Lstat(socketPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect existing socket path: %w", err)
	}
	if info.Mode()&os.ModeSocket == 0 {
		return fmt.Errorf("socket path exists and is not a socket: %s", socketPath)
	}
	if err := os.Remove(socketPath); err != nil {
		return fmt.Errorf("remove stale socket: %w", err)
	}
	return nil
}
