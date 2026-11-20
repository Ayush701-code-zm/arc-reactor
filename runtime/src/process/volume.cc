#include "tiny/process/volume.h"

#include <cstdint>
#include <fstream>
#include <functional>
#include <string>

#if defined(__linux__)
#include <errno.h>
#include <fcntl.h>
#include <linux/loop.h>
#include <sched.h>
#include <string.h>
#include <sys/ioctl.h>
#include <sys/mount.h>
#include <sys/stat.h>
#include <sys/wait.h>
#include <unistd.h>
#endif

namespace tiny::process {
namespace {

#if defined(__linux__)

bool ReadFirstChildPid(int32_t parent, int32_t* child, std::string* err) {
  const std::string path = "/proc/" + std::to_string(parent) + "/task/" + std::to_string(parent) + "/children";
  std::ifstream in(path);
  if (!in) {
    if (err) *err = "cannot read children of pid " + std::to_string(parent);
    return false;
  }
  int32_t pid = 0;
  if (!(in >> pid) || pid <= 0) {
    if (err) *err = "no mount-ns child for pid " + std::to_string(parent);
    return false;
  }
  *child = pid;
  return true;
}

bool EnsureExt4(const std::string& disk_path, std::string* err) {
  std::ifstream in(disk_path, std::ios::binary);
  if (!in) {
    if (err) *err = "cannot open disk image";
    return false;
  }
  in.seekg(1024 + 56);
  uint16_t magic = 0;
  in.read(reinterpret_cast<char*>(&magic), sizeof(magic));
  if (in && magic == 0xEF53) {
    return true;
  }
  const pid_t pid = fork();
  if (pid < 0) {
    if (err) *err = "fork mkfs failed";
    return false;
  }
  if (pid == 0) {
    execlp("mkfs.ext4", "mkfs.ext4", "-F", "-q", disk_path.c_str(), static_cast<char*>(nullptr));
    _exit(127);
  }
  int status = 0;
  if (waitpid(pid, &status, 0) < 0 || !WIFEXITED(status) || WEXITSTATUS(status) != 0) {
    if (err) *err = "mkfs.ext4 failed for " + disk_path;
    return false;
  }
  return true;
}

bool AttachLoop(const std::string& disk_path, std::string* device_path, std::string* err) {
  const int ctl = open("/dev/loop-control", O_RDWR | O_CLOEXEC);
  if (ctl < 0) {
    if (err) *err = std::string("open loop-control: ") + strerror(errno);
    return false;
  }
  const int devnum = ioctl(ctl, LOOP_CTL_GET_FREE);
  close(ctl);
  if (devnum < 0) {
    if (err) *err = std::string("LOOP_CTL_GET_FREE: ") + strerror(errno);
    return false;
  }
  const std::string dev = "/dev/loop" + std::to_string(devnum);
  const int loopfd = open(dev.c_str(), O_RDWR | O_CLOEXEC);
  if (loopfd < 0) {
    if (err) *err = std::string("open ") + dev + ": " + strerror(errno);
    return false;
  }
  const int filefd = open(disk_path.c_str(), O_RDWR | O_CLOEXEC);
  if (filefd < 0) {
    close(loopfd);
    if (err) *err = std::string("open disk: ") + strerror(errno);
    return false;
  }
  if (ioctl(loopfd, LOOP_SET_FD, filefd) != 0) {
    close(filefd);
    close(loopfd);
    if (err) *err = std::string("LOOP_SET_FD: ") + strerror(errno);
    return false;
  }
  close(filefd);
  close(loopfd);
  *device_path = dev;
  return true;
}

bool DetachLoop(const std::string& device_path, std::string* err) {
  if (device_path.empty()) return true;
  const int loopfd = open(device_path.c_str(), O_RDWR | O_CLOEXEC);
  if (loopfd < 0) {
    if (err) *err = std::string("open loop for detach: ") + strerror(errno);
    return false;
  }
  if (ioctl(loopfd, LOOP_CLR_FD, 0) != 0) {
    close(loopfd);
    if (err) *err = std::string("LOOP_CLR_FD: ") + strerror(errno);
    return false;
  }
  close(loopfd);
  return true;
}

bool RunInMountNs(int32_t mnt_pid, const std::function<int()>& fn, std::string* err) {
  const std::string nspath = "/proc/" + std::to_string(mnt_pid) + "/ns/mnt";
  const int nsfd = open(nspath.c_str(), O_RDONLY | O_CLOEXEC);
  if (nsfd < 0) {
    if (err) *err = std::string("open mnt ns: ") + strerror(errno);
    return false;
  }
  const pid_t helper = fork();
  if (helper < 0) {
    close(nsfd);
    if (err) *err = "fork mount helper failed";
    return false;
  }
  if (helper == 0) {
    if (setns(nsfd, CLONE_NEWNS) != 0) _exit(10);
    close(nsfd);
    _exit(fn());
  }
  close(nsfd);
  int status = 0;
  if (waitpid(helper, &status, 0) < 0) {
    if (err) *err = "wait mount helper failed";
    return false;
  }
  if (!WIFEXITED(status) || WEXITSTATUS(status) != 0) {
    if (err) *err = "mount helper failed with code " + std::to_string(WEXITSTATUS(status));
    return false;
  }
  return true;
}

#endif

}  // namespace

AttachResult AttachVolumeToInstance(int32_t supervisor_pid, const std::string& disk_path,
                                    const std::string& rootfs_path, const std::string& mount_name) {
  AttachResult out;
#if !defined(__linux__)
  (void)supervisor_pid;
  (void)disk_path;
  (void)rootfs_path;
  (void)mount_name;
  out.error_message = "volume attach requires Linux";
  return out;
#else
  if (supervisor_pid <= 0 || disk_path.empty() || rootfs_path.empty() || mount_name.empty()) {
    out.error_message = "missing attach fields";
    return out;
  }
  if (mount_name.find('/') != std::string::npos || mount_name == "." || mount_name == "..") {
    out.error_message = "invalid mount_name";
    return out;
  }
  std::string err;
  if (!EnsureExt4(disk_path, &err)) {
    out.error_message = err;
    return out;
  }
  std::string device;
  if (!AttachLoop(disk_path, &device, &err)) {
    out.error_message = err;
    return out;
  }
  int32_t mnt_pid = 0;
  if (!ReadFirstChildPid(supervisor_pid, &mnt_pid, &err)) {
    DetachLoop(device, nullptr);
    out.error_message = err;
    return out;
  }
  const std::string mount_path = rootfs_path + "/mnt/" + mount_name;
  const bool mounted = RunInMountNs(
      mnt_pid,
      [&]() -> int {
        if (mkdir((rootfs_path + "/mnt").c_str(), 0750) != 0 && errno != EEXIST) return 20;
        if (mkdir(mount_path.c_str(), 0750) != 0 && errno != EEXIST) return 21;
        if (mount(device.c_str(), mount_path.c_str(), "ext4", 0, nullptr) != 0) return 22;
        return 0;
      },
      &err);
  if (!mounted) {
    DetachLoop(device, nullptr);
    out.error_message = err.empty() ? "mount in instance ns failed" : err;
    return out;
  }
  out.ok = true;
  out.device_path = device;
  out.mount_path = mount_path;
  return out;
#endif
}

bool DetachVolumeFromInstance(int32_t supervisor_pid, const std::string& device_path,
                              const std::string& rootfs_path, const std::string& mount_name,
                              std::string* err) {
#if !defined(__linux__)
  (void)supervisor_pid;
  (void)device_path;
  (void)rootfs_path;
  (void)mount_name;
  if (err) *err = "volume detach requires Linux";
  return false;
#else
  if (mount_name.find('/') != std::string::npos || mount_name.empty()) {
    if (err) *err = "invalid mount_name";
    return false;
  }
  const std::string mount_path = rootfs_path + "/mnt/" + mount_name;
  if (supervisor_pid > 0 && !rootfs_path.empty()) {
    int32_t mnt_pid = 0;
    std::string local;
    if (ReadFirstChildPid(supervisor_pid, &mnt_pid, &local)) {
      RunInMountNs(
          mnt_pid,
          [&]() -> int {
            if (umount2(mount_path.c_str(), MNT_DETACH) != 0 && errno != EINVAL && errno != ENOENT) {
              return 30;
            }
            return 0;
          },
          err);
    }
  }
  return DetachLoop(device_path, err);
#endif
}

}  // namespace tiny::process
