#include "tiny/grpc_server.h"
#include "tiny/process/supervisor.h"

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
  if (!tiny::ServeGrpc(addr, &supervisor)) {
    std::cerr << "failed to bind gRPC " << addr << "\n";
    return 1;
  }
  return 0;
}
