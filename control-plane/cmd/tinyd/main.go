package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/ayushanand/tiny-aws/control-plane/internal/instance"
	"github.com/ayushanand/tiny-aws/control-plane/internal/runtimeclient"
	"github.com/ayushanand/tiny-aws/control-plane/internal/store"
	"github.com/ayushanand/tiny-aws/control-plane/internal/types"
)

func main() {
	addr := env("TINY_API_ADDR", "127.0.0.1:8080")
	dbPath := os.Getenv("TINY_DB")
	if dbPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			fatal(err)
		}
		dbPath = filepath.Join(home, ".tiny-aws", "tiny.db")
	}
	rtAddr := env("TINY_RUNTIME_ADDR", "127.0.0.1:50051")

	st, err := store.Open(dbPath)
	if err != nil {
		fatal(err)
	}
	defer st.Close()

	rt, err := runtimeclient.Dial(rtAddr)
	if err != nil {
		fatal(err)
	}
	defer rt.Close()

	svc := &instance.Service{Store: st, Runtime: rt}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /v1/instances", func(w http.ResponseWriter, r *http.Request) {
		list, err := svc.List(r.Context())
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		writeJSON(w, list)
	})
	mux.HandleFunc("POST /v1/instances", func(w http.ResponseWriter, r *http.Request) {
		var spec types.Spec
		if err := json.NewDecoder(r.Body).Decode(&spec); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		inst, err := svc.Create(r.Context(), spec)
		if err != nil {
			http.Error(w, err.Error(), 500)
			_ = json.NewEncoder(w).Encode(inst)
			return
		}
		writeJSON(w, inst)
	})

	fmt.Printf("tinyd listening on http://%s (db=%s runtime=%s)\n", addr, dbPath, rtAddr)
	fatal(http.ListenAndServe(addr, mux))
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "tinyd: %v\n", err)
	os.Exit(1)
}
