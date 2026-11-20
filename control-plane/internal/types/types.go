package types

type Status string

const (
	StatusPending  Status = "PENDING"
	StatusRunning  Status = "RUNNING"
	StatusStopping Status = "STOPPING"
	StatusStopped  Status = "STOPPED"
	StatusFailed   Status = "FAILED"
)

type ImageStatus string

const (
	ImageStatusReady  ImageStatus = "READY"
	ImageStatusFailed ImageStatus = "FAILED"
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
	RootfsPath   string
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

type Image struct {
	ID           string
	Name         string
	ArtifactPath string
	RootfsPath   string
	Status       ImageStatus
	ErrorMessage string
}

type VolumeStatus string

const (
	VolumeStatusAvailable VolumeStatus = "AVAILABLE"
	VolumeStatusAttached  VolumeStatus = "ATTACHED"
	VolumeStatusFailed    VolumeStatus = "FAILED"
)

type Volume struct {
	ID                 string
	Name               string
	SizeBytes          uint64
	DiskPath           string
	Status             VolumeStatus
	AttachedInstanceID string
	AttachedInstance   string
	DevicePath         string
	MountName          string
	ErrorMessage       string
}
