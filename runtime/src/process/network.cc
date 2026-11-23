#include "tiny/process/network.h"

#include <fstream>
#include <string>
#include <vector>

#if defined(__linux__)
#include <dirent.h>
#include <errno.h>
#include <fcntl.h>
#include <sched.h>
#include <stdlib.h>
#include <string.h>
#include <sys/stat.h>
#include <sys/types.h>
#include <sys/wait.h>
#include <unistd.h>
#endif

namespace tiny::process {
namespace {

#if defined(__linux__)

bool ReadFirstChildPid(int32_t parent, int32_t* child, std::string* err) {
  const std::string path = "/proc/" + std::to_string(parent) + "/task/" + std::to_string(parent) + "/children";
  std::ifstream in(path);
  int32_t pid = 0;
  if (in && (in >> pid) && pid > 0) {
    *child = pid;
    return true;
  }
  DIR* dir = opendir("/proc");
  if (!dir) {
    if (err) *err = "cannot scan /proc for children of pid " + std::to_string(parent);
    return false;
  }
  while (dirent* ent = readdir(dir)) {
    if (ent->d_name[0] < '1' || ent->d_name[0] > '9') continue;
    const int32_t cand = static_cast<int32_t>(atoi(ent->d_name));
    if (cand <= 0) continue;
    std::ifstream st("/proc/" + std::to_string(cand) + "/stat");
    if (!st) continue;
    int32_t rpid = 0;
    std::string comm;
    char state = 0;
    int32_t ppid = 0;
    st >> rpid >> comm >> state >> ppid;
    if (ppid == parent && rpid > 0) {
      closedir(dir);
      *child = rpid;
      return true;
    }
  }
  closedir(dir);
  if (err) *err = "no netns child for pid " + std::to_string(parent);
  return false;
}

bool RunCmd(const char* bin, const std::vector<std::string>& args, std::string* err) {
  std::vector<char*> argv;
  argv.push_back(const_cast<char*>(bin));
  for (const auto& a : args) argv.push_back(const_cast<char*>(a.c_str()));
  argv.push_back(nullptr);
  const pid_t pid = fork();
  if (pid < 0) {
    if (err) *err = std::string("fork ") + bin + " failed";
    return false;
  }
  if (pid == 0) {
    execvp(bin, argv.data());
    _exit(127);
  }
  int status = 0;
  if (waitpid(pid, &status, 0) < 0 || !WIFEXITED(status) || WEXITSTATUS(status) != 0) {
    if (err) {
      *err = bin;
      for (const auto& a : args) *err += " " + a;
      *err += " failed";
      if (WIFEXITED(status) && WEXITSTATUS(status) == 127) {
        *err += " (command not found; install iproute2)";
      }
    }
    return false;
  }
  return true;
}

bool RunIP(const std::vector<std::string>& args, std::string* err) { return RunCmd("ip", args, err); }

bool RunInNetns(int32_t net_pid, const std::vector<std::string>& ip_args, std::string* err) {
  std::vector<std::string> args = {"-t", std::to_string(net_pid), "-n", "--", "ip"};
  args.insert(args.end(), ip_args.begin(), ip_args.end());
  return RunCmd("nsenter", args, err);
}

bool EnsureBridge(const std::string& bridge, const std::string& gateway_cidr, std::string* err) {
  if (!RunIP({"link", "show", "dev", bridge}, nullptr)) {
    if (!RunIP({"link", "add", "name", bridge, "type", "bridge"}, err)) return false;
  }
  if (!RunIP({"addr", "replace", gateway_cidr, "dev", bridge}, err)) return false;
  return RunIP({"link", "set", "dev", bridge, "up"}, err);
}

#endif

}  // namespace

NetworkAttachResult AttachNetworkToInstance(int32_t supervisor_pid, const std::string& bridge_name,
                                            const std::string& veth_host, const std::string& veth_guest,
                                            const std::string& guest_ip_cidr, const std::string& gateway_ip) {
  NetworkAttachResult out;
#if !defined(__linux__)
  (void)supervisor_pid;
  (void)bridge_name;
  (void)veth_host;
  (void)veth_guest;
  (void)guest_ip_cidr;
  (void)gateway_ip;
  out.error_message = "network attach requires Linux";
  return out;
#else
  if (supervisor_pid <= 0 || bridge_name.empty() || veth_host.empty() || guest_ip_cidr.empty() ||
      gateway_ip.empty()) {
    out.error_message = "missing attach network fields";
    return out;
  }
  std::string err;
  std::string gw = gateway_ip;
  if (gw.find('/') == std::string::npos) {
    const auto slash = guest_ip_cidr.find('/');
    if (slash != std::string::npos) gw += guest_ip_cidr.substr(slash);
    else gw += "/24";
  }
  if (!EnsureBridge(bridge_name, gw, &err)) {
    out.error_message = err;
    return out;
  }
  RunIP({"link", "delete", veth_host}, nullptr);
  const std::string peer = veth_host + "p";
  if (!RunIP({"link", "add", veth_host, "type", "veth", "peer", "name", peer}, &err)) {
    out.error_message = err;
    return out;
  }
  int32_t net_pid = 0;
  if (!ReadFirstChildPid(supervisor_pid, &net_pid, &err)) {
    RunIP({"link", "delete", veth_host}, nullptr);
    out.error_message = err;
    return out;
  }
  if (!RunIP({"link", "set", "dev", peer, "netns", std::to_string(net_pid)}, &err)) {
    RunIP({"link", "delete", veth_host}, nullptr);
    out.error_message = err;
    return out;
  }
  if (!RunIP({"link", "set", "dev", veth_host, "master", bridge_name}, &err)) {
    RunIP({"link", "delete", veth_host}, nullptr);
    out.error_message = err;
    return out;
  }
  if (!RunIP({"link", "set", "dev", veth_host, "up"}, &err)) {
    RunIP({"link", "delete", veth_host}, nullptr);
    out.error_message = err;
    return out;
  }

  const std::string guest = veth_guest.empty() ? "eth0" : veth_guest;
  if (peer != guest) {
    if (!RunInNetns(net_pid, {"link", "set", "dev", peer, "name", guest}, &err)) {
      RunIP({"link", "delete", veth_host}, nullptr);
      out.error_message = err;
      return out;
    }
  }
  if (!RunInNetns(net_pid, {"addr", "replace", guest_ip_cidr, "dev", guest}, &err)) {
    RunIP({"link", "delete", veth_host}, nullptr);
    out.error_message = err;
    return out;
  }
  if (!RunInNetns(net_pid, {"link", "set", "dev", guest, "up"}, &err)) {
    RunIP({"link", "delete", veth_host}, nullptr);
    out.error_message = err;
    return out;
  }
  RunInNetns(net_pid, {"route", "replace", "default", "via", gateway_ip, "dev", guest}, &err);
  out.ok = true;
  return out;
#endif
}

bool DetachNetworkFromInstance(int32_t supervisor_pid, const std::string& bridge_name,
                               const std::string& veth_host, const std::string& veth_guest,
                               bool delete_bridge_if_empty, std::string* err) {
#if !defined(__linux__)
  (void)supervisor_pid;
  (void)bridge_name;
  (void)veth_host;
  (void)veth_guest;
  (void)delete_bridge_if_empty;
  if (err) *err = "network detach requires Linux";
  return false;
#else
  (void)supervisor_pid;
  (void)veth_guest;
  if (!veth_host.empty()) {
    RunIP({"link", "delete", veth_host}, nullptr);
  }
  if (delete_bridge_if_empty && !bridge_name.empty()) {
    RunIP({"link", "delete", bridge_name}, nullptr);
  }
  return true;
#endif
}

}  // namespace tiny::process
