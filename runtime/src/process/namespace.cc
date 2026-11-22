#include "tiny/process/namespace.h"

#if defined(__linux__)
#include <errno.h>
#include <fcntl.h>
#include <net/if.h>
#include <sched.h>
#include <string.h>
#include <sys/ioctl.h>
#include <sys/socket.h>
#include <sys/stat.h>
#include <unistd.h>
#endif

namespace tiny::process {

bool EnterPidNamespace(std::string* err) {
#if !defined(__linux__)
  if (err) *err = "pid namespace requires Linux";
  return false;
#else
  if (unshare(CLONE_NEWPID) != 0) {
    if (err) *err = std::string("unshare(CLONE_NEWPID) failed: ") + strerror(errno);
    return false;
  }
  return true;
#endif
}

bool EnterMountNamespace(std::string* err) {
#if !defined(__linux__)
  if (err) *err = "mount namespace requires Linux";
  return false;
#else
  if (unshare(CLONE_NEWNS) != 0) {
    if (err) *err = std::string("unshare(CLONE_NEWNS) failed: ") + strerror(errno);
    return false;
  }
  return true;
#endif
}

bool EnterNetNamespace(std::string* err) {
#if !defined(__linux__)
  if (err) *err = "net namespace requires Linux";
  return false;
#else
  if (unshare(CLONE_NEWNET) != 0) {
    if (err) *err = std::string("unshare(CLONE_NEWNET) failed: ") + strerror(errno);
    return false;
  }
  return true;
#endif
}

bool BringUpLoopback(std::string* err) {
#if !defined(__linux__)
  if (err) *err = "loopback requires Linux";
  return false;
#else
  const int fd = socket(AF_INET, SOCK_DGRAM, 0);
  if (fd < 0) {
    if (err) *err = std::string("socket for lo: ") + strerror(errno);
    return false;
  }
  struct ifreq ifr {};
  strncpy(ifr.ifr_name, "lo", IFNAMSIZ - 1);
  if (ioctl(fd, SIOCGIFFLAGS, &ifr) != 0) {
    close(fd);
    if (err) *err = std::string("SIOCGIFFLAGS lo: ") + strerror(errno);
    return false;
  }
  ifr.ifr_flags |= IFF_UP | IFF_RUNNING;
  if (ioctl(fd, SIOCSIFFLAGS, &ifr) != 0) {
    close(fd);
    if (err) *err = std::string("SIOCSIFFLAGS lo: ") + strerror(errno);
    return false;
  }
  close(fd);
  return true;
#endif
}

bool ChrootRootfs(const std::string& rootfs_path, std::string* err) {
#if !defined(__linux__)
  (void)rootfs_path;
  if (err) *err = "chroot requires Linux";
  return false;
#else
  if (rootfs_path.empty()) {
    if (err) *err = "rootfs_path is empty";
    return false;
  }
  struct stat st {};
  if (stat(rootfs_path.c_str(), &st) != 0 || !S_ISDIR(st.st_mode)) {
    if (err) *err = "rootfs_path is not a directory: " + rootfs_path;
    return false;
  }
  if (chdir(rootfs_path.c_str()) != 0) {
    if (err) *err = std::string("chdir rootfs failed: ") + strerror(errno);
    return false;
  }
  if (chroot(".") != 0) {
    if (err) *err = std::string("chroot failed: ") + strerror(errno);
    return false;
  }
  if (chdir("/") != 0) {
    if (err) *err = std::string("chdir / after chroot failed: ") + strerror(errno);
    return false;
  }
  return true;
#endif
}

}  // namespace tiny::process
