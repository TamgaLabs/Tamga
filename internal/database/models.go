package database

import "time"

type User struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type Project struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	UserID      string    `json:"user_id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Domain struct {
	ID        string    `json:"id"`
	ProjectID string    `json:"project_id"`
	Domain    string    `json:"domain"`
	Verified  bool      `json:"verified"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type EnvVar struct {
	ID        string    `json:"id"`
	ProjectID string    `json:"project_id"`
	Key       string    `json:"key"`
	Value     string    `json:"value"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type GitRepository struct {
	ID        string    `json:"id"`
	ProjectID string    `json:"project_id"`
	Url       string    `json:"url"`
	Branch    string    `json:"branch"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Deployment struct {
	ID            string    `json:"id"`
	ProjectID     string    `json:"project_id"`
	Status        string    `json:"status"`
	CommitSha     string    `json:"commit_sha"`
	CommitMessage string    `json:"commit_message"`
	ImageTag      string    `json:"image_tag"`
	ContainerID   string    `json:"container_id"`
	Domain        string    `json:"domain"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type DeploymentLog struct {
	ID           string    `json:"id"`
	DeploymentID string    `json:"deployment_id"`
	Stream       string    `json:"stream"`
	Message      string    `json:"message"`
	CreatedAt    time.Time `json:"created_at"`
}
