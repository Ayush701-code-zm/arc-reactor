package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

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
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /v1/instances", func(w http.ResponseWriter, r *http.Request) {
		list, err := svc.List(r.Context())
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, list)
	})
	mux.HandleFunc("POST /v1/instances", func(w http.ResponseWriter, r *http.Request) {
		var spec types.Spec
		if err := json.NewDecoder(r.Body).Decode(&spec); err != nil {
			writeErr(w, http.StatusBadRequest, err)
			return
		}
		inst, err := svc.Create(r.Context(), spec)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, inst)
			return
		}
		writeJSON(w, http.StatusOK, inst)
	})
	mux.HandleFunc("GET /v1/instances/{name}", func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")
		inst, err := svc.Inspect(r.Context(), name)
		if err != nil {
			writeErr(w, statusFor(err), err)
			return
		}
		writeJSON(w, http.StatusOK, inst)
	})
	mux.HandleFunc("POST /v1/instances/{name}/stop", func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")
		inst, err := svc.Stop(r.Context(), name)
		if err != nil {
			if inst.ID != "" {
				writeJSON(w, statusFor(err), inst)
				return
			}
			writeErr(w, statusFor(err), err)
			return
		}
		writeJSON(w, http.StatusOK, inst)
	})
	mux.HandleFunc("POST /v1/instances/{name}/start", func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")
		inst, err := svc.Start(r.Context(), name)
		if err != nil {
			writeErr(w, statusFor(err), err)
			return
		}
		writeJSON(w, http.StatusOK, inst)
	})
	mux.HandleFunc("DELETE /v1/instances/{name}", func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")
		if err := svc.Delete(r.Context(), name); err != nil {
			writeErr(w, statusFor(err), err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"deleted": true})
	})

	fmt.Printf("tinyd listening on http://%s (db=%s runtime=%s)\n", addr, dbPath, rtAddr)
	fatal(http.ListenAndServe(addr, mux))
}

func statusFor(err error) int {
	if err != nil && strings.Contains(err.Error(), "not found") {
		return http.StatusNotFound
	}
	return http.StatusInternalServerError
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, err error) {
	writeJSON(w, code, map[string]string{"error": err.Error()})
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
