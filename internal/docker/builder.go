package docker

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/docker/docker/api/types"
)

type BuildOptions struct {
	Dockerfile   string
	BuildContext io.Reader
	Tag          string
	Labels       map[string]string
}

type BuildOutputLine struct {
	Stream      string `json:"stream,omitempty"`
	ErrorDetail *struct {
		Message string `json:"message"`
	} `json:"errorDetail,omitempty"`
	Error string `json:"error,omitempty"`
}

func (c *Client) BuildImage(ctx context.Context, opts BuildOptions, out io.Writer) error {
	resp, err := c.cli.ImageBuild(ctx, opts.BuildContext, types.ImageBuildOptions{
		Dockerfile:  opts.Dockerfile,
		Tags:        []string{opts.Tag},
		Labels:      opts.Labels,
		Remove:      true,
		ForceRemove: true,
		PullParent:  true,
	})
	if err != nil {
		return fmt.Errorf("build failed: %w", err)
	}
	defer resp.Body.Close()

	dec := json.NewDecoder(resp.Body)
	for {
		var line BuildOutputLine
		if err := dec.Decode(&line); err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("failed to read build output: %w", err)
		}
		if line.ErrorDetail != nil {
			return fmt.Errorf("build error: %s", line.ErrorDetail.Message)
		}
		if line.Stream != "" && out != nil {
			io.WriteString(out, line.Stream)
		}
	}

	return nil
}

func NewBuildContext(dockerfileContent string, files map[string]string) (io.Reader, error) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)

	if files == nil {
		files = make(map[string]string)
	}
	files["Dockerfile"] = dockerfileContent

	for name, content := range files {
		hdr := &tar.Header{
			Name: name,
			Mode: 0644,
			Size: int64(len(content)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return nil, fmt.Errorf("failed to write tar header: %w", err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			return nil, fmt.Errorf("failed to write tar content: %w", err)
		}
	}

	if err := tw.Close(); err != nil {
		return nil, fmt.Errorf("failed to close tar: %w", err)
	}

	return &buf, nil
}

func NewDockerfileFromReader(r io.Reader) (string, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func HasDockerfile(files []string) bool {
	for _, f := range files {
		if f == "Dockerfile" || strings.EqualFold(f, "dockerfile") {
			return true
		}
	}
	return false
}

func DetectRuntime(files []string) string {
	if HasDockerfile(files) {
		return "dockerfile"
	}
	for _, f := range files {
		switch {
		case f == "package.json":
			return "node"
		case f == "go.mod" || f == "go.sum":
			return "go"
		case f == "requirements.txt" || f == "Pipfile" || f == "pyproject.toml":
			return "python"
		}
	}
	return "unknown"
}

func GenerateDockerfile(runtime string) string {
	switch runtime {
	case "node":
		return `FROM node:20-alpine
WORKDIR /app
COPY package*.json ./
RUN npm ci --only=production
COPY . .
EXPOSE 3000
CMD ["node", "index.js"]
`
	case "go":
		return `FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /app/server .

FROM alpine:3.21
RUN apk --no-cache add ca-certificates
WORKDIR /app
COPY --from=builder /app/server .
EXPOSE 8080
CMD ["./server"]
`
	case "python":
		return `FROM python:3.12-slim
WORKDIR /app
COPY requirements.txt* ./
RUN pip install --no-cache-dir -r requirements.txt 2>/dev/null || true
COPY . .
EXPOSE 8000
CMD ["python", "main.py"]
`
	default:
		return ""
	}
}
