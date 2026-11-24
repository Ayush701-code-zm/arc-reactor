#include "tiny/process/cgroup.h"

#include <cerrno>
#include <cstdlib>
#include <cstring>
#include <fcntl.h>
#include <fstream>
#include <sstream>
#include <sys/stat.h>
#include <unistd.h>
#include <vector>

namespace tiny::process {
namespace {

constexpr const char* kFsRoot = "/sys/fs/cgroup";

bool Exists(const std::string& path) {
  struct stat st {};
  return stat(path.c_str(), &st) == 0;
}

bool WriteFile(const std::string& path, const std::string& data, std::string* err) {
  const int fd = ::open(path.c_str(), O_WRONLY | O_TRUNC);
  if (fd < 0) {
    if (err) *err = "failed to open " + path + ": " + std::strerror(errno);
    return false;
  }
  const ssize_t n = ::write(fd, data.data(), data.size());
  const int werrno = errno;
  ::close(fd);
  if (n < 0 || static_cast<size_t>(n) != data.size()) {
    if (err) *err = "failed to write " + path + ": " + std::strerror(werrno);
    return false;
  }
  return true;
}

bool MkdirIfMissing(const std::string& path, std::string* err) {
  if (Exists(path)) return true;
  if (mkdir(path.c_str(), 0750) != 0) {
    if (err) *err = "mkdir " + path + " failed: " + std::strerror(errno);
    return false;
  }
  return true;
}

std::string ReadFile(const std::string& path) {
  std::ifstream in(path);
  if (!in) return "";
  return std::string((std::istreambuf_iterator<char>(in)), std::istreambuf_iterator<char>());
}

bool HasToken(const std::string& body, const std::string& token) {
  std::istringstream in(body);
  std::string t;
  while (in >> t) {
    if (t == token) return true;
  }
  return false;
}

std::string SelfCgroupRel() {
  std::ifstream in("/proc/self/cgroup");
  std::string line;
  while (std::getline(in, line)) {
    if (line.rfind("0::", 0) == 0) {
      auto rel = line.substr(3);
      if (rel.empty()) return "/";
      return rel;
    }
  }
  return "/";
}

std::string JoinCgroup(const std::string& rel) {
  if (rel == "/" || rel.empty()) return kFsRoot;
  if (rel.front() == '/') return std::string(kFsRoot) + rel;
  return std::string(kFsRoot) + "/" + rel;
}

std::string ParentDir(const std::string& path) {
  constexpr std::size_t kFsRootLen = sizeof("/sys/fs/cgroup") - 1;
  if (path == kFsRoot) return path;
  const auto pos = path.find_last_of('/');
  if (pos == std::string::npos || pos == 0) return kFsRoot;
  if (pos <= kFsRootLen) return kFsRoot;
  return path.substr(0, pos);
}

bool ControllersAvailable(const std::string& dir) {
  const auto body = ReadFile(dir + "/cgroup.controllers");
  return HasToken(body, "cpu") && HasToken(body, "memory");
}

bool ControllersEnabled(const std::string& parent) {
  const auto body = ReadFile(parent + "/cgroup.subtree_control");
  return HasToken(body, "cpu") && HasToken(body, "memory");
}

std::vector<int> ReadPids(const std::string& procs_path) {
  std::vector<int> pids;
  std::ifstream in(procs_path);
  int pid = 0;
  while (in >> pid) pids.push_back(pid);
  return pids;
}

bool VacateForDelegation(const std::string& parent, std::string* err) {
  if (ControllersEnabled(parent)) return true;
  const auto leaf = parent + "/tiny.init";
  if (!MkdirIfMissing(leaf, err)) return false;

  for (int round = 0; round < 8; ++round) {
    const auto pids = ReadPids(parent + "/cgroup.procs");
    if (pids.empty()) break;
    for (int pid : pids) {
      std::string local;
      (void)WriteFile(leaf + "/cgroup.procs", std::to_string(pid), &local);
    }
  }
  if (!ReadPids(parent + "/cgroup.procs").empty()) {
    std::string local;
    (void)WriteFile(leaf + "/cgroup.procs", std::to_string(::getpid()), &local);
  }
  return true;
}

bool EnableCpuMemory(const std::string& parent, std::string* err) {
  if (ControllersEnabled(parent)) return true;
  if (!ControllersAvailable(parent)) {
    if (err) {
      *err = parent + " cgroup.controllers lacks cpu/memory (got: " +
             ReadFile(parent + "/cgroup.controllers") + ")";
    }
    return false;
  }
  if (!VacateForDelegation(parent, err)) return false;

  std::string local;
  if (!WriteFile(parent + "/cgroup.subtree_control", "+cpu", &local)) {
    if (err) *err = local;
    return false;
  }
  if (!WriteFile(parent + "/cgroup.subtree_control", "+memory", &local)) {
    if (err) *err = local;
    return false;
  }
  if (!ControllersEnabled(parent)) {
    if (err) {
      *err = "subtree_control write ok but cpu/memory not listed on " + parent +
             " (subtree_control=" + ReadFile(parent + "/cgroup.subtree_control") + ")";
    }
    return false;
  }
  return true;
}

std::string FindDelegationParent(std::string* err) {
  std::string last_err;

  if (const char* forced = std::getenv("TINY_CGROUP_PARENT"); forced && *forced) {
    const std::string parent = forced;
    if (EnableCpuMemory(parent, &last_err)) {
      return parent;
    }
    if (err) {
      *err = "TINY_CGROUP_PARENT=" + parent + " failed: " + last_err;
    }
    return "";
  }

  std::vector<std::string> candidates;
  candidates.push_back(kFsRoot);
  std::string cur = JoinCgroup(SelfCgroupRel());
  for (int i = 0; i < 16; ++i) {
    if (cur != kFsRoot) candidates.push_back(cur);
    if (cur == kFsRoot) break;
    cur = ParentDir(cur);
  }

  for (const auto& candidate : candidates) {
    if (EnableCpuMemory(candidate, &last_err)) {
      return candidate;
    }
  }
  if (err) *err = "no cgroup parent accepts +cpu +memory (last: " + last_err + ")";
  return "";
}

std::string g_tiny_root;

std::string TinyRoot(std::string* err) {
  if (!g_tiny_root.empty()) return g_tiny_root;
  const auto parent = FindDelegationParent(err);
  if (parent.empty()) return "";
  const auto root = parent + "/tinyaws";
  if (!MkdirIfMissing(root, err)) return "";
  if (!EnableCpuMemory(root, err)) return "";
  g_tiny_root = root;
  return g_tiny_root;
}

}  // namespace

std::string CgroupPath(const std::string& id) {
  std::string err;
  const auto root = TinyRoot(&err);
  if (root.empty()) return std::string(kFsRoot) + "/tinyaws/" + id;
  return root + "/" + id;
}

bool EnsureCgroup(const std::string& id, std::string* err) {
#if !defined(__linux__)
  (void)id;
  if (err) *err = "cgroup v2 requires Linux";
  return false;
#else
  if (!Exists(std::string(kFsRoot) + "/cgroup.controllers")) {
    if (err) *err = "cgroup v2 not mounted at /sys/fs/cgroup";
    return false;
  }

  const auto root = TinyRoot(err);
  if (root.empty()) return false;

  const auto path = root + "/" + id;
  if (!MkdirIfMissing(path, err)) return false;

  if (!Exists(path + "/cpu.max") || !Exists(path + "/memory.max")) {
    if (err) {
      *err = path + " exists but cpu.max/memory.max missing (controllers not delegated)";
    }
    return false;
  }
  return true;
#endif
}

bool ApplyLimits(const std::string& id, uint32_t cpu, uint64_t memory_bytes, std::string* err) {
#if !defined(__linux__)
  (void)id;
  (void)cpu;
  (void)memory_bytes;
  if (err) *err = "cgroup v2 requires Linux";
  return false;
#else
  if (!EnsureCgroup(id, err)) return false;
  const auto path = CgroupPath(id);

  std::ostringstream cpu_max;
  cpu_max << (static_cast<uint64_t>(cpu) * 100000ULL) << " 100000";
  if (!WriteFile(path + "/cpu.max", cpu_max.str(), err)) return false;
  if (!WriteFile(path + "/memory.max", std::to_string(memory_bytes), err)) return false;
  return true;
#endif
}

bool AddPid(const std::string& id, int pid, std::string* err) {
#if !defined(__linux__)
  (void)id;
  (void)pid;
  if (err) *err = "cgroup v2 requires Linux";
  return false;
#else
  return WriteFile(CgroupPath(id) + "/cgroup.procs", std::to_string(pid), err);
#endif
}

bool RemoveCgroup(const std::string& id, std::string* err) {
#if !defined(__linux__)
  (void)id;
  if (err) *err = "cgroup v2 requires Linux";
  return false;
#else
  const auto path = CgroupPath(id);
  if (rmdir(path.c_str()) != 0) {
    if (err) *err = "rmdir " + path + " failed: " + std::strerror(errno);
    return false;
  }
  return true;
#endif
}

}  // namespace tiny::process
