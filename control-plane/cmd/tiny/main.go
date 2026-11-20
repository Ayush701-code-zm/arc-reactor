package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
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

	base := strings.TrimRight(env("TINY_API_ADDR", "127.0.0.1:8080"), "/")
	if !strings.Contains(base, "://") {
		base = "http://" + base
	}
	client := &http.Client{Timeout: 120 * time.Second}

	switch os.Args[1] {
	case "instance":
		runInstance(client, base, os.Args[2:])
	case "image":
		runImage(client, base, os.Args[2:])
	case "volume":
		runVolume(client, base, os.Args[2:])
	default:
		usage()
		os.Exit(2)
	}
}

func runInstance(client *http.Client, base string, args []string) {
	if len(args) < 1 {
		usage()
		os.Exit(2)
	}
	switch args[0] {
	case "create":
		spec, err := parseCreate(args[1:])
		if err != nil {
			fail(err)
		}
		var inst types.Instance
		callJSON(client, http.MethodPost, base+"/v1/instances", spec, &inst, "create")
	case "list":
		var list []types.Instance
		callJSON(client, http.MethodGet, base+"/v1/instances", nil, &list, "list")
	case "inspect":
		name := mustName(args[1:])
		var inst types.Instance
		callJSON(client, http.MethodGet, base+"/v1/instances/"+url.PathEscape(name), nil, &inst, "inspect")
	case "stop":
		name := mustName(args[1:])
		var inst types.Instance
		callJSON(client, http.MethodPost, base+"/v1/instances/"+url.PathEscape(name)+"/stop", nil, &inst, "stop")
	case "start":
		name := mustName(args[1:])
		var inst types.Instance
		callJSON(client, http.MethodPost, base+"/v1/instances/"+url.PathEscape(name)+"/start", nil, &inst, "start")
	case "delete":
		name := mustName(args[1:])
		var out map[string]any
		callJSON(client, http.MethodDelete, base+"/v1/instances/"+url.PathEscape(name), nil, &out, "delete")
	default:
		usage()
		os.Exit(2)
	}
}

func runImage(client *http.Client, base string, args []string) {
	if len(args) < 1 {
		usage()
		os.Exit(2)
	}
	switch args[0] {
	case "import":
		name, file, err := parseImageImport(args[1:])
		if err != nil {
			fail(err)
		}
		abs, err := filepath.Abs(file)
		if err != nil {
			fail(err)
		}
		body := map[string]string{"Name": name, "File": abs}
		var img types.Image
		callJSON(client, http.MethodPost, base+"/v1/images", body, &img, "import")
	case "list":
		var list []types.Image
		callJSON(client, http.MethodGet, base+"/v1/images", nil, &list, "list")
	case "inspect":
		name := mustName(args[1:])
		var img types.Image
		callJSON(client, http.MethodGet, base+"/v1/images/"+url.PathEscape(name), nil, &img, "inspect")
	case "delete":
		name := mustName(args[1:])
		var out map[string]any
		callJSON(client, http.MethodDelete, base+"/v1/images/"+url.PathEscape(name), nil, &out, "delete")
	default:
		usage()
		os.Exit(2)
	}
}

func runVolume(client *http.Client, base string, args []string) {
	if len(args) < 1 {
		usage()
		os.Exit(2)
	}
	switch args[0] {
	case "create":
		name, size, err := parseVolumeCreate(args[1:])
		if err != nil {
			fail(err)
		}
		body := map[string]any{"Name": name, "SizeBytes": size}
		var vol types.Volume
		callJSON(client, http.MethodPost, base+"/v1/volumes", body, &vol, "create")
	case "list":
		var list []types.Volume
		callJSON(client, http.MethodGet, base+"/v1/volumes", nil, &list, "list")
	case "inspect":
		name := mustName(args[1:])
		var vol types.Volume
		callJSON(client, http.MethodGet, base+"/v1/volumes/"+url.PathEscape(name), nil, &vol, "inspect")
	case "delete":
		name := mustName(args[1:])
		var out map[string]any
		callJSON(client, http.MethodDelete, base+"/v1/volumes/"+url.PathEscape(name), nil, &out, "delete")
	case "attach":
		volName, instName, err := parseVolumeAttach(args[1:])
		if err != nil {
			fail(err)
		}
		body := map[string]string{"Instance": instName}
		var vol types.Volume
		callJSON(client, http.MethodPost, base+"/v1/volumes/"+url.PathEscape(volName)+"/attach", body, &vol, "attach")
	case "detach":
		volName, err := parseVolumeDetach(args[1:])
		if err != nil {
			fail(err)
		}
		var vol types.Volume
		callJSON(client, http.MethodPost, base+"/v1/volumes/"+url.PathEscape(volName)+"/detach", map[string]any{}, &vol, "detach")
	default:
		usage()
		os.Exit(2)
	}
}

