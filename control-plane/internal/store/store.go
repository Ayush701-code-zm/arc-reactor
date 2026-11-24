package store

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ayushanand/tiny-aws/control-plane/internal/types"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) migrate() error {
	_, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS instances (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL UNIQUE,
  image TEXT NOT NULL,
  rootfs_path TEXT NOT NULL DEFAULT '',
  cpu INTEGER NOT NULL,
  memory_bytes INTEGER NOT NULL,
  status TEXT NOT NULL,
  pid INTEGER NOT NULL DEFAULT 0,
  error_message TEXT NOT NULL DEFAULT ''
);
CREATE TABLE IF NOT EXISTS images (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL UNIQUE,
  artifact_path TEXT NOT NULL,
  rootfs_path TEXT NOT NULL,
  status TEXT NOT NULL,
  error_message TEXT NOT NULL DEFAULT ''
);
CREATE TABLE IF NOT EXISTS volumes (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL UNIQUE,
  size_bytes INTEGER NOT NULL,
  disk_path TEXT NOT NULL,
  status TEXT NOT NULL,
  attached_instance_id TEXT NOT NULL DEFAULT '',
  attached_instance TEXT NOT NULL DEFAULT '',
  device_path TEXT NOT NULL DEFAULT '',
  mount_name TEXT NOT NULL DEFAULT '',
  error_message TEXT NOT NULL DEFAULT ''
);
CREATE TABLE IF NOT EXISTS networks (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL UNIQUE,
  cidr TEXT NOT NULL,
  gateway_ip TEXT NOT NULL,
  bridge_name TEXT NOT NULL,
  status TEXT NOT NULL,
  error_message TEXT NOT NULL DEFAULT ''
);
CREATE TABLE IF NOT EXISTS network_attachments (
  network_id TEXT NOT NULL,
  instance_id TEXT NOT NULL,
  instance_name TEXT NOT NULL,
  guest_ip TEXT NOT NULL,
  guest_ip_cidr TEXT NOT NULL,
  veth_host TEXT NOT NULL,
  veth_guest TEXT NOT NULL,
  status TEXT NOT NULL,
  PRIMARY KEY (network_id, instance_id)
);`)
	if err != nil {
		return err
	}
	_, _ = s.db.Exec(`ALTER TABLE instances ADD COLUMN rootfs_path TEXT NOT NULL DEFAULT ''`)
	_, _ = s.db.Exec(`ALTER TABLE instances ADD COLUMN owner_id TEXT NOT NULL DEFAULT ''`)
	_, _ = s.db.Exec(`ALTER TABLE instances ADD COLUMN node_id TEXT NOT NULL DEFAULT ''`)
	_, _ = s.db.Exec(`ALTER TABLE images ADD COLUMN owner_id TEXT NOT NULL DEFAULT ''`)
	_, _ = s.db.Exec(`ALTER TABLE volumes ADD COLUMN owner_id TEXT NOT NULL DEFAULT ''`)
	_, _ = s.db.Exec(`ALTER TABLE networks ADD COLUMN owner_id TEXT NOT NULL DEFAULT ''`)
	_, err = s.db.Exec(`
