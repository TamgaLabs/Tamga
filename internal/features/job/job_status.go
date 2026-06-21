package job

type Status string

const (
	WAITING Status = "WAITING"
	DONE    Status = "DONE"
	FAILED  Status = "FAILED"
)
