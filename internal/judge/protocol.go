package judge

import "time"

type FileInfo struct {
	ID           string    `json:"_id"`
	Name         string    `json:"name"`
	Size         int64     `json:"size"`
	ETag         string    `json:"etag,omitempty"`
	LastModified time.Time `json:"lastModified,omitempty"`
}

type Task struct {
	Rid      string            `json:"rid"`
	Lang     string            `json:"lang"`
	Code     string            `json:"code"`
	Data     []FileInfo        `json:"data,omitempty"`
	Source   string            `json:"source,omitempty"`
	Meta     map[string]any    `json:"meta,omitempty"`
	Files    map[string]string `json:"files,omitempty"`
	Input    any               `json:"input,omitempty"`
	Contest  string            `json:"contest,omitempty"`
	UID      int               `json:"uid"`
	DomainID string            `json:"domainId"`
	Config   map[string]any    `json:"config,omitempty"`
	Priority int               `json:"priority"`
	Type     string            `json:"type"`
	Trusted  bool              `json:"trusted"`
}

type Result struct {
	Key          string  `json:"key"`
	Rid          string  `json:"rid"`
	Status       int     `json:"status,omitempty"`
	Score        int     `json:"score,omitempty"`
	Time         float64 `json:"time,omitempty"`
	Memory       int     `json:"memory,omitempty"`
	Message      any     `json:"message,omitempty"`
	CompilerText string  `json:"compilerText,omitempty"`
}
