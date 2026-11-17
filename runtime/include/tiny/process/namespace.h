#pragma once

#include <string>

namespace tiny::process {

bool EnterPidNamespace(std::string* err);

}  // namespace tiny::process
