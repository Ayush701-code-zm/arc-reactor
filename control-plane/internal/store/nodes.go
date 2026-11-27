package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/ayushanand/tiny-aws/control-plane/internal/types"
)

func (s *Store) UpsertNode(ctx context.Context, id, hostname, imageRoot string, cpu uint32, mem uint64, running uint32, connected bool) error {
	conn := 0
	if connected {
		conn = 1
	}
	_, err := s.db.ExecContext(ctx, `
INSERT INTO nodes (id, hostname, image_root, allocatable_cpu, allocatable_memory_bytes, running_instances, last_heartbeat, connected)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
  hostname = CASE WHEN excluded.hostname = '' THEN nodes.hostname ELSE excluded.hostname END,
  image_root = CASE WHEN excluded.image_root = '' THEN nodes.image_root ELSE excluded.image_root END,
  allocatable_cpu = excluded.allocatable_cpu,
  allocatable_memory_bytes = excluded.allocatable_memory_bytes,
  running_instances = excluded.running_instances,
  last_heartbeat = excluded.last_heartbeat,
  connected = excluded.connected`,
		id, hostname, imageRoot, cpu, mem, running, time.Now().UTC().Format(time.RFC3339Nano), conn)
	if err != nil {
		return fmt.Errorf("upsert node: %w", err)
	}
	return nil
}

func (s *Store) SetNodeConnected(ctx context.Context, id string, connected bool) error {
	conn := 0
	if connected {
		conn = 1
	}
	_, err := s.db.ExecContext(ctx, `UPDATE nodes SET connected = ? WHERE id = ?`, conn, id)
	return err
}

func (s *Store) ListNodes(ctx context.Context) ([]types.Node, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, hostname, COALESCE(image_root, ''), allocatable_cpu, allocatable_memory_bytes, running_instances, connected
FROM nodes ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []types.Node
	for rows.Next() {
		var n types.Node
		var conn int
		if err := rows.Scan(&n.ID, &n.Hostname, &n.ImageRoot, &n.AllocatableCPU, &n.AllocatableMemoryBytes, &n.RunningInstances, &conn); err != nil {
			return nil, err
		}
		n.Connected = conn != 0
		cpu, mem, err := s.AllocatedOnNode(ctx, n.ID)
		if err != nil {
			return nil, err
		}
		n.AllocatedCPU = cpu
		n.AllocatedMemoryBytes = mem
		out = append(out, n)
	}
	return out, rows.Err()
}

func (s *Store) GetNode(ctx context.Context, id string) (types.Node, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT id, hostname, COALESCE(image_root, ''), allocatable_cpu, allocatable_memory_bytes, running_instances, connected
FROM nodes WHERE id = ?`, id)
	var n types.Node
	var conn int
	err := row.Scan(&n.ID, &n.Hostname, &n.ImageRoot, &n.AllocatableCPU, &n.AllocatableMemoryBytes, &n.RunningInstances, &conn)
	if err == sql.ErrNoRows {
		return types.Node{}, fmt.Errorf("node not found")
	}
	if err != nil {
		return types.Node{}, err
	}
	n.Connected = conn != 0
	cpu, mem, err := s.AllocatedOnNode(ctx, n.ID)
	if err != nil {
		return types.Node{}, err
	}
	n.AllocatedCPU = cpu
	n.AllocatedMemoryBytes = mem
	return n, nil
}

func (s *Store) UpsertNodeImage(ctx context.Context, ni types.NodeImage) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO node_images (node_id, image_id, status, rootfs_path, error_message)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT(node_id, image_id) DO UPDATE SET
  status = excluded.status,
  rootfs_path = excluded.rootfs_path,
  error_message = excluded.error_message`,
		ni.NodeID, ni.ImageID, string(ni.Status), ni.RootfsPath, ni.ErrorMessage)
	if err != nil {
		return fmt.Errorf("upsert node image: %w", err)
	}
	return nil
}

func (s *Store) GetNodeImage(ctx context.Context, nodeID, imageID string) (types.NodeImage, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT node_id, image_id, status, rootfs_path, error_message
FROM node_images WHERE node_id = ? AND image_id = ?`, nodeID, imageID)
	var ni types.NodeImage
	var status string
	err := row.Scan(&ni.NodeID, &ni.ImageID, &status, &ni.RootfsPath, &ni.ErrorMessage)
	if err == sql.ErrNoRows {
		return types.NodeImage{}, fmt.Errorf("node image not found")
	}
	if err != nil {
		return types.NodeImage{}, err
	}
	ni.Status = types.NodeImageStatus(status)
	return ni, nil
}
