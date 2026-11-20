#pragma once

#include <cstdint>
#include <string>

namespace tiny::process {

struct AttachResult {
  bool ok = false;
  std::string device_path;
  std::string mount_path;
  std::string error_message;
};

AttachResult AttachVolumeToInstance(int32_t supervisor_pid, const std::string& disk_path,
                                    const std::string& rootfs_path, const std::string& mount_name);

bool DetachVolumeFromInstance(int32_t supervisor_pid, const std::string& device_path,
                              const std::string& rootfs_path, const std::string& mount_name,
                              std::string* err);

}  // namespace tiny::process
