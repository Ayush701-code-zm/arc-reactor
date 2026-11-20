package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/ayushanand/tiny-aws/control-plane/internal/image"
	"github.com/ayushanand/tiny-aws/control-plane/internal/instance"
	"github.com/ayushanand/tiny-aws/control-plane/internal/runtimeclient"
	"github.com/ayushanand/tiny-aws/control-plane/internal/store"
	"github.com/ayushanand/tiny-aws/control-plane/internal/types"
	"github.com/ayushanand/tiny-aws/control-plane/internal/volume"
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
	imageRoot := image.DefaultRoot()
	volRoot := volume.DefaultRoot()

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

	imgSvc := &image.Service{Store: st, ImageRoot: imageRoot}
	volSvc := &volume.Service{Store: st, Runtime: rt, Images: imgSvc, VolRoot: volRoot}
	svc := &instance.Service{Store: st, Runtime: rt, Images: imgSvc, Volumes: volSvc}
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
			if inst.ID == "" {
				writeErr(w, statusFor(err), err)
				return
			}
			writeJSON(w, http.StatusInternalServerError, inst)
			return
		}
		writeJSON(w, http.StatusOK, inst)
	})
	mux.HandleFunc("GET /v1/instances/{name}", func(w http.ResponseWriter, r *http.Request) {
		inst, err := svc.Inspect(r.Context(), r.PathValue("name"))
		if err != nil {
			writeErr(w, statusFor(err), err)
			return
		}
		writeJSON(w, http.StatusOK, inst)
	})
	mux.HandleFunc("POST /v1/instances/{name}/stop", func(w http.ResponseWriter, r *http.Request) {
		inst, err := svc.Stop(r.Context(), r.PathValue("name"))
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
		inst, err := svc.Start(r.Context(), r.PathValue("name"))
		if err != nil {
			writeErr(w, statusFor(err), err)
			return
		}
		writeJSON(w, http.StatusOK, inst)
	})
	mux.HandleFunc("DELETE /v1/instances/{name}", func(w http.ResponseWriter, r *http.Request) {
		if err := svc.Delete(r.Context(), r.PathValue("name")); err != nil {
			writeErr(w, statusFor(err), err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"deleted": true})
	})

	mux.HandleFunc("GET /v1/images", func(w http.ResponseWriter, r *http.Request) {
		list, err := imgSvc.List(r.Context())
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, list)
	})
	mux.HandleFunc("POST /v1/images", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Name string `json:"Name"`
			File string `json:"File"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeErr(w, http.StatusBadRequest, err)
			return
		}
		img, err := imgSvc.Import(r.Context(), body.Name, body.File)
		if err != nil {
			if img.ID == "" {
				writeErr(w, statusFor(err), err)
				return
			}
			writeJSON(w, http.StatusInternalServerError, img)
			return
		}
		writeJSON(w, http.StatusOK, img)
	})
	mux.HandleFunc("GET /v1/images/{name}", func(w http.ResponseWriter, r *http.Request) {
		img, err := imgSvc.Inspect(r.Context(), r.PathValue("name"))
		if err != nil {
			writeErr(w, statusFor(err), err)
			return
		}
		writeJSON(w, http.StatusOK, img)
	})
	mux.HandleFunc("DELETE /v1/images/{name}", func(w http.ResponseWriter, r *http.Request) {
		if err := imgSvc.Delete(r.Context(), r.PathValue("name")); err != nil {
			writeErr(w, statusFor(err), err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"deleted": true})
	})

	mux.HandleFunc("GET /v1/volumes", func(w http.ResponseWriter, r *http.Request) {
		list, err := volSvc.List(r.Context())
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, list)
	})
	mux.HandleFunc("POST /v1/volumes", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Name      string `json:"Name"`
			SizeBytes uint64 `json:"SizeBytes"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeErr(w, http.StatusBadRequest, err)
			return
		}
		vol, err := volSvc.Create(r.Context(), body.Name, body.SizeBytes)
		if err != nil {
			writeErr(w, statusFor(err), err)
			return
		}
		writeJSON(w, http.StatusOK, vol)
	})
	mux.HandleFunc("GET /v1/volumes/{name}", func(w http.ResponseWriter, r *http.Request) {
		vol, err := volSvc.Inspect(r.Context(), r.PathValue("name"))
		if err != nil {
			writeErr(w, statusFor(err), err)
			return
		}
		writeJSON(w, http.StatusOK, vol)
	})
	mux.HandleFunc("DELETE /v1/volumes/{name}", func(w http.ResponseWriter, r *http.Request) {
		if err := volSvc.Delete(r.Context(), r.PathValue("name")); err != nil {
			writeErr(w, statusFor(err), err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"deleted": true})
	})
	mux.HandleFunc("POST /v1/volumes/{name}/attach", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Instance string `json:"Instance"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeErr(w, http.StatusBadRequest, err)
			return
		}
		vol, err := volSvc.Attach(r.Context(), r.PathValue("name"), body.Instance)
		if err != nil {
			if vol.ID != "" {
				writeJSON(w, statusFor(err), vol)
				return
			}
			writeErr(w, statusFor(err), err)
			return
		}
		writeJSON(w, http.StatusOK, vol)
	})
	mux.HandleFunc("POST /v1/volumes/{name}/detach", func(w http.ResponseWriter, r *http.Request) {
		vol, err := volSvc.Detach(r.Context(), r.PathValue("name"))
		if err != nil {
			if vol.ID != "" {
				writeJSON(w, statusFor(err), vol)
				return
			}
			writeErr(w, statusFor(err), err)
			return
		}
		writeJSON(w, http.StatusOK, vol)
	})

	fmt.Printf("tinyd listening on http://%s (db=%s runtime=%s images=%s volumes=%s)\n", addr, dbPath, rtAddr, imageRoot, volRoot)
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