CREATE TABLE IF NOT EXISTS users (
  id TEXT PRIMARY KEY,
  username TEXT NOT NULL UNIQUE,
  password_hash TEXT NOT NULL,
  role TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS access_sessions (
  token_hash TEXT PRIMARY KEY,
  user_id TEXT NOT NULL,
  expires_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS refresh_sessions (
  token_hash TEXT PRIMARY KEY,
  user_id TEXT NOT NULL,
  expires_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS nodes (
  id TEXT PRIMARY KEY,
  hostname TEXT NOT NULL DEFAULT '',
  allocatable_cpu INTEGER NOT NULL DEFAULT 0,
  allocatable_memory_bytes INTEGER NOT NULL DEFAULT 0,
  running_instances INTEGER NOT NULL DEFAULT 0,
  last_heartbeat TEXT NOT NULL DEFAULT '',
  connected INTEGER NOT NULL DEFAULT 0
);`)
	return err
}

func (s *Store) InsertImage(ctx context.Context, img types.Image) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO images (id, name, artifact_path, rootfs_path, status, error_message, owner_id)
VALUES (?, ?, ?, ?, ?, ?, ?)`,
		img.ID, img.Name, img.ArtifactPath, img.RootfsPath, string(img.Status), img.ErrorMessage, img.OwnerID)
	if err != nil {
		return fmt.Errorf("insert image: %w", err)
	}
	return nil
}

func (s *Store) GetImageByName(ctx context.Context, name string) (types.Image, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT id, name, artifact_path, rootfs_path, status, error_message, owner_id FROM images WHERE name = ?`, name)
	var img types.Image
	var status string
	err := row.Scan(&img.ID, &img.Name, &img.ArtifactPath, &img.RootfsPath, &status, &img.ErrorMessage, &img.OwnerID)
	if err == sql.ErrNoRows {
		return types.Image{}, fmt.Errorf("image not found")
	}
	if err != nil {
		return types.Image{}, err
	}
	img.Status = types.ImageStatus(status)
	return img, nil
}

func (s *Store) ListImages(ctx context.Context) ([]types.Image, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, name, artifact_path, rootfs_path, status, error_message, owner_id FROM images ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []types.Image
	for rows.Next() {
		var img types.Image
		var status string
		if err := rows.Scan(&img.ID, &img.Name, &img.ArtifactPath, &img.RootfsPath, &status, &img.ErrorMessage, &img.OwnerID); err != nil {
			return nil, err
		}
		img.Status = types.ImageStatus(status)
		out = append(out, img)
	}
	return out, rows.Err()
}

func (s *Store) DeleteImage(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM images WHERE id = ?`, id)
	return err
}

func (s *Store) InsertVolume(ctx context.Context, vol types.Volume) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO volumes (
  id, name, size_bytes, disk_path, status, attached_instance_id, attached_instance, device_path, mount_name, error_message, owner_id
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		vol.ID, vol.Name, vol.SizeBytes, vol.DiskPath, string(vol.Status),
		vol.AttachedInstanceID, vol.AttachedInstance, vol.DevicePath, vol.MountName, vol.ErrorMessage, vol.OwnerID)
	if err != nil {
		return fmt.Errorf("insert volume: %w", err)
	}
	return nil
}

func (s *Store) GetVolumeByName(ctx context.Context, name string) (types.Volume, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT id, name, size_bytes, disk_path, status, attached_instance_id, attached_instance, device_path, mount_name, error_message, owner_id
FROM volumes WHERE name = ?`, name)
	return scanVolume(row)
}

func (s *Store) GetVolumeByID(ctx context.Context, id string) (types.Volume, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT id, name, size_bytes, disk_path, status, attached_instance_id, attached_instance, device_path, mount_name, error_message, owner_id
FROM volumes WHERE id = ?`, id)
	return scanVolume(row)
}

func (s *Store) ListVolumes(ctx context.Context) ([]types.Volume, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, name, size_bytes, disk_path, status, attached_instance_id, attached_instance, device_path, mount_name, error_message, owner_id
FROM volumes ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []types.Volume
	for rows.Next() {
		vol, err := scanVolume(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, vol)
	}
	return out, rows.Err()
}

func (s *Store) ListVolumesByInstanceID(ctx context.Context, instanceID string) ([]types.Volume, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, name, size_bytes, disk_path, status, attached_instance_id, attached_instance, device_path, mount_name, error_message, owner_id
FROM volumes WHERE attached_instance_id = ? AND status = ? ORDER BY name`,
		instanceID, string(types.VolumeStatusAttached))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []types.Volume
	for rows.Next() {
		vol, err := scanVolume(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, vol)
	}
	return out, rows.Err()
}

func (s *Store) UpdateVolumeAttach(ctx context.Context, id string, status types.VolumeStatus, instanceID, instanceName, devicePath, mountName, errMsg string) error {
	res, err := s.db.ExecContext(ctx, `
UPDATE volumes SET status = ?, attached_instance_id = ?, attached_instance = ?, device_path = ?, mount_name = ?, error_message = ?
WHERE id = ?`, string(status), instanceID, instanceName, devicePath, mountName, errMsg, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("volume %s not found", id)
	}
	return nil
}

func (s *Store) DeleteVolume(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM volumes WHERE id = ?`, id)
	return err
}

func scanVolume(r rowScanner) (types.Volume, error) {
	var vol types.Volume
	var status string
	err := r.Scan(&vol.ID, &vol.Name, &vol.SizeBytes, &vol.DiskPath, &status,
		&vol.AttachedInstanceID, &vol.AttachedInstance, &vol.DevicePath, &vol.MountName, &vol.ErrorMessage, &vol.OwnerID)
	if err == sql.ErrNoRows {
		return types.Volume{}, fmt.Errorf("volume not found")
	}
	if err != nil {
		return types.Volume{}, err
	}
	vol.Status = types.VolumeStatus(status)
	return vol, nil
}

