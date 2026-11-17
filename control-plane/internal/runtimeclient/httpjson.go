package runtimeclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/ayushanand/tiny-aws/control-plane/internal/types"
)

// HTTPJSON talks to tinyruntimed using the same field names as runtime.proto.
// TODO(m1): replace with gRPC once scripts/gen-proto.sh output is wired.
type HTTPJSON struct {
	base   string
	client *http.Client
}

func Dial(addr string) (*HTTPJSON, error) {
	if !strings.Contains(addr, "://") {
		addr = "http://" + addr
	}
	return &HTTPJSON{
		base: strings.TrimRight(addr, "/"),
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}, nil
}

func (c *HTTPJSON) Close() error { return nil }

type createReq struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Image       string `json:"image"`
	CPU         uint32 `json:"cpu"`
	MemoryBytes uint64 `json:"memory_bytes"`
}

type createResp struct {
	ID           string `json:"id"`
	Status       string `json:"status"`
	PID          int32  `json:"pid"`
	ErrorMessage string `json:"error_message"`
}

type instanceJSON struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Image        string `json:"image"`
	CPU          uint32 `json:"cpu"`
	MemoryBytes  uint64 `json:"memory_bytes"`
	Status       string `json:"status"`
	PID          int32  `json:"pid"`
	ErrorMessage string `json:"error_message"`
}

func (c *HTTPJSON) CreateInstance(ctx context.Context, inst types.Instance) (types.RuntimeView, error) {
	var out createResp
	err := c.post(ctx, "/v1/create", createReq{
		ID: inst.ID, Name: inst.Name, Image: inst.Image, CPU: inst.CPU, MemoryBytes: inst.MemoryBytes,
	}, &out)
	if err != nil {
		return types.RuntimeView{}, err
	}
	return types.RuntimeView{
		ID: out.ID, Status: types.Status(out.Status), PID: out.PID, ErrorMessage: out.ErrorMessage,
		Name: inst.Name, Image: inst.Image, CPU: inst.CPU, MemoryBytes: inst.MemoryBytes,
	}, nil
}

func (c *HTTPJSON) ListInstances(ctx context.Context) ([]types.RuntimeView, error) {
	var out struct {
		Instances []instanceJSON `json:"instances"`
	}
	if err := c.get(ctx, "/v1/list", &out); err != nil {
		return nil, err
	}
	views := make([]types.RuntimeView, 0, len(out.Instances))
	for _, it := range out.Instances {
		views = append(views, toView(it))
	}
	return views, nil
}

func (c *HTTPJSON) InspectInstance(ctx context.Context, id string) (types.RuntimeView, error) {
	var out instanceJSON
	if err := c.get(ctx, "/v1/inspect?id="+id, &out); err != nil {
		return types.RuntimeView{}, err
	}
	return toView(out), nil
}

func (c *HTTPJSON) StartInstance(ctx context.Context, id string) (types.RuntimeView, error) {
	var out createResp
	if err := c.post(ctx, "/v1/start", map[string]string{"id": id}, &out); err != nil {
		return types.RuntimeView{}, err
	}
	return types.RuntimeView{ID: out.ID, Status: types.Status(out.Status), PID: out.PID, ErrorMessage: out.ErrorMessage}, nil
}

func (c *HTTPJSON) StopInstance(ctx context.Context, id string) (types.RuntimeView, error) {
	var out createResp
	if err := c.post(ctx, "/v1/stop", map[string]string{"id": id}, &out); err != nil {
		return types.RuntimeView{}, err
	}
	return types.RuntimeView{ID: out.ID, Status: types.Status(out.Status), PID: out.PID, ErrorMessage: out.ErrorMessage}, nil
}

func (c *HTTPJSON) DeleteInstance(ctx context.Context, id string) error {
	var out struct {
		ID           string `json:"id"`
		ErrorMessage string `json:"error_message"`
	}
	if err := c.post(ctx, "/v1/delete", map[string]string{"id": id}, &out); err != nil {
		return err
	}
	if out.ErrorMessage != "" {
		return fmt.Errorf("%s", out.ErrorMessage)
	}
	return nil
}

func toView(it instanceJSON) types.RuntimeView {
	return types.RuntimeView{
		ID: it.ID, Name: it.Name, Image: it.Image, CPU: it.CPU, MemoryBytes: it.MemoryBytes,
		Status: types.Status(it.Status), PID: it.PID, ErrorMessage: it.ErrorMessage,
	}
}

func (c *HTTPJSON) post(ctx context.Context, path string, body any, out any) error {
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+path, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("runtime rpc: %w", err)
	}
	defer res.Body.Close()
	return decode(res, out)
}

func (c *HTTPJSON) get(ctx context.Context, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+path, nil)
	if err != nil {
		return err
	}
	res, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("runtime rpc: %w", err)
	}
	defer res.Body.Close()
	return decode(res, out)
}

func decode(res *http.Response, out any) error {
	data, err := io.ReadAll(res.Body)
	if err != nil {
		return err
	}
	if res.StatusCode >= 400 {
		return fmt.Errorf("runtime http %s: %s", res.Status, strings.TrimSpace(string(data)))
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(data, out)
}
