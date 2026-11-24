#include <grpcpp/grpcpp.h>

#include "tiny/v1/node.grpc.pb.h"
#include "tiny/v1/runtime.grpc.pb.h"

#include <chrono>
#include <cstdlib>
#include <cerrno>
#include <fstream>
#include <iostream>
#include <memory>
#include <mutex>
#include <string>
#include <sys/stat.h>
#include <thread>
#include <unistd.h>

#if defined(__linux__)
#include <fstream>
#endif

namespace {

std::string EnvOr(const char* key, const std::string& def) {
  const char* v = std::getenv(key);
  if (v == nullptr || *v == '\0') {
    return def;
  }
  return v;
}

std::string Hostname() {
  char buf[256];
  if (gethostname(buf, sizeof(buf)) != 0) {
    return "unknown";
  }
  buf[sizeof(buf) - 1] = '\0';
  return buf;
}

uint32_t HostCPU() {
  long n = sysconf(_SC_NPROCESSORS_ONLN);
  if (n < 1) {
    return 1;
  }
  return static_cast<uint32_t>(n);
}

uint64_t HostMemoryBytes() {
#if defined(__linux__)
  std::ifstream in("/proc/meminfo");
  std::string key;
  uint64_t kb = 0;
  std::string unit;
  while (in >> key >> kb >> unit) {
    if (key == "MemTotal:") {
      return kb * 1024ULL;
    }
  }
#endif
  return 1024ULL * 1024ULL * 1024ULL;
}

bool EnsureParentDir(const std::string& path) {
  const auto pos = path.find_last_of('/');
  if (pos == std::string::npos || pos == 0) {
    return true;
  }
  const std::string dir = path.substr(0, pos);
#if defined(_WIN32)
  (void)dir;
  return true;
#else
  return mkdir(dir.c_str(), 0700) == 0 || errno == EEXIST;
#endif
}

std::string LoadOrCreateNodeID(const std::string& path) {
  (void)EnsureParentDir(path);
  {
    std::ifstream in(path);
    std::string id;
    if (in && (in >> id) && !id.empty()) {
      return id;
    }
  }
  const char* env = std::getenv("TINY_NODE_ID");
  if (env != nullptr && *env != '\0') {
    std::ofstream out(path);
    out << env;
    return env;
  }
  std::string id = "node-";
  id += std::to_string(static_cast<unsigned long long>(
      std::chrono::steady_clock::now().time_since_epoch().count()));
  std::ofstream out(path);
  out << id;
  return id;
}

tiny::v1::InstanceStatus MapStatus(const tiny::v1::InstanceStatus s) { return s; }

class Agent {
 public:
  Agent(std::string control_addr, std::string runtime_addr, std::string node_id)
      : control_addr_(std::move(control_addr)),
        runtime_addr_(std::move(runtime_addr)),
        node_id_(std::move(node_id)) {}

  int Run() {
    while (true) {
      auto control_chan =
          grpc::CreateChannel(control_addr_, grpc::InsecureChannelCredentials());
      const auto deadline =
          std::chrono::system_clock::now() + std::chrono::seconds(5);
      if (!control_chan->WaitForConnected(deadline)) {
        std::cerr << "tinynodeagent: waiting for control plane at "
                  << control_addr_ << "\n";
        std::this_thread::sleep_for(std::chrono::seconds(1));
        continue;
      }
      auto stub = tiny::v1::NodeControlService::NewStub(control_chan);

      grpc::ClientContext ctx;
      auto stream = stub->Session(&ctx);
      if (!stream) {
        std::cerr << "tinynodeagent: session open failed\n";
        std::this_thread::sleep_for(std::chrono::seconds(2));
        continue;
      }

      tiny::v1::AgentToControl reg_msg;
      auto* reg = reg_msg.mutable_register_();
      reg->set_node_id(node_id_);
      reg->set_hostname(Hostname());
      reg->set_allocatable_cpu(HostCPU());
      reg->set_allocatable_memory_bytes(HostMemoryBytes());
      if (!stream->Write(reg_msg)) {
        std::cerr << "tinynodeagent: register write failed: "
                  << ctx.debug_error_string() << "\n";
        std::this_thread::sleep_for(std::chrono::seconds(2));
        continue;
      }

      tiny::v1::ControlToAgent ack;
      if (!stream->Read(&ack) || !ack.has_register_ack()) {
        std::cerr << "tinynodeagent: register ack missing\n";
        std::this_thread::sleep_for(std::chrono::seconds(2));
        continue;
      }
      if (!ack.register_ack().error_message().empty()) {
        std::cerr << "tinynodeagent: register error: "
                  << ack.register_ack().error_message() << "\n";
        std::this_thread::sleep_for(std::chrono::seconds(2));
        continue;
      }
      if (!ack.register_ack().node_id().empty()) {
        node_id_ = ack.register_ack().node_id();
      }
      std::cerr << "tinynodeagent: registered as " << node_id_ << "\n";

      auto runtime_chan =
          grpc::CreateChannel(runtime_addr_, grpc::InsecureChannelCredentials());
      auto runtime = tiny::v1::RuntimeService::NewStub(runtime_chan);

      bool alive = true;
      std::mutex write_mu;
      std::thread heartbeat([&]() {
        while (alive) {
          tiny::v1::AgentToControl hb_msg;
          auto* hb = hb_msg.mutable_heartbeat();
          hb->set_node_id(node_id_);
          hb->set_allocatable_cpu(HostCPU());
          hb->set_allocatable_memory_bytes(HostMemoryBytes());
          hb->set_running_instances(CountRunning(runtime.get()));
          {
            std::lock_guard<std::mutex> lock(write_mu);
            if (!stream->Write(hb_msg)) {
              alive = false;
              break;
            }
          }
          for (int i = 0; i < 50 && alive; ++i) {
            std::this_thread::sleep_for(std::chrono::milliseconds(100));
          }
        }
      });

      tiny::v1::ControlToAgent cmd;
      while (alive && stream->Read(&cmd)) {
        if (!cmd.has_op()) {
          continue;
        }
        HandleOp(runtime.get(), stream.get(), write_mu, cmd.op());
      }
      alive = false;
      heartbeat.join();
      std::cerr << "tinynodeagent: session ended; reconnecting\n";
      std::this_thread::sleep_for(std::chrono::seconds(1));
    }
  }

