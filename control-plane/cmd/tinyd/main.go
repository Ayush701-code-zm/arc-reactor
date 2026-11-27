package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/ayushanand/tiny-aws/control-plane/internal/auth"
	"github.com/ayushanand/tiny-aws/control-plane/internal/ebpfclient"
	"github.com/ayushanand/tiny-aws/control-plane/internal/image"
	"github.com/ayushanand/tiny-aws/control-plane/internal/instance"
	"github.com/ayushanand/tiny-aws/control-plane/internal/metadata"
	"github.com/ayushanand/tiny-aws/control-plane/internal/network"
	"github.com/ayushanand/tiny-aws/control-plane/internal/nodehub"
	"github.com/ayushanand/tiny-aws/control-plane/internal/runtimeclient"
	"github.com/ayushanand/tiny-aws/control-plane/internal/scheduler"
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
	nodeCtrlAddr := env("TINY_NODE_CONTROL_ADDR", "127.0.0.1:50052")
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

	hub := nodehub.New(st)
	if err := hub.Listen(nodeCtrlAddr); err != nil {
		fatal(fmt.Errorf("node control listen %s: %w", nodeCtrlAddr, err))
	}
	go func() {
		if err := hub.Serve(); err != nil {
			fmt.Fprintf(os.Stderr, "tinyd node control: %v\n", err)
		}
	}()
	defer hub.Stop()

	sched := &scheduler.Service{Store: st, Hub: hub}
	imgSvc := &image.Service{Store: st, ImageRoot: imageRoot, Nodes: hub}
	volSvc := &volume.Service{Store: st, Runtime: rt, Images: imgSvc, VolRoot: volRoot}
	netSvc := &network.Service{Store: st, Runtime: rt}
	ebpf := ebpfclient.Dial()
	svc := &instance.Service{
		Store: st, Runtime: rt, Scheduler: sched, Nodes: hub,
		Images: imgSvc, Volumes: volSvc, Networks: netSvc, Ebpf: ebpf,
	}
	authSvc := &auth.Service{Store: st}
	metaSvc := &metadata.Service{Store: st}
	if err := authSvc.MaybeSeed(context.Background(), os.Getenv("TINY_BOOTSTRAP_USER"), os.Getenv("TINY_BOOTSTRAP_PASSWORD")); err != nil {
		fatal(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /v1/nodes", func(w http.ResponseWriter, r *http.Request) {
		list, err := sched.List(r.Context())
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, list)
	})
	mux.HandleFunc("GET /v1/nodes/{id}", func(w http.ResponseWriter, r *http.Request) {
		n, err := st.GetNode(r.Context(), r.PathValue("id"))
		if err != nil {
			writeErr(w, statusFor(err), err)
			return
		}
		writeJSON(w, http.StatusOK, n)
	})

	mux.HandleFunc("POST /v1/auth/bootstrap", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Username string `json:"Username"`
			Password string `json:"Password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeErr(w, http.StatusBadRequest, err)
			return
		}
		user, err := authSvc.Bootstrap(r.Context(), body.Username, body.Password)
		if err != nil {
			writeErr(w, statusFor(err), err)
			return
		}
		writeJSON(w, http.StatusOK, user)
	})
	mux.HandleFunc("POST /v1/auth/login", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Username string `json:"Username"`
			Password string `json:"Password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeErr(w, http.StatusBadRequest, err)
			return
		}
		pair, err := authSvc.Login(r.Context(), body.Username, body.Password)
		if err != nil {
			writeErr(w, statusFor(err), err)
			return
		}
		writeJSON(w, http.StatusOK, pair)
	})
	mux.HandleFunc("POST /v1/auth/refresh", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			RefreshToken string `json:"RefreshToken"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeErr(w, http.StatusBadRequest, err)
			return
		}
		pair, err := authSvc.Refresh(r.Context(), body.RefreshToken)
		if err != nil {
			writeErr(w, statusFor(err), err)
			return
		}
		writeJSON(w, http.StatusOK, pair)
	})
	mux.HandleFunc("POST /v1/auth/logout", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			RefreshToken string `json:"RefreshToken"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		access, _ := auth.BearerToken(r.Header.Get("Authorization"))
		_ = authSvc.Logout(r.Context(), access, body.RefreshToken)
		writeJSON(w, http.StatusOK, map[string]bool{"logged_out": true})
	})
	mux.HandleFunc("GET /v1/auth/whoami", func(w http.ResponseWriter, r *http.Request) {
		user, err := authSvc.WhoAmI(r.Context())
		if err != nil {
			writeErr(w, statusFor(err), err)
			return
		}
		writeJSON(w, http.StatusOK, user)
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
	mux.HandleFunc("GET /v1/instances/{name}/metadata", func(w http.ResponseWriter, r *http.Request) {
		doc, err := metaSvc.DocumentByName(r.Context(), r.PathValue("name"))
		if err != nil {
			writeErr(w, statusFor(err), err)
			return
		}
		writeJSON(w, http.StatusOK, doc)
	})
	mux.HandleFunc("GET /v1/instances/{name}/meta-data/{path...}", func(w http.ResponseWriter, r *http.Request) {
		val, err := metaSvc.PathByName(r.Context(), r.PathValue("name"), r.PathValue("path"))
		if err != nil {
			writeErr(w, statusFor(err), err)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(val))
	})
	mux.HandleFunc("GET /v1/metadata/by-veth/{veth}", func(w http.ResponseWriter, r *http.Request) {
		doc, err := metaSvc.DocumentByVeth(r.Context(), r.PathValue("veth"))
		if err != nil {
			writeErr(w, statusFor(err), err)
			return
		}
		writeJSON(w, http.StatusOK, doc)
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
	mux.HandleFunc("POST /v1/images/{name}/place", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			NodeID string `json:"NodeID"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeErr(w, http.StatusBadRequest, err)
			return
		}
		ni, err := imgSvc.PlaceOnNode(r.Context(), r.PathValue("name"), body.NodeID)
		if err != nil {
			if ni.ImageID != "" {
				writeJSON(w, http.StatusInternalServerError, ni)
				return
			}
			writeErr(w, statusFor(err), err)
			return
		}
		writeJSON(w, http.StatusOK, ni)
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

	mux.HandleFunc("GET /v1/networks", func(w http.ResponseWriter, r *http.Request) {
		list, err := netSvc.List(r.Context())
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, list)
	})
	mux.HandleFunc("POST /v1/networks", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Name string `json:"Name"`
			CIDR string `json:"CIDR"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeErr(w, http.StatusBadRequest, err)
			return
		}
		net, err := netSvc.Create(r.Context(), body.Name, body.CIDR)
		if err != nil {
			writeErr(w, statusFor(err), err)
			return
		}
		writeJSON(w, http.StatusOK, net)
	})
	mux.HandleFunc("GET /v1/networks/{name}", func(w http.ResponseWriter, r *http.Request) {
		net, err := netSvc.Inspect(r.Context(), r.PathValue("name"))
		if err != nil {
			writeErr(w, statusFor(err), err)
			return
		}
		writeJSON(w, http.StatusOK, net)
	})
	mux.HandleFunc("DELETE /v1/networks/{name}", func(w http.ResponseWriter, r *http.Request) {
		if err := netSvc.Delete(r.Context(), r.PathValue("name")); err != nil {
			writeErr(w, statusFor(err), err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"deleted": true})
	})
	mux.HandleFunc("POST /v1/networks/{name}/attach", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Instance string `json:"Instance"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeErr(w, http.StatusBadRequest, err)
			return
		}
		att, err := netSvc.Attach(r.Context(), r.PathValue("name"), body.Instance)
		if err != nil {
			if att.NetworkID != "" {
				writeJSON(w, statusFor(err), att)
				return
			}
			writeErr(w, statusFor(err), err)
			return
		}
		writeJSON(w, http.StatusOK, att)
	})
	mux.HandleFunc("POST /v1/networks/{name}/detach", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Instance string `json:"Instance"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeErr(w, http.StatusBadRequest, err)
			return
		}
		if err := netSvc.Detach(r.Context(), r.PathValue("name"), body.Instance); err != nil {
			writeErr(w, statusFor(err), err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"detached": true})
	})

	fmt.Printf("tinyd listening on http://%s (db=%s runtime=%s nodes=%s images=%s volumes=%s)\n", addr, dbPath, rtAddr, nodeCtrlAddr, imageRoot, volRoot)
	fatal(http.ListenAndServe(addr, authSvc.Middleware(mux)))
}

func statusFor(err error) int {
	if err == nil {
		return http.StatusInternalServerError
	}
	switch {
	case errors.Is(err, auth.ErrUnauthorized), errors.Is(err, auth.ErrInvalidCreds):
		return http.StatusUnauthorized
	case errors.Is(err, auth.ErrForbidden):
		return http.StatusForbidden
	case errors.Is(err, auth.ErrConflict), strings.Contains(err.Error(), "conflict"):
		return http.StatusConflict
	case strings.Contains(err.Error(), "not RUNNING"):
		return http.StatusConflict
	case strings.Contains(err.Error(), "not found"):
		return http.StatusNotFound
	default:
		return http.StatusInternalServerError
	}
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
