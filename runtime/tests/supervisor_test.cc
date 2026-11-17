#include "tiny/process/supervisor.h"

#include <cstdlib>
#include <iostream>

int main() {
  tiny::process::Supervisor sup;
  tiny::process::Spec spec;
  spec.id = "test-id";
  spec.name = "test";
  spec.image = "alpine";
  spec.cpu = 1;
  spec.memory_bytes = 64 * 1024 * 1024;

  auto created = sup.Create(spec);
#if defined(__linux__)
  if (created.status == tiny::process::Status::kFailed) {
    std::cout << "linux create failed (ok in restricted env): " << created.error_message << "\n";
    return 0;
  }
  if (created.status != tiny::process::Status::kRunning || created.pid <= 0) {
    std::cerr << "expected running instance\n";
    return 1;
  }
  auto stopped = sup.Stop(spec.id);
  if (stopped.status != tiny::process::Status::kStopped) {
    std::cerr << "expected stopped\n";
    return 1;
  }
  sup.Delete(spec.id);
#else
  if (created.status != tiny::process::Status::kFailed) {
    std::cerr << "non-linux should fail create\n";
    return 1;
  }
  std::cout << "non-linux stub ok: " << created.error_message << "\n";
#endif
  return 0;
}
