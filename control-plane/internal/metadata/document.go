package metadata

import (
	"fmt"
	"strings"

	"github.com/ayushanand/tiny-aws/control-plane/internal/types"
)

type Document struct {
	InstanceID    string `json:"instance-id"`
	Name          string `json:"name"`
	OwnerID       string `json:"owner-id"`
	Image         string `json:"image"`
	LocalIPv4     string `json:"local-ipv4,omitempty"`
	NetworkID     string `json:"network-id,omitempty"`
	VethHost      string `json:"veth-host,omitempty"`
	LocalHostname string `json:"local-hostname"`
}

func Build(inst types.Instance, att *types.NetworkAttachment) Document {
	doc := Document{
		InstanceID:    inst.ID,
		Name:          inst.Name,
		OwnerID:       inst.OwnerID,
		Image:         inst.Image,
		LocalHostname: inst.Name,
	}
	if att != nil {
		doc.LocalIPv4 = att.GuestIP
		doc.NetworkID = att.NetworkID
		doc.VethHost = att.VethHost
	}
	return doc
}

func PathValue(doc Document, path string) (string, bool) {
	path = strings.Trim(path, "/")
	if path == "" {
		return listing(), true
	}
	switch path {
	case "instance-id":
		return doc.InstanceID, true
	case "local-hostname":
		return doc.LocalHostname, true
	case "local-ipv4":
		return doc.LocalIPv4, true
	case "public-ipv4":
		return "", true
	case "owner-id":
		return doc.OwnerID, true
	case "veth-host":
		return doc.VethHost, true
	default:
		return "", false
	}
}

func listing() string {
	return strings.Join([]string{
		"instance-id",
		"local-hostname",
		"local-ipv4",
		"public-ipv4",
		"owner-id",
		"veth-host",
	}, "\n") + "\n"
}

func ErrNotRunning(name string) error {
	return fmt.Errorf("instance %q is not RUNNING", name)
}
