#pragma once

#include <string>

namespace tiny::process {

bool EnterPidNamespace(std::string* err);
bool EnterMountNamespace(std::string* err);
bool EnterNetNamespace(std::string* err);
bool BringUpLoopback(std::string* err);
bool ChrootRootfs(const std::string& rootfs_path, std::string* err);

}  // namespace tiny::process
