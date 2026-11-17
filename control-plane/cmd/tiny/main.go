package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/ayushanand/tiny-aws/control-plane/internal/instance"
	"github.com/ayushanand/tiny-aws/control-plane/internal/memory"
	"github.com/ayushanand/tiny-aws/control-plane/internal/runtimeclient"
	"github.com/ayushanand/tiny-aws/control-plane/internal/store"
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

	ctx := context.Background()
	svc, cleanup, err := newService()
	if err != nil {
		fail(err)
	}
	defer cleanup()

	switch os.Args[2] {
	case "create":
		spec, err := parseCreate(os.Args[3:])
		if err != nil {
			fail(err)
		}
		inst, err := svc.Create(ctx, spec)
		if err != nil {
			// Still print instance if partially persisted.
			_ = printJSON(inst)
			fail(err)
		}
		_ = printJSON(inst)
	case "list":
		list, err := svc.List(ctx)
		if err != nil {
			fail(err)
		}
		_ = printJSON(list)
	case "inspect":
		name, err := oneName(os.Args[3:])
		if err != nil {
			fail(err)
		}
		inst, err := svc.Inspect(ctx, name)
		if err != nil {
			fail(err)
		}
		_ = printJSON(inst)
	case "stop":
		name, err := oneName(os.Args[3:])
		if err != nil {
			fail(err)
		}
		inst, err := svc.Stop(ctx, name)
		if err != nil {
			fail(err)
		}
		_ = printJSON(inst)
	case "start":
		name, err := oneName(os.Args[3:])
		if err != nil {
			fail(err)
		}
		inst, err := svc.Start(ctx, name)
		if err != nil {
			fail(err)
		}
		_ = printJSON(inst)
	case "delete":
		name, err := oneName(os.Args[3:])
		if err != nil {
			fail(err)
		}
		if err := svc.Delete(ctx, name); err != nil {
			fail(err)
		}
		fmt.Println(`{"deleted":true}`)
	default:
		usage()
		os.Exit(2)
	}
}

func newService() (*instance.Service, func(), error) {
	dbPath := os.Getenv("TINY_DB")
	if dbPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, nil, err
		}
		dbPath = filepath.Join(home, ".tiny-aws", "tiny.db")
	}
	st, err := store.Open(dbPath)
	if err != nil {
		return nil, nil, err
	}

	addr := os.Getenv("TINY_RUNTIME_ADDR")
	if addr == "" {
		addr = "127.0.0.1:50051"
	}
	rt, err := runtimeclient.Dial(addr)
	if err != nil {
		_ = st.Close()
		return nil, nil, err
	}

	svc := &instance.Service{Store: st, Runtime: rt}
	return svc, func() {
		_ = rt.Close()
		_ = st.Close()
	}, nil
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
`)
}