func (s *Store) InsertNetwork(ctx context.Context, net types.Network) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO networks (id, name, cidr, gateway_ip, bridge_name, status, error_message, owner_id)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		net.ID, net.Name, net.CIDR, net.GatewayIP, net.BridgeName, string(net.Status), net.ErrorMessage, net.OwnerID)
	if err != nil {
		return fmt.Errorf("insert network: %w", err)
	}
	return nil
}

func (s *Store) GetNetworkByName(ctx context.Context, name string) (types.Network, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT id, name, cidr, gateway_ip, bridge_name, status, error_message, owner_id FROM networks WHERE name = ?`, name)
	return scanNetwork(row)
}

func (s *Store) GetNetworkByID(ctx context.Context, id string) (types.Network, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT id, name, cidr, gateway_ip, bridge_name, status, error_message, owner_id FROM networks WHERE id = ?`, id)
	return scanNetwork(row)
}

func (s *Store) ListNetworks(ctx context.Context) ([]types.Network, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, name, cidr, gateway_ip, bridge_name, status, error_message, owner_id FROM networks ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []types.Network
	for rows.Next() {
		net, err := scanNetwork(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, net)
	}
	return out, rows.Err()
}

func (s *Store) DeleteNetwork(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM networks WHERE id = ?`, id)
	return err
}

func (s *Store) CountAttachmentsByNetwork(ctx context.Context, networkID string) (int, error) {
	row := s.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM network_attachments WHERE network_id = ?`, networkID)
	var n int
	if err := row.Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

func (s *Store) InsertAttachment(ctx context.Context, a types.NetworkAttachment) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO network_attachments (
  network_id, instance_id, instance_name, guest_ip, guest_ip_cidr, veth_host, veth_guest, status
) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		a.NetworkID, a.InstanceID, a.InstanceName, a.GuestIP, a.GuestIPCIDR, a.VethHost, a.VethGuest, a.Status)
	if err != nil {
		return fmt.Errorf("insert network attachment: %w", err)
	}
	return nil
}

func (s *Store) DeleteAttachment(ctx context.Context, networkID, instanceID string) error {
	_, err := s.db.ExecContext(ctx, `
DELETE FROM network_attachments WHERE network_id = ? AND instance_id = ?`, networkID, instanceID)
	return err
}

func (s *Store) GetAttachment(ctx context.Context, networkID, instanceID string) (types.NetworkAttachment, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT network_id, instance_id, instance_name, guest_ip, guest_ip_cidr, veth_host, veth_guest, status
FROM network_attachments WHERE network_id = ? AND instance_id = ?`, networkID, instanceID)
	return scanAttachment(row)
}

func (s *Store) GetAttachmentByVethHost(ctx context.Context, vethHost string) (types.NetworkAttachment, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT network_id, instance_id, instance_name, guest_ip, guest_ip_cidr, veth_host, veth_guest, status
FROM network_attachments WHERE veth_host = ?`, vethHost)
	return scanAttachment(row)
}

