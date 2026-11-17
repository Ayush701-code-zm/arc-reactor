package types

// Status mirrors tiny.v1.InstanceStatus for the control plane.
type Status string

const (
	StatusPending  Status = "PENDING"
	StatusRunning  Status = "RUNNING"
	StatusStopping Status = "STOPPING"
	StatusStopped  Status = "STOPPED"
	StatusFailed   Status = "FAILED"
)

// Spec is the create input after CLI parsing.
type Spec struct {
	Name        string
	Image       string
	CPU         uint32
	MemoryBytes uint64
}

// Instance is desired + last-known state stored in SQLite.
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

// RuntimeView is live state from the C++ runtime.
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
