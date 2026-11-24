cmake_minimum_required(VERSION 3.20)
project(tiny_runtime LANGUAGES CXX)

set(CMAKE_CXX_STANDARD 20)
set(CMAKE_CXX_STANDARD_REQUIRED ON)
set(CMAKE_EXPORT_COMPILE_COMMANDS ON)

add_library(tiny_process
  src/process/supervisor.cc
  src/process/cgroup.cc
  src/process/namespace.cc
  src/process/volume.cc
  src/process/network.cc
)
target_include_directories(tiny_process PUBLIC ${CMAKE_CURRENT_SOURCE_DIR}/include)

enable_testing()
add_executable(tiny_process_test tests/supervisor_test.cc)
target_link_libraries(tiny_process_test PRIVATE tiny_process)
add_test(NAME tiny_process_test COMMAND tiny_process_test)

find_package(Protobuf REQUIRED)
find_package(PkgConfig REQUIRED)
pkg_check_modules(GRPCPP REQUIRED IMPORTED_TARGET grpc++)

find_program(GRPC_CPP_PLUGIN grpc_cpp_plugin REQUIRED)

set(PROTO_ROOT "${CMAKE_CURRENT_SOURCE_DIR}/../proto")
set(RUNTIME_PROTO "${PROTO_ROOT}/tiny/v1/runtime.proto")
set(NODE_PROTO "${PROTO_ROOT}/tiny/v1/node.proto")
set(GEN_DIR "${CMAKE_CURRENT_BINARY_DIR}/gen")
file(MAKE_DIRECTORY "${GEN_DIR}")

set(RUNTIME_PB_CC "${GEN_DIR}/tiny/v1/runtime.pb.cc")
set(RUNTIME_PB_H "${GEN_DIR}/tiny/v1/runtime.pb.h")
set(RUNTIME_GRPC_CC "${GEN_DIR}/tiny/v1/runtime.grpc.pb.cc")
set(RUNTIME_GRPC_H "${GEN_DIR}/tiny/v1/runtime.grpc.pb.h")
set(NODE_PB_CC "${GEN_DIR}/tiny/v1/node.pb.cc")
set(NODE_PB_H "${GEN_DIR}/tiny/v1/node.pb.h")
set(NODE_GRPC_CC "${GEN_DIR}/tiny/v1/node.grpc.pb.cc")
set(NODE_GRPC_H "${GEN_DIR}/tiny/v1/node.grpc.pb.h")

add_custom_command(
  OUTPUT "${RUNTIME_PB_CC}" "${RUNTIME_PB_H}" "${RUNTIME_GRPC_CC}" "${RUNTIME_GRPC_H}"
         "${NODE_PB_CC}" "${NODE_PB_H}" "${NODE_GRPC_CC}" "${NODE_GRPC_H}"
  COMMAND ${CMAKE_COMMAND} -E make_directory "${GEN_DIR}/tiny/v1"
  COMMAND ${Protobuf_PROTOC_EXECUTABLE}
    --proto_path=${PROTO_ROOT}
    --cpp_out=${GEN_DIR}
    --grpc_out=${GEN_DIR}
    --plugin=protoc-gen-grpc=${GRPC_CPP_PLUGIN}
    ${RUNTIME_PROTO}
    ${NODE_PROTO}
  DEPENDS "${RUNTIME_PROTO}" "${NODE_PROTO}"
  COMMENT "Generating C++ gRPC stubs from runtime.proto and node.proto"
  VERBATIM
)

add_executable(tinyruntimed
  src/main.cc
  src/grpc_server.cc
  "${RUNTIME_PB_CC}"
  "${RUNTIME_GRPC_CC}"
)
target_include_directories(tinyruntimed PRIVATE
  ${CMAKE_CURRENT_SOURCE_DIR}/include
  ${GEN_DIR}
  ${Protobuf_INCLUDE_DIRS}
)
target_link_libraries(tinyruntimed PRIVATE
  tiny_process
  protobuf::libprotobuf
  PkgConfig::GRPCPP
)

add_executable(tinynodeagent
  src/node_agent.cc
  "${RUNTIME_PB_CC}"
  "${RUNTIME_GRPC_CC}"
  "${NODE_PB_CC}"
  "${NODE_GRPC_CC}"
)
target_include_directories(tinynodeagent PRIVATE
  ${GEN_DIR}
  ${Protobuf_INCLUDE_DIRS}
)
target_link_libraries(tinynodeagent PRIVATE
  protobuf::libprotobuf
  PkgConfig::GRPCPP
)
target_compile_features(tinynodeagent PRIVATE cxx_std_20)
