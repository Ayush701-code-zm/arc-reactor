#include "tiny/grpc_server.h"

#include "tiny/process/network.h"
#include "tiny/process/volume.h"
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
    spec.rootfs_path = req->rootfs_path();
    spec.cpu = req->cpu();
    spec.memory_bytes = req->memory_bytes();
    if (spec.id.empty() || spec.name.empty() || spec.image.empty() || spec.rootfs_path.empty() ||
        spec.cpu < 1 || spec.memory_bytes == 0) {
      resp->set_id(spec.id);
      resp->set_status(tiny::v1::INSTANCE_STATUS_FAILED);
      resp->set_pid(0);
      resp->set_error_message("missing required create fields (including rootfs_path)");
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
    std::string err;
    resp->set_id(req->id());
    if (!supervisor_->Delete(req->id(), &err)) {
      resp->set_error_message(err.empty() ? "delete not confirmed" : err);
      return grpc::Status::OK;
    }
    resp->set_error_message("");
    return grpc::Status::OK;
  }

  grpc::Status AttachVolume(grpc::ServerContext*, const tiny::v1::AttachVolumeRequest* req,
                            tiny::v1::AttachVolumeResponse* resp) override {
    resp->set_volume_id(req->volume_id());
    int32_t pid = 0;
    std::string err;
    if (!supervisor_->RunningPid(req->instance_id(), &pid, &err)) {
      resp->set_error_message(err.empty() ? "instance not running" : err);
      return grpc::Status::OK;
    }
    const auto attached = process::AttachVolumeToInstance(pid, req->disk_path(), req->rootfs_path(),
                                                          req->mount_name());
    if (!attached.ok) {
      resp->set_error_message(attached.error_message);
      return grpc::Status::OK;
    }
    resp->set_device_path(attached.device_path);
    resp->set_mount_path(attached.mount_path);
    resp->set_error_message("");
    return grpc::Status::OK;
  }

  grpc::Status DetachVolume(grpc::ServerContext*, const tiny::v1::DetachVolumeRequest* req,
                            tiny::v1::DetachVolumeResponse* resp) override {
    resp->set_volume_id(req->volume_id());
    int32_t pid = 0;
    std::string err;
    (void)supervisor_->RunningPid(req->instance_id(), &pid, &err);
    if (!process::DetachVolumeFromInstance(pid, req->device_path(), req->rootfs_path(), req->mount_name(),
                                           &err)) {
      resp->set_error_message(err.empty() ? "detach failed" : err);
      return grpc::Status::OK;
    }
    resp->set_error_message("");
    return grpc::Status::OK;
  }

  grpc::Status AttachNetwork(grpc::ServerContext*, const tiny::v1::AttachNetworkRequest* req,
                             tiny::v1::AttachNetworkResponse* resp) override {
    resp->set_network_id(req->network_id());
    resp->set_guest_ip_cidr(req->guest_ip_cidr());
    int32_t pid = 0;
    std::string err;
    if (!supervisor_->RunningPid(req->instance_id(), &pid, &err)) {
      resp->set_error_message(err.empty() ? "instance not running" : err);
      return grpc::Status::OK;
    }
    const auto attached = process::AttachNetworkToInstance(pid, req->bridge_name(), req->veth_host(),
                                                           req->veth_guest(), req->guest_ip_cidr(),
                                                           req->gateway_ip());
    if (!attached.ok) {
      resp->set_error_message(attached.error_message);
      return grpc::Status::OK;
    }
    resp->set_error_message("");
    return grpc::Status::OK;
  }

  grpc::Status DetachNetwork(grpc::ServerContext*, const tiny::v1::DetachNetworkRequest* req,
                             tiny::v1::DetachNetworkResponse* resp) override {
    resp->set_network_id(req->network_id());
    int32_t pid = 0;
    std::string err;
    (void)supervisor_->RunningPid(req->instance_id(), &pid, &err);
    if (!process::DetachNetworkFromInstance(pid, req->bridge_name(), req->veth_host(), req->veth_guest(),
                                            req->delete_bridge_if_empty(), &err)) {
      resp->set_error_message(err.empty() ? "detach network failed" : err);
      return grpc::Status::OK;
    }
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
