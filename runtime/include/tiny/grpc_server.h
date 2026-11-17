#pragma once

#include "tiny/process/supervisor.h"

#include <string>

namespace tiny {

bool ServeGrpc(const std::string& addr, process::Supervisor* supervisor);

}  // namespace tiny
