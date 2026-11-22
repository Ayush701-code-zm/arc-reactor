#pragma once

#include <cstdint>
#include <string>

namespace tiny::process {

struct NetworkAttachResult {
  bool ok = false;
  std::string error_message;
};

NetworkAttachResult AttachNetworkToInstance(int32_t supervisor_pid, const std::string& bridge_name,
                                            const std::string& veth_host, const std::string& veth_guest,
                                            const std::string& guest_ip_cidr, const std::string& gateway_ip);

bool DetachNetworkFromInstance(int32_t supervisor_pid, const std::string& bridge_name,
                               const std::string& veth_host, const std::string& veth_guest,
                               bool delete_bridge_if_empty, std::string* err);

}  // namespace tiny::process
