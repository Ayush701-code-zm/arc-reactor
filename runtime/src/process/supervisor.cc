#include "tiny/process/supervisor.h"

#include "tiny/process/cgroup.h"
#include "tiny/process/namespace.h"

#include <chrono>
#include <csignal>
#include <cstdlib>
#include <cstring>
#include <thread>

#if defined(__linux__)
#include <sys/prctl.h>
#include <sys/wait.h>
#include <unistd.h>
#endif

namespace tiny::process {
namespace {

bool RequireCgroup() {
  const char* v = std::getenv("TINY_REQUIRE_CGROUP");
  return v != nullptr && std::string(v) == "1";
}

bool CanSkipCgroup(const std::string& err) {
  return err.find("Operation not supported") != std::string::npos ||
         err.find("no cgroup parent accepts") != std::string::npos ||
         err.find("cgroup.controllers lacks") != std::string::npos;
}

}  // namespace

std::string ToString(Status s) {
  switch (s) {
    case Status::kPending:
      return "PENDING";
    case Status::kRunning:
      return "RUNNING";
    case Status::kStopping:
      return "STOPPING";
    case Status::kStopped:
      return "STOPPED";
    case Status::kFailed:
      return "FAILED";
  }
  return "UNSPECIFIED";
}

Instance Supervisor::Create(const Spec& spec) {
  std::lock_guard lock(mu_);
  if (instances_.contains(spec.id)) {
    Instance failed;
    failed.spec = spec;
    failed.status = Status::kFailed;
    failed.error_message = "instance id already exists in runtime";
    return failed;
  }
  Instance inst;
  inst.spec = spec;
  inst.status = Status::kPending;
  auto started = StartLocked(inst);
  instances_[spec.id] = started;
  return started;
}

std::vector<Instance> Supervisor::List() const {
  std::lock_guard lock(mu_);
  std::vector<Instance> out;
  out.reserve(instances_.size());
  for (const auto& [_, inst] : instances_) out.push_back(inst);
  return out;
}

Instance Supervisor::Inspect(const std::string& id) const {
  std::lock_guard lock(mu_);
  auto it = instances_.find(id);
  if (it == instances_.end()) {
    Instance missing;
    missing.spec.id = id;
    missing.status = Status::kFailed;
    missing.error_message = "instance not found";
    return missing;
  }
  return it->second;
}

Instance Supervisor::Start(const std::string& id) {
  std::lock_guard lock(mu_);
  auto it = instances_.find(id);
  if (it == instances_.end()) {
    Instance missing;
    missing.spec.id = id;
    missing.status = Status::kFailed;
    missing.error_message = "instance not found";
    return missing;
  }
  if (it->second.status == Status::kRunning) return it->second;
  it->second = StartLocked(it->second);
  return it->second;
}

Instance Supervisor::Stop(const std::string& id) {
  std::lock_guard lock(mu_);
  auto it = instances_.find(id);
  if (it == instances_.end()) {
    Instance missing;
    missing.spec.id = id;
    missing.status = Status::kFailed;
    missing.error_message = "instance not found";
    return missing;
  }
  it->second = StopLocked(it->second);
  return it->second;
}

bool Supervisor::Delete(const std::string& id, std::string* err) {
  std::lock_guard lock(mu_);
  auto it = instances_.find(id);
  if (it == instances_.end()) {
    return true;
  }
  if (it->second.status == Status::kRunning || it->second.status == Status::kStopping) {
    it->second = StopLocked(it->second);
  }
  if (it->second.status != Status::kStopped && it->second.status != Status::kFailed &&
      it->second.status != Status::kPending) {
    if (err) *err = "instance not confirmed stopped before delete";
    return false;
  }
  CleanupCgroup(id);
  instances_.erase(it);
  return true;
}

bool Supervisor::RunningPid(const std::string& id, int32_t* pid, std::string* err) const {
  std::lock_guard lock(mu_);
  auto it = instances_.find(id);
  if (it == instances_.end()) {
    if (err) *err = "instance not found";
    return false;
  }
  if (it->second.status != Status::kRunning || it->second.pid <= 0) {
    if (err) *err = "instance is not running";
    return false;
  }
  if (pid) *pid = it->second.pid;
  return true;
}

Instance Supervisor::StartLocked(Instance& inst) {
#if !defined(__linux__)
  inst.status = Status::kFailed;
  inst.pid = 0;
  inst.error_message = "process runtime requires Linux (Ubuntu 24.04). Built on this host as a stub.";
  return inst;
#else
  std::string err;
  if (inst.spec.rootfs_path.empty()) {
    inst.status = Status::kFailed;
    inst.error_message = "rootfs_path is required";
    return inst;
  }
  bool cgroup_ok = ApplyLimits(inst.spec.id, inst.spec.cpu, inst.spec.memory_bytes, &err);
  if (!cgroup_ok) {
    if (RequireCgroup() || !CanSkipCgroup(err)) {
      inst.status = Status::kFailed;
      inst.error_message = err;
      return inst;
    }
  }

  const pid_t pid = fork();
  if (pid < 0) {
    inst.status = Status::kFailed;
    inst.error_message = "fork failed";
    return inst;
  }

  if (pid == 0) {
    setpgid(0, 0);
    if (cgroup_ok) {
      std::string local;
      if (!AddPid(inst.spec.id, getpid(), &local)) {
        _exit(110);
      }
    }
    std::string local;
    if (!EnterPidNamespace(&local)) {
      _exit(111);
    }
    const pid_t inner = fork();
    if (inner < 0) _exit(112);
    if (inner > 0) {
      int status = 0;
      waitpid(inner, &status, 0);
      _exit(WIFEXITED(status) ? WEXITSTATUS(status) : 113);
    }
    if (!EnterMountNamespace(&local)) {
      _exit(114);
    }
    if (!EnterNetNamespace(&local)) {
      _exit(116);
    }
    if (!BringUpLoopback(&local)) {
      _exit(117);
    }
    if (!ChrootRootfs(inst.spec.rootfs_path, &local)) {
      _exit(115);
    }
    prctl(PR_SET_PDEATHSIG, SIGKILL);
    execl("/bin/sleep", "sleep", "infinity", static_cast<char*>(nullptr));
    _exit(127);
  }

  inst.pid = static_cast<int32_t>(pid);
  inst.status = Status::kRunning;
  if (cgroup_ok) {
    inst.error_message.clear();
  } else {
    inst.error_message = "cgroup_limits=skipped: " + err;
  }
  return inst;
#endif
}

Instance Supervisor::StopLocked(Instance& inst) {
#if !defined(__linux__)
  inst.status = Status::kStopped;
  inst.pid = 0;
  return inst;
#else
  if (inst.pid <= 0) {
    inst.status = Status::kStopped;
    return inst;
  }
  inst.status = Status::kStopping;
  kill(-inst.pid, SIGTERM);

  const auto deadline = std::chrono::steady_clock::now() + std::chrono::milliseconds(kStopTimeoutMs);
  int status = 0;
  while (std::chrono::steady_clock::now() < deadline) {
    const pid_t r = waitpid(inst.pid, &status, WNOHANG);
    if (r == inst.pid) {
      inst.status = Status::kStopped;
      inst.pid = 0;
      return inst;
    }
    std::this_thread::sleep_for(std::chrono::milliseconds(50));
  }

  kill(-inst.pid, SIGKILL);
  waitpid(inst.pid, &status, 0);
  inst.status = Status::kStopped;
  inst.pid = 0;
  return inst;
#endif
}

void Supervisor::CleanupCgroup(const std::string& id) {
  std::string err;
  (void)RemoveCgroup(id, &err);
}

}  // namespace tiny::process
