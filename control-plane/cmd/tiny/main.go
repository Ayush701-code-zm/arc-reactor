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
	case "auth":
		runAuth(client, base, os.Args[2:])
	case "instance":
		runInstance(client, base, os.Args[2:])
	case "image":
		runImage(client, base, os.Args[2:])
	case "volume":
		runVolume(client, base, os.Args[2:])
	case "network":
		runNetwork(client, base, os.Args[2:])
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
	case "metadata":
		name, path, err := parseInstanceMetadata(args[1:])
		if err != nil {
			fail(err)
		}
		if path == "" {
			var doc map[string]any
			callJSON(client, http.MethodGet, base+"/v1/instances/"+url.PathEscape(name)+"/metadata", nil, &doc, "metadata")
			return
		}
		rawURL := base + "/v1/instances/" + url.PathEscape(name) + "/meta-data/" + path
		code, body, err := doRaw(client, http.MethodGet, rawURL)
		if err != nil {
			fail(err)
		}
		fmt.Print(string(body))
		if code >= 300 {
			fail(fmt.Errorf("metadata failed: HTTP %d", code))
		}
	default:
		usage()
		os.Exit(2)
	}
}

func parseInstanceMetadata(args []string) (string, string, error) {
	if len(args) < 1 {
		return "", "", fmt.Errorf("expected <name> [path]")
	}
	name := args[0]
	path := ""
	if len(args) >= 2 {
		path = strings.Trim(args[1], "/")
	}
	if len(args) > 2 {
		return "", "", fmt.Errorf("expected <name> [path]")
	}
	return name, path, nil
}

func doRaw(client *http.Client, method, rawURL string) (int, []byte, error) {
	req, err := http.NewRequest(method, rawURL, nil)
	if err != nil {
		return 0, nil, err
	}
	if tok := os.Getenv("TINY_TOKEN"); tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	return resp.StatusCode, data, err
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
	if tok := os.Getenv("TINY_TOKEN"); tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
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
		case "--network":
			i++
			if i >= len(args) {
				return spec, fmt.Errorf("--network needs a value")
			}
			spec.Network = args[i]
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

func runNetwork(client *http.Client, base string, args []string) {
	if len(args) < 1 {
		usage()
		os.Exit(2)
	}
	switch args[0] {
	case "create":
		name, cidr, err := parseNetworkCreate(args[1:])
		if err != nil {
			fail(err)
		}
		body := map[string]string{"Name": name, "CIDR": cidr}
		var net types.Network
		callJSON(client, http.MethodPost, base+"/v1/networks", body, &net, "create")
	case "list":
		var list []types.Network
		callJSON(client, http.MethodGet, base+"/v1/networks", nil, &list, "list")
	case "inspect":
		name := mustName(args[1:])
		var net types.Network
		callJSON(client, http.MethodGet, base+"/v1/networks/"+url.PathEscape(name), nil, &net, "inspect")
	case "delete":
		name := mustName(args[1:])
		var out map[string]any
		callJSON(client, http.MethodDelete, base+"/v1/networks/"+url.PathEscape(name), nil, &out, "delete")
	case "attach":
		netName, instName, err := parseNetworkAttach(args[1:])
		if err != nil {
			fail(err)
		}
		body := map[string]string{"Instance": instName}
		var att types.NetworkAttachment
		callJSON(client, http.MethodPost, base+"/v1/networks/"+url.PathEscape(netName)+"/attach", body, &att, "attach")
	case "detach":
		netName, instName, err := parseNetworkAttach(args[1:])
		if err != nil {
			fail(err)
		}
		body := map[string]string{"Instance": instName}
		var out map[string]any
		callJSON(client, http.MethodPost, base+"/v1/networks/"+url.PathEscape(netName)+"/detach", body, &out, "detach")
	default:
		usage()
		os.Exit(2)
	}
}

func parseNetworkCreate(args []string) (string, string, error) {
	var name, cidr string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--name":
			i++
			if i >= len(args) {
				return "", "", fmt.Errorf("--name needs a value")
			}
			name = args[i]
		case "--cidr":
			i++
			if i >= len(args) {
				return "", "", fmt.Errorf("--cidr needs a value")
			}
			cidr = args[i]
		default:
			return "", "", fmt.Errorf("unknown flag %s", args[i])
		}
	}
	if name == "" || cidr == "" {
		return "", "", fmt.Errorf("--name and --cidr are required")
	}
	return name, cidr, nil
}