func callJSON(client *http.Client, method, rawURL string, body any, out any, op string) {
	code, err := doJSON(client, method, rawURL, body, out)
	if err != nil {
		fail(err)
	}
	_ = printJSON(out)
	if code >= 300 {
		fail(fmt.Errorf("%s failed: HTTP %d", op, code))
	}
}

func mustName(args []string) string {
	name, err := oneName(args)
	if err != nil {
		fail(err)
	}
	return name
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

func parseImageImport(args []string) (string, string, error) {
	var name, file string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--name":
			i++
			if i >= len(args) {
				return "", "", fmt.Errorf("--name needs a value")
			}
			name = args[i]
		case "--file":
			i++
			if i >= len(args) {
				return "", "", fmt.Errorf("--file needs a value")
			}
			file = args[i]
		default:
			return "", "", fmt.Errorf("unknown flag %s", args[i])
		}
	}
	if name == "" || file == "" {
		return "", "", fmt.Errorf("--name and --file are required")
	}
	return name, file, nil
}

func parseVolumeCreate(args []string) (string, uint64, error) {
	var name, sizeStr string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--name":
			i++
			if i >= len(args) {
				return "", 0, fmt.Errorf("--name needs a value")
			}
			name = args[i]
		case "--size":
			i++
			if i >= len(args) {
				return "", 0, fmt.Errorf("--size needs a value")
			}
			sizeStr = args[i]
		default:
			return "", 0, fmt.Errorf("unknown flag %s", args[i])
		}
	}
	if name == "" || sizeStr == "" {
		return "", 0, fmt.Errorf("--name and --size are required")
	}
	b, err := memory.ParseSize(sizeStr)
	if err != nil {
		return "", 0, err
	}
	return name, b, nil
}

func parseVolumeAttach(args []string) (string, string, error) {
	var vol, inst string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--volume":
			i++
			if i >= len(args) {
				return "", "", fmt.Errorf("--volume needs a value")
			}
			vol = args[i]
		case "--instance":
			i++
			if i >= len(args) {
				return "", "", fmt.Errorf("--instance needs a value")
			}
			inst = args[i]
		default:
			return "", "", fmt.Errorf("unknown flag %s", args[i])
		}
	}
	if vol == "" || inst == "" {
		return "", "", fmt.Errorf("--volume and --instance are required")
	}
	return vol, inst, nil
}

func parseVolumeDetach(args []string) (string, error) {
	var vol string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--volume":
			i++
			if i >= len(args) {
				return "", fmt.Errorf("--volume needs a value")
			}
			vol = args[i]
		default:
			if vol == "" && !strings.HasPrefix(args[i], "--") {
				vol = args[i]
				continue
			}
			return "", fmt.Errorf("unknown flag %s", args[i])
		}
	}
	if vol == "" {
		return "", fmt.Errorf("--volume is required")
	}
	return vol, nil
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
tiny instance list|inspect|stop|start|delete ...
tiny image import --name NAME --file PATH.tar.gz
tiny image list
tiny image inspect NAME
tiny image delete NAME
tiny volume create --name NAME --size SIZE
tiny volume list
tiny volume inspect NAME
tiny volume delete NAME
tiny volume attach --volume NAME --instance INST
tiny volume detach --volume NAME

Env: TINY_API_ADDR (default 127.0.0.1:8080). CLI talks only to tinyd over HTTP.
`)
}
