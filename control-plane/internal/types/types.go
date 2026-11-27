package types

import "time"

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
	Network     string
}

type Instance struct {
	ID            string
	Name          string
	Image         string
	ImageID       string `json:",omitempty"`
	RootfsPath    string
	CPU           uint32
	MemoryBytes   uint64
	Status        Status
	PID           int32
	ErrorMessage  string
	OwnerID       string
	NodeID        string
	Observability *Observability `json:",omitempty"`
}

type Node struct {
	ID                     string
	Hostname               string
	ImageRoot              string
	AllocatableCPU         uint32
	AllocatableMemoryBytes uint64
	AllocatedCPU           uint32
	AllocatedMemoryBytes   uint64
	RunningInstances       uint32
	Connected              bool
}

type NodeImageStatus string

const (
	NodeImageReady  NodeImageStatus = "READY"
	NodeImageFailed NodeImageStatus = "FAILED"
)

type NodeImage struct {
	NodeID       string
	ImageID      string
	Status       NodeImageStatus
	RootfsPath   string
	ErrorMessage string
}

type Observability struct {
	Ebpf        bool
	CPUUsec     uint64
	MemoryBytes uint64
	RxPackets   uint64
	RxBytes     uint64
	TxPackets   uint64
	TxBytes     uint64
	Error       string `json:",omitempty"`
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
	OwnerID      string
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
	OwnerID            string
}

type NetworkStatus string

const (
	NetworkStatusReady  NetworkStatus = "READY"
	NetworkStatusFailed NetworkStatus = "FAILED"
)

type Network struct {
	ID           string
	Name         string
	CIDR         string
	GatewayIP    string
	BridgeName   string
	Status       NetworkStatus
	ErrorMessage string
	OwnerID      string
}

type UserRole = string

const (
	RoleAdmin UserRole = "admin"
	RoleUser  UserRole = "user"
)

type User struct {
	ID           string
	Username     string
	PasswordHash string `json:"-"`
	Role         string
}

type AccessSession struct {
	TokenHash string
	UserID    string
	ExpiresAt time.Time
}

type RefreshSession struct {
	TokenHash string
	UserID    string
	ExpiresAt time.Time
}

type NetworkAttachment struct {
	NetworkID    string
	NetworkName  string
	InstanceID   string
	InstanceName string
	GuestIP      string
	GuestIPCIDR  string
	VethHost     string
	VethGuest    string
	Status       string
}