func parseNetworkAttach(args []string) (string, string, error) {
	var net, inst string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--network":
			i++
			if i >= len(args) {
				return "", "", fmt.Errorf("--network needs a value")
			}
			net = args[i]
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
	if net == "" || inst == "" {
		return "", "", fmt.Errorf("--network and --instance are required")
	}
	return net, inst, nil
}

func runAuth(client *http.Client, base string, args []string) {
	if len(args) < 1 {
		usage()
		os.Exit(2)
	}
	switch args[0] {
	case "bootstrap":
		user, pass, err := parseUserPass(args[1:])
		if err != nil {
			fail(err)
		}
		body := map[string]string{"Username": user, "Password": pass}
		var out types.User
		callJSON(client, http.MethodPost, base+"/v1/auth/bootstrap", body, &out, "bootstrap")
	case "login":
		user, pass, err := parseUserPass(args[1:])
		if err != nil {
			fail(err)
		}
		body := map[string]string{"Username": user, "Password": pass}
		var out map[string]any
		callJSON(client, http.MethodPost, base+"/v1/auth/login", body, &out, "login")
		if tok, ok := out["AccessToken"].(string); ok && tok != "" {
			fmt.Fprintf(os.Stderr, "export TINY_TOKEN=%s\n", tok)
		}
	case "refresh":
		rt, err := parseRefresh(args[1:])
		if err != nil {
			fail(err)
		}
		body := map[string]string{"RefreshToken": rt}
		var out map[string]any
		callJSON(client, http.MethodPost, base+"/v1/auth/refresh", body, &out, "refresh")
		if tok, ok := out["AccessToken"].(string); ok && tok != "" {
			fmt.Fprintf(os.Stderr, "export TINY_TOKEN=%s\n", tok)
		}
	case "logout":
		rt, _ := parseRefresh(args[1:])
		body := map[string]string{"RefreshToken": rt}
		var out map[string]any
		callJSON(client, http.MethodPost, base+"/v1/auth/logout", body, &out, "logout")
	case "whoami":
		var out types.User
		callJSON(client, http.MethodGet, base+"/v1/auth/whoami", nil, &out, "whoami")
	default:
		usage()
		os.Exit(2)
	}
}

func parseUserPass(args []string) (string, string, error) {
	var user, pass string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--username":
			i++
			if i >= len(args) {
				return "", "", fmt.Errorf("--username needs a value")
			}
			user = args[i]
		case "--password":
			i++
			if i >= len(args) {
				return "", "", fmt.Errorf("--password needs a value")
			}
			pass = args[i]
		default:
			return "", "", fmt.Errorf("unknown flag %s", args[i])
		}
	}
	if user == "" || pass == "" {
		return "", "", fmt.Errorf("--username and --password are required")
	}
	return user, pass, nil
}

func parseRefresh(args []string) (string, error) {
	var rt string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--refresh-token":
			i++
			if i >= len(args) {
				return "", fmt.Errorf("--refresh-token needs a value")
			}
			rt = args[i]
		default:
			return "", fmt.Errorf("unknown flag %s", args[i])
		}
	}
	return rt, nil
}

func usage() {
	fmt.Fprintf(os.Stderr, `tiny auth bootstrap --username U --password P
tiny auth login --username U --password P
tiny auth refresh --refresh-token T
tiny auth logout [--refresh-token T]
tiny auth whoami
tiny instance create --name NAME --image IMAGE --cpu N --memory SIZE [--network NAME]
tiny instance metadata NAME [path]
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
tiny network create --name NAME --cidr CIDR
tiny network list
tiny network inspect NAME
tiny network delete NAME
tiny network attach --network NAME --instance INST
tiny network detach --network NAME --instance INST

Env: TINY_API_ADDR (default 127.0.0.1:8080), TINY_TOKEN (Bearer access token).
`)
}
