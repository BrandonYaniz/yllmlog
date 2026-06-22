package socket

import "encoding/json"

const (
	ActionStatus     = "status"
	ActionLogsList   = "logs.list"
	ActionLogsAdd    = "logs.add"
	ActionLogsRemove = "logs.remove"
)

// Request is one daemon API request.
type Request struct {
	ID     string          `json:"id,omitempty"`
	Action string          `json:"action"`
	Params json.RawMessage `json:"params,omitempty"`
}

// Response is one daemon API response.
type Response struct {
	ID     string          `json:"id,omitempty"`
	OK     bool            `json:"ok"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  string          `json:"error,omitempty"`
}

// Status reports daemon readiness.
type Status struct {
	Version        string `json:"version"`
	Ready          bool   `json:"ready"`
	LastCycleAt    string `json:"last_cycle_at,omitempty"`
	LastCycleError string `json:"last_cycle_error,omitempty"`
}

type LogsAddParams struct {
	Path        string `json:"path"`
	ServiceName string `json:"service_name,omitempty"`
}

type LogsRemoveParams struct {
	Path string `json:"path"`
}
