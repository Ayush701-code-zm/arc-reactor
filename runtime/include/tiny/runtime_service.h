#pragma once

#include "tiny/process/supervisor.h"

#include <string>

namespace tiny {

// JSON field names match proto/tiny/v1/runtime.proto.
// Interim HTTP transport until gRPC codegen is wired.
class RuntimeService {
 public:
  explicit RuntimeService(process::Supervisor* supervisor) : supervisor_(supervisor) {}

  std::string HandleCreate(const std::string& body);
  std::string HandleList();
  std::string HandleInspect(const std::string& id);
  std::string HandleStart(const std::string& body);
  std::string HandleStop(const std::string& body);
  std::string HandleDelete(const std::string& body);

 private:
  process::Supervisor* supervisor_;
};

bool ServeHttp(const std::string& addr, RuntimeService* service);

}  // namespace tiny
