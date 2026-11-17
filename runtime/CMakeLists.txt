cmake_minimum_required(VERSION 3.20)
project(tiny_runtime LANGUAGES CXX)

set(CMAKE_CXX_STANDARD 20)
set(CMAKE_CXX_STANDARD_REQUIRED ON)
set(CMAKE_EXPORT_COMPILE_COMMANDS ON)

add_library(tiny_process
  src/process/supervisor.cc
  src/process/cgroup.cc
  src/process/namespace.cc
)
target_include_directories(tiny_process PUBLIC ${CMAKE_CURRENT_SOURCE_DIR}/include)

add_executable(tinyruntimed
  src/main.cc
  src/http_server.cc
  src/runtime_service.cc
)
target_link_libraries(tinyruntimed PRIVATE tiny_process)

enable_testing()
add_executable(tiny_process_test tests/supervisor_test.cc)
target_link_libraries(tiny_process_test PRIVATE tiny_process)
add_test(NAME tiny_process_test COMMAND tiny_process_test)