func (s *Store) ListAttachmentsByNetwork(ctx context.Context, networkID string) ([]types.NetworkAttachment, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT network_id, instance_id, instance_name, guest_ip, guest_ip_cidr, veth_host, veth_guest, status
FROM network_attachments WHERE network_id = ? ORDER BY guest_ip`, networkID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []types.NetworkAttachment
	for rows.Next() {
		a, err := scanAttachment(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *Store) ListAttachmentsByInstance(ctx context.Context, instanceID string) ([]types.NetworkAttachment, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT network_id, instance_id, instance_name, guest_ip, guest_ip_cidr, veth_host, veth_guest, status
FROM network_attachments WHERE instance_id = ? ORDER BY network_id`, instanceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []types.NetworkAttachment
	for rows.Next() {
		a, err := scanAttachment(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func scanNetwork(r rowScanner) (types.Network, error) {
	var net types.Network
	var status string
	err := r.Scan(&net.ID, &net.Name, &net.CIDR, &net.GatewayIP, &net.BridgeName, &status, &net.ErrorMessage, &net.OwnerID)
	if err == sql.ErrNoRows {
		return types.Network{}, fmt.Errorf("network not found")
	}
	if err != nil {
		return types.Network{}, err
	}
	net.Status = types.NetworkStatus(status)
	return net, nil
}

func scanAttachment(r rowScanner) (types.NetworkAttachment, error) {
	var a types.NetworkAttachment
	err := r.Scan(&a.NetworkID, &a.InstanceID, &a.InstanceName, &a.GuestIP, &a.GuestIPCIDR, &a.VethHost, &a.VethGuest, &a.Status)
	if err == sql.ErrNoRows {
		return types.NetworkAttachment{}, fmt.Errorf("network attachment not found")
	}
	if err != nil {
		return types.NetworkAttachment{}, err
	}
	return a, nil
}

func (s *Store) CountInstancesByImage(ctx context.Context, imageName string) (int, error) {
	row := s.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM instances WHERE image = ?`, imageName)
	var n int
	if err := row.Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

func (s *Store) InsertPending(ctx context.Context, inst types.Instance) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO instances (id, name, image, rootfs_path, cpu, memory_bytes, status, pid, error_message, owner_id, node_id)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		inst.ID, inst.Name, inst.Image, inst.RootfsPath, inst.CPU, inst.MemoryBytes, string(inst.Status), inst.PID, inst.ErrorMessage, inst.OwnerID, inst.NodeID)
	if err != nil {
		return fmt.Errorf("insert instance: %w", err)
	}
	return nil
}

func (s *Store) UpdateLifecycle(ctx context.Context, id string, status types.Status, pid int32, errMsg string) error {
	res, err := s.db.ExecContext(ctx, `
UPDATE instances SET status = ?, pid = ?, error_message = ? WHERE id = ?`,
		string(status), pid, errMsg, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("instance %s not found", id)
	}
	return nil
}

func (s *Store) Delete(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM instances WHERE id = ?`, id)
	return err
}

func (s *Store) GetByName(ctx context.Context, name string) (types.Instance, error) {
	return s.scanOne(ctx, `SELECT id, name, image, rootfs_path, cpu, memory_bytes, status, pid, error_message, owner_id, node_id FROM instances WHERE name = ?`, name)
}

func (s *Store) GetByID(ctx context.Context, id string) (types.Instance, error) {
	return s.scanOne(ctx, `SELECT id, name, image, rootfs_path, cpu, memory_bytes, status, pid, error_message, owner_id, node_id FROM instances WHERE id = ?`, id)
}

func (s *Store) List(ctx context.Context) ([]types.Instance, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, name, image, rootfs_path, cpu, memory_bytes, status, pid, error_message, owner_id, node_id FROM instances ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []types.Instance
	for rows.Next() {
		inst, err := scanRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, inst)
	}
	return out, rows.Err()
}

func (s *Store) AllocatedOnNode(ctx context.Context, nodeID string) (cpu uint32, mem uint64, err error) {
	row := s.db.QueryRowContext(ctx, `
SELECT COALESCE(SUM(cpu), 0), COALESCE(SUM(memory_bytes), 0)
FROM instances
WHERE node_id = ? AND status IN ('PENDING', 'RUNNING', 'STOPPING')`, nodeID)
	var c int64
	var m int64
	if err := row.Scan(&c, &m); err != nil {
		return 0, 0, err
	}
	return uint32(c), uint64(m), nil
}

func (s *Store) scanOne(ctx context.Context, q string, arg any) (types.Instance, error) {
	row := s.db.QueryRowContext(ctx, q, arg)
	var inst types.Instance
	var status string
	err := row.Scan(&inst.ID, &inst.Name, &inst.Image, &inst.RootfsPath, &inst.CPU, &inst.MemoryBytes, &status, &inst.PID, &inst.ErrorMessage, &inst.OwnerID, &inst.NodeID)
	if err == sql.ErrNoRows {
		return types.Instance{}, fmt.Errorf("instance not found")
	}
	if err != nil {
		return types.Instance{}, err
	}
	inst.Status = types.Status(status)
	return inst, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanRow(r rowScanner) (types.Instance, error) {
	var inst types.Instance
	var status string
	if err := r.Scan(&inst.ID, &inst.Name, &inst.Image, &inst.RootfsPath, &inst.CPU, &inst.MemoryBytes, &status, &inst.PID, &inst.ErrorMessage, &inst.OwnerID, &inst.NodeID); err != nil {
		return types.Instance{}, err
	}
	inst.Status = types.Status(status)
	return inst, nil
}
