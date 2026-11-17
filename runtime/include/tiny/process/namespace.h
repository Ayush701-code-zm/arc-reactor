#pragma once

#include <string>

namespace tiny::process {

// Enter a new PID namespace in the child (Linux). Returns false with err on failure.
bool EnterPidNamespace(std::string* err);

}  // namespace tiny::process
