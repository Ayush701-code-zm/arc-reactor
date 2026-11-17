#include "tiny/grpc_server.h"

#include "tiny/v1/runtime.grpc.pb.h"
#include "tiny/v1/runtime.pb.h"

#include <grpcpp/grpcpp.h>

#include <iostream>
#include <memory>
#include <string>

namespace tiny {
namespace {

tiny::v1::InstanceStatus ToProto(process::Status s) {
  switch (s) {
    case process::Status::kPending:
      return tiny::v1::INSTANCE_STATUS_PENDING;
    case process::Status::kRunning:
      return tiny::v1::INSTANCE_STATUS_RUNNING;
    case process::Status::kStopping:
      return tiny::v1::INSTANCE_STATUS_STOPPING;
    case process::Status::kStopped:
      return tiny::v1::INSTANCE_STATUS_STOPPED;
    case process::Status::kFailed:
      return tiny::v1::INSTANCE_STATUS_FAILED;
  }
  return tiny::v1::INSTANCE_STATUS_UNSPECIFIED;
}

void FillInstance(const process::Instance& in, tiny::v1::Instance* out) {
  out->set_id(in.spec.id);
  out->set_name(in.spec.name);
  out->set_image(in.spec.image);
  out->set_cpu(in.spec.cpu);
  out->set_memory_bytes(in.spec.memory_bytes);
  out->set_status(ToProto(in.status));
  out->set_pid(in.pid);
  out->set_error_message(in.error_message);
}

class RuntimeServiceImpl final : public tiny::v1::RuntimeService::Service {
 public:
  explicit RuntimeServiceImpl(process::Supervisor* supervisor) : supervisor_(supervisor) {}

  grpc::Status CreateInstance(grpc::ServerContext*, const tiny::v1::CreateInstanceRequest* req,
                              tiny::v1::CreateInstanceResponse* resp) override {
    process::Spec spec;
    spec.id = req->id();
    spec.name = req->name();
    spec.image = req->image();
    spec.cpu = req->cpu();
    spec.memory_bytes = req->memory_bytes();
    if (spec.id.empty() || spec.name.empty() || spec.image.empty() || spec.cpu < 1 ||
        spec.memory_bytes == 0) {
      resp->set_id(spec.id);
      resp->set_status(tiny::v1::INSTANCE_STATUS_FAILED);
      resp->set_pid(0);
      resp->set_error_message("missing required create fields");
      return grpc::Status::OK;
    }
    const auto created = supervisor_->Create(spec);
    resp->set_id(created.spec.id);
    resp->set_status(ToProto(created.status));
    resp->set_pid(created.pid);
    resp->set_error_message(created.error_message);
    return grpc::Status::OK;
  }

  grpc::Status ListInstances(grpc::ServerContext*, const tiny::v1::ListInstancesRequest*,
                             tiny::v1::ListInstancesResponse* resp) override {
    for (const auto& inst : supervisor_->List()) {
      FillInstance(inst, resp->add_instances());
    }
    return grpc::Status::OK;
  }

  grpc::Status InspectInstance(grpc::ServerContext*, const tiny::v1::InspectInstanceRequest* req,
                               tiny::v1::InspectInstanceResponse* resp) override {
    FillInstance(supervisor_->Inspect(req->id()), resp->mutable_instance());
    return grpc::Status::OK;
  }

  grpc::Status StartInstance(grpc::ServerContext*, const tiny::v1::StartInstanceRequest* req,
                             tiny::v1::StartInstanceResponse* resp) override {
    const auto started = supervisor_->Start(req->id());
    resp->set_id(started.spec.id);
    resp->set_status(ToProto(started.status));
    resp->set_pid(started.pid);
    resp->set_error_message(started.error_message);
    return grpc::Status::OK;
  }

  grpc::Status StopInstance(grpc::ServerContext*, const tiny::v1::StopInstanceRequest* req,
                            tiny::v1::StopInstanceResponse* resp) override {
    const auto stopped = supervisor_->Stop(req->id());
    resp->set_id(stopped.spec.id);
    resp->set_status(ToProto(stopped.status));
    resp->set_error_message(stopped.error_message);
    return grpc::Status::OK;
  }

  grpc::Status DeleteInstance(grpc::ServerContext*, const tiny::v1::DeleteInstanceRequest* req,
                              tiny::v1::DeleteInstanceResponse* resp) override {
    supervisor_->Delete(req->id());
    resp->set_id(req->id());
    resp->set_error_message("");
    return grpc::Status::OK;
  }

 private:
  process::Supervisor* supervisor_;
};

}  // namespace

bool ServeGrpc(const std::string& addr, process::Supervisor* supervisor) {
  RuntimeServiceImpl service(supervisor);
  grpc::ServerBuilder builder;
  builder.AddListeningPort(addr, grpc::InsecureServerCredentials());
  builder.RegisterService(&service);
  std::unique_ptr<grpc::Server> server(builder.BuildAndStart());
  if (!server) {
    return false;
  }
  std::cerr << "tinyruntimed listening on grpc://" << addr << "\n";
  server->Wait();
  return true;
}

}  // namespace tiny
