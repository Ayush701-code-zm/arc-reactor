package nodehub

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	tinyv1 "github.com/ayushanand/tiny-aws/control-plane/gen/tiny/v1"
	"github.com/ayushanand/tiny-aws/control-plane/internal/store"
	"github.com/ayushanand/tiny-aws/control-plane/internal/types"
	"github.com/google/uuid"
	"google.golang.org/grpc"
)

type session struct {
	nodeID string
	stream tinyv1.NodeControlService_SessionServer
	sendMu sync.Mutex
	pending map[string]chan *tinyv1.RuntimeOpResult
}

type Hub struct {
	tinyv1.UnimplementedNodeControlServiceServer
	Store *store.Store

	mu       sync.Mutex
	sessions map[string]*session
	order    []string
	grpcSrv  *grpc.Server
	lis      net.Listener
}

func New(st *store.Store) *Hub {
	return &Hub{
		Store:    st,
		sessions: make(map[string]*session),
	}
}

func (h *Hub) Listen(addr string) error {
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	h.lis = lis
	h.grpcSrv = grpc.NewServer()
	tinyv1.RegisterNodeControlServiceServer(h.grpcSrv, h)
	return nil
}

func (h *Hub) Serve() error {
	if h.lis == nil || h.grpcSrv == nil {
		return fmt.Errorf("node hub not listening")
	}
	return h.grpcSrv.Serve(h.lis)
}

func (h *Hub) ListenAndServe(addr string) error {
	if err := h.Listen(addr); err != nil {
		return err
	}
	return h.Serve()
}

func (h *Hub) Stop() {
	if h.grpcSrv != nil {
		h.grpcSrv.GracefulStop()
	}
	if h.lis != nil {
		_ = h.lis.Close()
	}
}

func (h *Hub) ConnectedIDs() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]string, len(h.order))
	copy(out, h.order)
	return out
}

func (h *Hub) HasConnected() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.sessions) > 0
}

func (h *Hub) Session(stream tinyv1.NodeControlService_SessionServer) error {
	var sess *session
	defer func() {
		if sess == nil {
			return
		}
		h.mu.Lock()
		if cur, ok := h.sessions[sess.nodeID]; ok && cur == sess {
			delete(h.sessions, sess.nodeID)
			next := h.order[:0]
			for _, id := range h.order {
				if id != sess.nodeID {
					next = append(next, id)
				}
			}
			h.order = next
		}
		h.mu.Unlock()
		_ = h.Store.SetNodeConnected(context.Background(), sess.nodeID, false)
		for _, ch := range sess.pending {
			close(ch)
		}
	}()

	for {
		msg, err := stream.Recv()
		if err != nil {
			return err
		}
		switch body := msg.Body.(type) {
		case *tinyv1.AgentToControl_Register:
			reg := body.Register
			nodeID := reg.GetNodeId()
			if nodeID == "" {
				nodeID = uuid.NewString()
			}
			if err := h.Store.UpsertNode(stream.Context(), nodeID, reg.GetHostname(), reg.GetImageRoot(),
				reg.GetAllocatableCpu(), reg.GetAllocatableMemoryBytes(), 0, true); err != nil {
				_ = stream.Send(&tinyv1.ControlToAgent{
					Body: &tinyv1.ControlToAgent_RegisterAck{
						RegisterAck: &tinyv1.RegisterAck{NodeId: nodeID, ErrorMessage: err.Error()},
					},
				})
				return err
			}
			sess = &session{
				nodeID:  nodeID,
				stream:  stream,
				pending: make(map[string]chan *tinyv1.RuntimeOpResult),
			}
			h.mu.Lock()
			if old, ok := h.sessions[nodeID]; ok {
				delete(h.sessions, nodeID)
				_ = old
			}
			h.sessions[nodeID] = sess
			found := false
			for _, id := range h.order {
				if id == nodeID {
					found = true
					break
				}
			}
			if !found {
				h.order = append(h.order, nodeID)
			}
			h.mu.Unlock()
			if err := stream.Send(&tinyv1.ControlToAgent{
				Body: &tinyv1.ControlToAgent_RegisterAck{
					RegisterAck: &tinyv1.RegisterAck{NodeId: nodeID},
				},
			}); err != nil {
				return err
			}
		case *tinyv1.AgentToControl_Heartbeat:
			hb := body.Heartbeat
			if sess == nil || hb.GetNodeId() != sess.nodeID {
				continue
			}
			_ = h.Store.UpsertNode(stream.Context(), sess.nodeID, "", "",
				hb.GetAllocatableCpu(), hb.GetAllocatableMemoryBytes(), hb.GetRunningInstances(), true)
		case *tinyv1.AgentToControl_OpResult:
			if sess == nil {
				continue
			}
			res := body.OpResult
			sess.sendMu.Lock()
			ch, ok := sess.pending[res.GetRequestId()]
			if ok {
				delete(sess.pending, res.GetRequestId())
			}
			sess.sendMu.Unlock()
			if ok {
				ch <- res
				close(ch)
			}
		}
	}
}

