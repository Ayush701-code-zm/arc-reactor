#include "tiny/runtime_service.h"

#include <arpa/inet.h>
#include <netinet/in.h>
#include <sys/socket.h>
#include <unistd.h>

#include <cstring>
#include <iostream>
#include <sstream>
#include <string>
#include <thread>

namespace tiny {
namespace {

std::string ReadHeaders(int fd) {
  std::string data;
  char buf[1024];
  while (data.find("\r\n\r\n") == std::string::npos) {
    const ssize_t n = ::recv(fd, buf, sizeof(buf), 0);
    if (n <= 0) break;
    data.append(buf, static_cast<size_t>(n));
    if (data.size() > 1 << 20) break;
  }
  return data;
}

size_t ContentLength(const std::string& headers) {
  auto pos = headers.find("Content-Length:");
  if (pos == std::string::npos) pos = headers.find("content-length:");
  if (pos == std::string::npos) return 0;
  return std::strtoul(headers.c_str() + pos + 15, nullptr, 10);
}

std::string QueryParam(const std::string& path, const std::string& key) {
  auto q = path.find('?');
  if (q == std::string::npos) return "";
  auto pos = path.find(key + "=", q);
  if (pos == std::string::npos) return "";
  pos += key.size() + 1;
  auto end = path.find('&', pos);
  if (end == std::string::npos) end = path.size();
  return path.substr(pos, end - pos);
}

void Send(int fd, int code, const std::string& body) {
  std::ostringstream o;
  o << "HTTP/1.1 " << code << " OK\r\n"
    << "Content-Type: application/json\r\n"
    << "Content-Length: " << body.size() << "\r\n"
    << "Connection: close\r\n\r\n"
    << body;
  const auto s = o.str();
  (void)::send(fd, s.data(), s.size(), 0);
}

void HandleClient(int fd, RuntimeService* service) {
  const std::string raw = ReadHeaders(fd);
  if (raw.empty()) {
    ::close(fd);
    return;
  }
  const auto header_end = raw.find("\r\n\r\n");
  std::string headers = raw.substr(0, header_end);
  std::string body = raw.substr(header_end + 4);
  const size_t want = ContentLength(headers);
  while (body.size() < want) {
    char buf[1024];
    const ssize_t n = ::recv(fd, buf, sizeof(buf), 0);
    if (n <= 0) break;
    body.append(buf, static_cast<size_t>(n));
  }

  std::string method;
  std::string path;
  {
    std::istringstream line(headers);
    line >> method >> path;
  }

  std::string resp;
  int code = 200;
  if (method == "POST" && path == "/v1/create") {
    resp = service->HandleCreate(body);
  } else if (method == "GET" && path.rfind("/v1/list", 0) == 0) {
    resp = service->HandleList();
  } else if (method == "GET" && path.rfind("/v1/inspect", 0) == 0) {
    resp = service->HandleInspect(QueryParam(path, "id"));
  } else if (method == "POST" && path == "/v1/start") {
    resp = service->HandleStart(body);
  } else if (method == "POST" && path == "/v1/stop") {
    resp = service->HandleStop(body);
  } else if (method == "POST" && path == "/v1/delete") {
    resp = service->HandleDelete(body);
  } else if (method == "GET" && path == "/healthz") {
    resp = "{\"status\":\"ok\"}";
  } else {
    code = 404;
    resp = "{\"error_message\":\"not found\"}";
  }
  Send(fd, code, resp);
  ::close(fd);
}

}  // namespace

bool ServeHttp(const std::string& addr, RuntimeService* service) {
  std::string host = "127.0.0.1";
  int port = 50051;
  auto colon = addr.rfind(':');
  if (colon != std::string::npos) {
    host = addr.substr(0, colon);
    port = std::stoi(addr.substr(colon + 1));
  }

  const int fd = ::socket(AF_INET, SOCK_STREAM, 0);
  if (fd < 0) return false;
  int yes = 1;
  setsockopt(fd, SOL_SOCKET, SO_REUSEADDR, &yes, sizeof(yes));

  sockaddr_in sa {};
  sa.sin_family = AF_INET;
  sa.sin_port = htons(static_cast<uint16_t>(port));
  if (::inet_pton(AF_INET, host.c_str(), &sa.sin_addr) != 1) {
    ::close(fd);
    return false;
  }
  if (::bind(fd, reinterpret_cast<sockaddr*>(&sa), sizeof(sa)) != 0) {
    ::close(fd);
    return false;
  }
  if (::listen(fd, 16) != 0) {
    ::close(fd);
    return false;
  }

  std::cerr << "tinyruntimed listening on http://" << host << ":" << port
            << " (JSON mirror of proto; replace with gRPC via scripts/gen-proto.sh)\n";

  while (true) {
    const int cfd = ::accept(fd, nullptr, nullptr);
    if (cfd < 0) continue;
    std::thread(HandleClient, cfd, service).detach();
  }
}

}  // namespace tiny
