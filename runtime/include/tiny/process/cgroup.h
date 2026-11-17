#pragma once

#include <cstdint>
#include <string>

namespace tiny::process {

// cgroup v2 helpers.
// Paths: <self-cgroup>/tinyaws/<id>/ under /sys/fs/cgroup (works inside Docker host).
std::string CgroupPath(const std::string& id);
bool EnsureCgroup(const std::string& id, std::string* err);
bool ApplyLimits(const std::string& id, uint32_t cpu, uint64_t memory_bytes, std::string* err);
bool AddPid(const std::string& id, int pid, std::string* err);
bool RemoveCgroup(const std::string& id, std::string* err);

}  // namespace tiny::process
