package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ayushanand/tiny-aws/control-plane/internal/memory"
	"github.com/ayushanand/tiny-aws/control-plane/internal/types"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	if os.Args[1] != "instance" {
		usage()
		os.Exit(2)
	}
	if len(os.Args) < 3 {
		usage()
		os.Exit(2)
	}

	base := strings.TrimRight(env("TINY_API_ADDR", "127.0.0.1:8080"), "/")
	if !strings.Contains(base, "://") {
		base = "http://" + base
	}
	client := &http.Client{Timeout: 30 * time.Second}

	switch os.Args[2] {
	case "create":
		spec, err := parseCreate(os.Args[3:])
		if err != nil {
			fail(err)
		}
		var inst types.Instance
		code, err := doJSON(client, http.MethodPost, base+"/v1/instances", spec, &inst)
		if err != nil {
			fail(err)
		}
		_ = printJSON(inst)
		if code >= 300 {
			fail(fmt.Errorf("create failed: HTTP %d", code))
		}
	case "list":
		var list []types.Instance
		code, err := doJSON(client, http.MethodGet, base+"/v1/instances", nil, &list)
		if err != nil {
			fail(err)
		}
		_ = printJSON(list)
		if code >= 300 {
			fail(fmt.Errorf("list failed: HTTP %d", code))
		}
	case "inspect":
		name, err := oneName(os.Args[3:])
		if err != nil {
			fail(err)
		}
		var inst types.Instance
		code, err := doJSON(client, http.MethodGet, base+"/v1/instances/"+url.PathEscape(name), nil, &inst)
		if err != nil {
			fail(err)
		}
		_ = printJSON(inst)
		if code >= 300 {
			fail(fmt.Errorf("inspect failed: HTTP %d", code))
		}
	case "stop":
		name, err := oneName(os.Args[3:])
		if err != nil {
			fail(err)
		}
		var inst types.Instance
		code, err := doJSON(client, http.MethodPost, base+"/v1/instances/"+url.PathEscape(name)+"/stop", nil, &inst)
		if err != nil {
			fail(err)
		}
		_ = printJSON(inst)
		if code >= 300 {
			fail(fmt.Errorf("stop failed: HTTP %d", code))
		}
	case "start":
		name, err := oneName(os.Args[3:])
		if err != nil {
			fail(err)
		}
		var inst types.Instance
		code, err := doJSON(client, http.MethodPost, base+"/v1/instances/"+url.PathEscape(name)+"/start", nil, &inst)
		if err != nil {
			fail(err)
		}
		_ = printJSON(inst)
		if code >= 300 {
			fail(fmt.Errorf("start failed: HTTP %d", code))
		}
	case "delete":
		name, err := oneName(os.Args[3:])
		if err != nil {
			fail(err)
		}
		var out map[string]any
		code, err := doJSON(client, http.MethodDelete, base+"/v1/instances/"+url.PathEscape(name), nil, &out)
		if err != nil {
			fail(err)
		}
		_ = printJSON(out)
		if code >= 300 {
			fail(fmt.Errorf("delete failed: HTTP %d", code))
		}
	default:
		usage()
		os.Exit(2)
	}
}

func doJSON(client *http.Client, method, rawURL string, body any, out any) (int, error) {
	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return 0, err
		}
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, rawURL, rdr)
	if err != nil {
		return 0, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, err
	}
	if len(data) > 0 && out != nil {
		if err := json.Unmarshal(data, out); err != nil {
			return resp.StatusCode, fmt.Errorf("decode response: %w (body=%s)", err, string(data))
		}
	}
	return resp.StatusCode, nil
}

func parseCreate(args []string) (types.Spec, error) {
	var spec types.Spec
	var mem string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--name":
			i++
			if i >= len(args) {
				return spec, fmt.Errorf("--name needs a value")
			}
			spec.Name = args[i]
		case "--image":
			i++
			if i >= len(args) {
				return spec, fmt.Errorf("--image needs a value")
			}
			spec.Image = args[i]
		case "--cpu":
			i++
			if i >= len(args) {
				return spec, fmt.Errorf("--cpu needs a value")
			}
			n, err := strconv.ParseUint(args[i], 10, 32)
			if err != nil {
				return spec, err
			}
			spec.CPU = uint32(n)
		case "--memory":
			i++
			if i >= len(args) {
				return spec, fmt.Errorf("--memory needs a value")
			}
			mem = args[i]
		default:
			return spec, fmt.Errorf("unknown flag %s", args[i])
		}
	}
	if mem == "" {
		return spec, fmt.Errorf("--memory is required")
	}
	b, err := memory.ParseSize(mem)
	if err != nil {
		return spec, err
	}
	spec.MemoryBytes = b
	return spec, nil
}

func oneName(args []string) (string, error) {
	if len(args) != 1 {
		return "", fmt.Errorf("expected <name>")
	}
	return args[0], nil
}

func printJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func fail(err error) {
	fmt.Fprintf(os.Stderr, "error: %v\n", err)
	os.Exit(1)
}

func usage() {
	fmt.Fprintf(os.Stderr, `tiny instance create --name NAME --image IMAGE --cpu N --memory SIZE
tiny instance list
tiny instance inspect NAME
tiny instance stop NAME
tiny instance start NAME
tiny instance delete NAME

Env: TINY_API_ADDR (default 127.0.0.1:8080). CLI talks only to tinyd over HTTP.
`)
}
