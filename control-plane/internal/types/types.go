package types

type Status string

const (
	StatusPending  Status = "PENDING"
	StatusRunning  Status = "RUNNING"
	StatusStopping Status = "STOPPING"
	StatusStopped  Status = "STOPPED"
	StatusFailed   Status = "FAILED"
)

type Spec struct {
	Name        string
	Image       string
	CPU         uint32
	MemoryBytes uint64
}

type Instance struct {
	ID           string
	Name         string
	Image        string
	CPU          uint32
	MemoryBytes  uint64
	Status       Status
	PID          int32
	ErrorMessage string
}

type RuntimeView struct {
	ID           string
	Name         string
	Image        string
	CPU          uint32
	MemoryBytes  uint64
	Status       Status
	PID          int32
	ErrorMessage string
}
