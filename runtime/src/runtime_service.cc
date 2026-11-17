#include "tiny/runtime_service.h"

#include <sstream>
#include <string>

namespace tiny {
namespace {

std::string JsonEscape(const std::string& s) {
  std::string out;
  out.reserve(s.size());
  for (char c : s) {
    switch (c) {
      case '\\':
        out += "\\\\";
        break;
      case '"':
        out += "\\\"";
        break;
      case '\n':
        out += "\\n";
        break;
      default:
        out += c;
    }
  }
  return out;
}

std::string ExtractString(const std::string& body, const std::string& key) {
  const std::string pat = "\"" + key + "\"";
  auto pos = body.find(pat);
  if (pos == std::string::npos) return "";
  pos = body.find(':', pos);
  if (pos == std::string::npos) return "";
  pos = body.find('"', pos);
  if (pos == std::string::npos) return "";
  auto end = body.find('"', pos + 1);
  if (end == std::string::npos) return "";
  return body.substr(pos + 1, end - pos - 1);
}

uint64_t ExtractUint(const std::string& body, const std::string& key) {
  const std::string pat = "\"" + key + "\"";
  auto pos = body.find(pat);
  if (pos == std::string::npos) return 0;
  pos = body.find(':', pos);
  if (pos == std::string::npos) return 0;
  pos++;
  while (pos < body.size() && (body[pos] == ' ' || body[pos] == '\t')) pos++;
  return std::strtoull(body.c_str() + pos, nullptr, 10);
}

std::string InstanceJson(const process::Instance& inst) {
  std::ostringstream o;
  o << "{"
    << "\"id\":\"" << JsonEscape(inst.spec.id) << "\","
    << "\"name\":\"" << JsonEscape(inst.spec.name) << "\","
    << "\"image\":\"" << JsonEscape(inst.spec.image) << "\","
    << "\"cpu\":" << inst.spec.cpu << ","
    << "\"memory_bytes\":" << inst.spec.memory_bytes << ","
    << "\"status\":\"" << process::ToString(inst.status) << "\","
    << "\"pid\":" << inst.pid << ","
    << "\"error_message\":\"" << JsonEscape(inst.error_message) << "\""
    << "}";
  return o.str();
}

std::string CreateRespJson(const process::Instance& inst) {
  std::ostringstream o;
  o << "{"
    << "\"id\":\"" << JsonEscape(inst.spec.id) << "\","
    << "\"status\":\"" << process::ToString(inst.status) << "\","
    << "\"pid\":" << inst.pid << ","
    << "\"error_message\":\"" << JsonEscape(inst.error_message) << "\""
    << "}";
  return o.str();
}

}  // namespace

std::string RuntimeService::HandleCreate(const std::string& body) {
  process::Spec spec;
  spec.id = ExtractString(body, "id");
  spec.name = ExtractString(body, "name");
  spec.image = ExtractString(body, "image");
  spec.cpu = static_cast<uint32_t>(ExtractUint(body, "cpu"));
  spec.memory_bytes = ExtractUint(body, "memory_bytes");
  if (spec.id.empty() || spec.name.empty() || spec.image.empty() || spec.cpu < 1 ||
      spec.memory_bytes == 0) {
    process::Instance failed;
    failed.spec = spec;
    failed.status = process::Status::kFailed;
    failed.error_message = "missing required create fields";
    return CreateRespJson(failed);
  }
  return CreateRespJson(supervisor_->Create(spec));
}

std::string RuntimeService::HandleList() {
  auto list = supervisor_->List();
  std::ostringstream o;
  o << "{\"instances\":[";
  for (size_t i = 0; i < list.size(); ++i) {
    if (i) o << ",";
    o << InstanceJson(list[i]);
  }
  o << "]}";
  return o.str();
}

std::string RuntimeService::HandleInspect(const std::string& id) {
  return InstanceJson(supervisor_->Inspect(id));
}

std::string RuntimeService::HandleStart(const std::string& body) {
  return CreateRespJson(supervisor_->Start(ExtractString(body, "id")));
}

std::string RuntimeService::HandleStop(const std::string& body) {
  return CreateRespJson(supervisor_->Stop(ExtractString(body, "id")));
}

std::string RuntimeService::HandleDelete(const std::string& body) {
  const auto id = ExtractString(body, "id");
  supervisor_->Delete(id);
  return "{\"id\":\"" + JsonEscape(id) + "\",\"error_message\":\"\"}";
}

}  // namespace tiny