 private:
  static uint32_t CountRunning(tiny::v1::RuntimeService::Stub* runtime) {
    grpc::ClientContext ctx;
    tiny::v1::ListInstancesRequest req;
    tiny::v1::ListInstancesResponse resp;
    auto st = runtime->ListInstances(&ctx, req, &resp);
    if (!st.ok()) {
      return 0;
    }
    uint32_t n = 0;
    for (const auto& inst : resp.instances()) {
      if (inst.status() == tiny::v1::INSTANCE_STATUS_RUNNING) {
        ++n;
      }
    }
    return n;
  }

  void HandleOp(tiny::v1::RuntimeService::Stub* runtime,
                grpc::ClientReaderWriterInterface<tiny::v1::AgentToControl,
                                                  tiny::v1::ControlToAgent>* stream,
                std::mutex& write_mu,
                const tiny::v1::RuntimeOpRequest& op) {
    tiny::v1::AgentToControl out;
    auto* res = out.mutable_op_result();
    res->set_request_id(op.request_id());

    grpc::ClientContext ctx;
    if (op.has_create()) {
      tiny::v1::CreateInstanceResponse resp;
      auto st = runtime->CreateInstance(&ctx, op.create(), &resp);
      if (!st.ok()) {
        res->set_error_message(st.error_message());
      } else {
        res->set_id(resp.id());
        res->set_status(MapStatus(resp.status()));
        res->set_pid(resp.pid());
        res->set_error_message(resp.error_message());
      }
    } else if (op.has_start()) {
      tiny::v1::StartInstanceResponse resp;
      auto st = runtime->StartInstance(&ctx, op.start(), &resp);
      if (!st.ok()) {
        res->set_error_message(st.error_message());
      } else {
        res->set_id(resp.id());
        res->set_status(MapStatus(resp.status()));
        res->set_pid(resp.pid());
        res->set_error_message(resp.error_message());
      }
    } else if (op.has_stop()) {
      tiny::v1::StopInstanceResponse resp;
      auto st = runtime->StopInstance(&ctx, op.stop(), &resp);
      if (!st.ok()) {
        res->set_error_message(st.error_message());
      } else {
        res->set_id(resp.id());
        res->set_status(MapStatus(resp.status()));
        res->set_error_message(resp.error_message());
      }
    } else if (op.has_delete_()) {
      tiny::v1::DeleteInstanceResponse resp;
      auto st = runtime->DeleteInstance(&ctx, op.delete_(), &resp);
      if (!st.ok()) {
        res->set_error_message(st.error_message());
      } else {
        res->set_id(resp.id());
        res->set_error_message(resp.error_message());
      }
    } else if (op.has_inspect()) {
      tiny::v1::InspectInstanceResponse resp;
      auto st = runtime->InspectInstance(&ctx, op.inspect(), &resp);
      if (!st.ok()) {
        res->set_error_message(st.error_message());
      } else if (resp.has_instance()) {
        res->set_id(resp.instance().id());
        res->set_status(MapStatus(resp.instance().status()));
        res->set_pid(resp.instance().pid());
        res->set_error_message(resp.instance().error_message());
      }
    } else {
      res->set_error_message("unknown op");
    }
    std::lock_guard<std::mutex> lock(write_mu);
    stream->Write(out);
  }

  std::string control_addr_;
  std::string runtime_addr_;
  std::string node_id_;
};

}  // namespace

int main(int argc, char** argv) {
  std::string control = EnvOr("TINY_NODE_CONTROL_ADDR", "127.0.0.1:50052");
  std::string runtime = EnvOr("TINY_RUNTIME_ADDR", "127.0.0.1:50051");
  std::string id_default = "./tiny-aws-node-id";
  if (const char* home = std::getenv("HOME"); home != nullptr && home[0] != '\0') {
    id_default = std::string(home) + "/.tiny-aws/node-id";
  }
  std::string id_file = EnvOr("TINY_NODE_ID_FILE", id_default);

  for (int i = 1; i < argc; ++i) {
    const std::string a = argv[i];
    if (a == "--control" && i + 1 < argc) {
      control = argv[++i];
    } else if (a == "--runtime" && i + 1 < argc) {
      runtime = argv[++i];
    } else if (a == "--node-id-file" && i + 1 < argc) {
      id_file = argv[++i];
    }
  }

  const std::string node_id = LoadOrCreateNodeID(id_file);
  Agent agent(control, runtime, node_id);
  return agent.Run();
}
