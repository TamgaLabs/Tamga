package domain

type ProtocolVersion int

const ProtocolV1 ProtocolVersion = 1

type ClientCapabilities struct {
	Fs       *FSCapabilities       `json:"fs,omitempty"`
	Terminal bool                   `json:"terminal,omitempty"`
}

type FSCapabilities struct {
	ReadTextFile  bool `json:"readTextFile"`
	WriteTextFile bool `json:"writeTextFile"`
}

type AgentCapabilities struct {
	LoadSession      bool                   `json:"loadSession"`
	PromptCapabilities *PromptCapabilities  `json:"promptCapabilities,omitempty"`
	McpCapabilities  *McpCapabilities       `json:"mcpCapabilities,omitempty"`
	SessionCapabilities *SessionCapabilities `json:"sessionCapabilities,omitempty"`
	Auth             *AgentAuthCapabilities `json:"auth,omitempty"`
}

type PromptCapabilities struct {
	Image           bool `json:"image"`
	Audio           bool `json:"audio"`
	EmbeddedContext bool `json:"embeddedContext"`
}

type McpCapabilities struct {
	HTTP bool `json:"http"`
	SSE  bool `json:"sse"`
}

type SessionCapabilities struct {
	List    map[string]any `json:"list,omitempty"`
	Delete  map[string]any `json:"delete,omitempty"`
	Close   map[string]any `json:"close,omitempty"`
	Resume  map[string]any `json:"resume,omitempty"`
}

type AgentAuthCapabilities struct {
	Logout map[string]any `json:"logout,omitempty"`
}

type Implementation struct {
	Name    string `json:"name"`
	Title   string `json:"title,omitempty"`
	Version string `json:"version"`
}

type AuthMethod struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
}

type MCPServer struct {
	Name    string        `json:"name"`
	Command string        `json:"command,omitempty"`
	Args    []string      `json:"args,omitempty"`
	Env     []EnvVariable `json:"env,omitempty"`
	Type    string        `json:"type,omitempty"`
	URL     string        `json:"url,omitempty"`
	Headers []HTTPHeader  `json:"headers,omitempty"`
}

type EnvVariable struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type HTTPHeader struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type ContentBlock struct {
	Type       string          `json:"type"`
	Text       string          `json:"text,omitempty"`
	Data       string          `json:"data,omitempty"`
	MimeType   string          `json:"mimeType,omitempty"`
	URI        string          `json:"uri,omitempty"`
	Resource   *EmbeddedResource `json:"resource,omitempty"`
	Name       string          `json:"name,omitempty"`
	Size       int64           `json:"size,omitempty"`
	Title      string          `json:"title,omitempty"`
	Description string         `json:"description,omitempty"`
}

type EmbeddedResource struct {
	URI      string `json:"uri"`
	MimeType string `json:"mimeType,omitempty"`
	Text     string `json:"text,omitempty"`
	Blob     string `json:"blob,omitempty"`
}

type SessionUpdate struct {
	SessionUpdate string       `json:"sessionUpdate"`
	MessageID     string       `json:"messageId,omitempty"`
	Content       *ContentBlock `json:"content,omitempty"`
	Entries       []PlanEntry  `json:"entries,omitempty"`
	ToolCallID    string       `json:"toolCallId,omitempty"`
	Title         string       `json:"title,omitempty"`
	Kind          string       `json:"kind,omitempty"`
	Status        string       `json:"status,omitempty"`
}

type PlanEntry struct {
	Content  string `json:"content"`
	Priority string `json:"priority"`
	Status   string `json:"status"`
}
