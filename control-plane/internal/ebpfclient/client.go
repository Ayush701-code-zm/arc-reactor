package ebpfclient

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"time"
)

type Client struct {
	Sock string
}

type Sample struct {
	OK          bool   `json:"ok"`
	Ebpf        bool   `json:"ebpf"`
	CPUUsec     uint64 `json:"cpu_usec"`
	MemoryBytes uint64 `json:"memory_bytes"`
	RxPackets   uint64 `json:"rx_packets"`
	RxBytes     uint64 `json:"rx_bytes"`
	TxPackets   uint64 `json:"tx_packets"`
	TxBytes     uint64 `json:"tx_bytes"`
	Error       string `json:"error"`
}

func DefaultSock() string {
	if v := os.Getenv("TINY_EBPF_SOCK"); v != "" {
		return v
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		return home + "/.tiny-aws/ebpf.sock"
	}
	return "./tiny-aws-ebpf.sock"
}

func Dial() *Client {
	return &Client{Sock: DefaultSock()}
}

func Require() bool {
	return os.Getenv("TINY_REQUIRE_EBPF") == "1"
}

func (c *Client) call(req map[string]any) (Sample, error) {
	if c == nil || c.Sock == "" {
		return Sample{}, fmt.Errorf("ebpf client not configured")
	}
	conn, err := net.DialTimeout("unix", c.Sock, 2*time.Second)
	if err != nil {
		return Sample{}, err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
	b, err := json.Marshal(req)
	if err != nil {
		return Sample{}, err
	}
	if _, err := conn.Write(append(b, '\n')); err != nil {
		return Sample{}, err
	}
	dec := json.NewDecoder(conn)
	var out Sample
	if err := dec.Decode(&out); err != nil {
		return Sample{}, err
	}
	if !out.OK {
		if out.Error == "" {
			out.Error = "ebpf request failed"
		}
		return out, fmt.Errorf("%s", out.Error)
	}
	return out, nil
}

func (c *Client) Ping() (Sample, error) {
	return c.call(map[string]any{"op": "ping"})
}

func (c *Client) Watch(instanceID string, pid int32) (Sample, error) {
	return c.call(map[string]any{
		"op":          "watch",
		"instance_id": instanceID,
		"pid":         pid,
	})
}

func (c *Client) Unwatch(instanceID string) error {
	_, err := c.call(map[string]any{"op": "unwatch", "instance_id": instanceID})
	return err
}

func (c *Client) Sample(instanceID string) (Sample, error) {
	return c.call(map[string]any{"op": "sample", "instance_id": instanceID})
}