func (h *Hub) CreateInstance(ctx context.Context, nodeID string, inst types.Instance) (types.RuntimeView, error) {
	return h.roundTrip(ctx, nodeID, &tinyv1.RuntimeOpRequest{
		RequestId: uuid.NewString(),
		Op: &tinyv1.RuntimeOpRequest_Create{
			Create: &tinyv1.CreateInstanceRequest{
				Id:          inst.ID,
				Name:        inst.Name,
				Image:       inst.Image,
				Cpu:         inst.CPU,
				MemoryBytes: inst.MemoryBytes,
				RootfsPath:  inst.RootfsPath,
				ImageId:     inst.ImageID,
			},
		},
	})
}

func (h *Hub) PlaceImage(ctx context.Context, nodeID, imageID, artifactPath, rootfsSource string) error {
	view, err := h.roundTrip(ctx, nodeID, &tinyv1.RuntimeOpRequest{
		RequestId: uuid.NewString(),
		Op: &tinyv1.RuntimeOpRequest_PlaceImage{
			PlaceImage: &tinyv1.PlaceImageRequest{
				ImageId:          imageID,
				ArtifactPath:     artifactPath,
				RootfsSourcePath: rootfsSource,
			},
		},
	})
	if err != nil {
		return err
	}
	if view.ErrorMessage != "" {
		return fmt.Errorf("%s", view.ErrorMessage)
	}
	return nil
}

func (h *Hub) StartInstance(ctx context.Context, nodeID, id string) (types.RuntimeView, error) {
	return h.roundTrip(ctx, nodeID, &tinyv1.RuntimeOpRequest{
		RequestId: uuid.NewString(),
		Op:        &tinyv1.RuntimeOpRequest_Start{Start: &tinyv1.StartInstanceRequest{Id: id}},
	})
}

func (h *Hub) StopInstance(ctx context.Context, nodeID, id string) (types.RuntimeView, error) {
	return h.roundTrip(ctx, nodeID, &tinyv1.RuntimeOpRequest{
		RequestId: uuid.NewString(),
		Op:        &tinyv1.RuntimeOpRequest_Stop{Stop: &tinyv1.StopInstanceRequest{Id: id}},
	})
}

func (h *Hub) DeleteInstance(ctx context.Context, nodeID, id string) error {
	_, err := h.roundTrip(ctx, nodeID, &tinyv1.RuntimeOpRequest{
		RequestId: uuid.NewString(),
		Op:        &tinyv1.RuntimeOpRequest_Delete{Delete: &tinyv1.DeleteInstanceRequest{Id: id}},
	})
	return err
}

func (h *Hub) InspectInstance(ctx context.Context, nodeID, id string) (types.RuntimeView, error) {
	return h.roundTrip(ctx, nodeID, &tinyv1.RuntimeOpRequest{
		RequestId: uuid.NewString(),
		Op:        &tinyv1.RuntimeOpRequest_Inspect{Inspect: &tinyv1.InspectInstanceRequest{Id: id}},
	})
}

func (h *Hub) roundTrip(ctx context.Context, nodeID string, req *tinyv1.RuntimeOpRequest) (types.RuntimeView, error) {
	h.mu.Lock()
	sess := h.sessions[nodeID]
	h.mu.Unlock()
	if sess == nil {
		return types.RuntimeView{}, fmt.Errorf("node %s not connected", nodeID)
	}
	ch := make(chan *tinyv1.RuntimeOpResult, 1)
	sess.sendMu.Lock()
	sess.pending[req.RequestId] = ch
	err := sess.stream.Send(&tinyv1.ControlToAgent{
		Body: &tinyv1.ControlToAgent_Op{Op: req},
	})
	sess.sendMu.Unlock()
	if err != nil {
		sess.sendMu.Lock()
		delete(sess.pending, req.RequestId)
		sess.sendMu.Unlock()
		return types.RuntimeView{}, err
	}

	timer := time.NewTimer(30 * time.Second)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return types.RuntimeView{}, ctx.Err()
	case <-timer.C:
		sess.sendMu.Lock()
		delete(sess.pending, req.RequestId)
		sess.sendMu.Unlock()
		return types.RuntimeView{}, fmt.Errorf("node %s op timeout", nodeID)
	case res, ok := <-ch:
		if !ok || res == nil {
			return types.RuntimeView{}, fmt.Errorf("node %s disconnected", nodeID)
		}
		if res.GetErrorMessage() != "" && res.GetStatus() == tinyv1.InstanceStatus_INSTANCE_STATUS_UNSPECIFIED {
			return types.RuntimeView{}, fmt.Errorf("%s", res.GetErrorMessage())
		}
		return types.RuntimeView{
			ID:           res.GetId(),
			Status:       statusFromProto(res.GetStatus()),
			PID:          res.GetPid(),
			ErrorMessage: res.GetErrorMessage(),
		}, nil
	}
}

func statusFromProto(s tinyv1.InstanceStatus) types.Status {
	switch s {
	case tinyv1.InstanceStatus_INSTANCE_STATUS_PENDING:
		return types.StatusPending
	case tinyv1.InstanceStatus_INSTANCE_STATUS_RUNNING:
		return types.StatusRunning
	case tinyv1.InstanceStatus_INSTANCE_STATUS_STOPPING:
		return types.StatusStopping
	case tinyv1.InstanceStatus_INSTANCE_STATUS_STOPPED:
		return types.StatusStopped
	case tinyv1.InstanceStatus_INSTANCE_STATUS_FAILED:
		return types.StatusFailed
	default:
		return ""
	}
}
