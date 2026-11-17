#include "tiny/process/supervisor.h"
#include "tiny/runtime_service.h"

#include <cstdlib>
#include <iostream>
#include <string>

int main(int argc, char** argv) {
  std::string addr = "127.0.0.1:50051";
  for (int i = 1; i < argc; ++i) {
    const std::string a = argv[i];
    if (a == "--addr" && i + 1 < argc) {
      addr = argv[++i];
    }
  }

  tiny::process::Supervisor supervisor;
  tiny::RuntimeService service(&supervisor);
  if (!tiny::ServeHttp(addr, &service)) {
    std::cerr << "failed to bind " << addr << "\n";
    return 1;
  }
  return 0;
}
