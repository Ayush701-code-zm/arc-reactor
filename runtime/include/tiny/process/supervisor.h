#pragma once

#include <cstdint>
#include <mutex>
#include <string>
#include <unordered_map>
#include <vector>

namespace tiny::process {

enum class Status {
  kPending,
  kRunning,
  kStopping,
  kStopped,
  kFailed,
};

struct Spec {
  std::string id;
  std::string name;
  std::string image;
  std::string rootfs_path;
  uint32_t cpu = 0;
  uint64_t memory_bytes = 0;
};

struct Instance {
  Spec spec;
  Status status = Status::kPending;
  int32_t pid = 0;
  std::string error_message;
};

std::string ToString(Status s);

class Supervisor {
 public:
  static constexpr int kStopTimeoutMs = 5000;

  Instance Create(const Spec& spec);
  std::vector<Instance> List() const;
  Instance Inspect(const std::string& id) const;
  Instance Start(const std::string& id);
  Instance Stop(const std::string& id);
  bool Delete(const std::string& id, std::string* err);
  bool RunningPid(const std::string& id, int32_t* pid, std::string* err) const;

 private:
  Instance StartLocked(Instance& inst);
  Instance StopLocked(Instance& inst);
  void CleanupCgroup(const std::string& id);

  mutable std::mutex mu_;
  std::unordered_map<std::string, Instance> instances_;
};

}  // namespace tiny::process
