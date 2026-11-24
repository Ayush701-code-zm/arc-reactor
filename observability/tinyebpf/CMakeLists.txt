cmake_minimum_required(VERSION 3.20)
project(tinyebpf C)

set(CMAKE_C_STANDARD 11)

add_executable(tinyebpf
  src/main.c
  src/cgroup_stats.c
  src/ebpf_attach.c
)
target_include_directories(tinyebpf PRIVATE ${CMAKE_CURRENT_SOURCE_DIR}/src)

find_package(PkgConfig)
set(TINY_HAVE_LIBBPF 0)
if(CMAKE_SYSTEM_NAME STREQUAL "Linux" AND PkgConfig_FOUND)
  pkg_check_modules(LIBBPF libbpf)
  if(LIBBPF_FOUND)
    find_program(CLANG_BIN clang)
    if(CLANG_BIN)
      set(BPF_SRC ${CMAKE_CURRENT_SOURCE_DIR}/bpf/cgroup_skb.bpf.c)
      set(BPF_OBJ ${CMAKE_CURRENT_BINARY_DIR}/cgroup_skb.bpf.o)
      if(CMAKE_SYSTEM_PROCESSOR MATCHES "aarch64|arm64")
        set(BPF_ARCH_DEFINE -D__TARGET_ARCH_arm64)
        set(BPF_ARCH_INCLUDE -I/usr/include/aarch64-linux-gnu)
      else()
        set(BPF_ARCH_DEFINE -D__TARGET_ARCH_x86)
        set(BPF_ARCH_INCLUDE -I/usr/include/x86_64-linux-gnu)
      endif()
      add_custom_command(
        OUTPUT ${BPF_OBJ}
        COMMAND ${CLANG_BIN} -O2 -g -target bpf ${BPF_ARCH_DEFINE}
          -c ${BPF_SRC} -o ${BPF_OBJ}
          ${BPF_ARCH_INCLUDE} -I/usr/include ${LIBBPF_CFLAGS}
        DEPENDS ${BPF_SRC}
        COMMENT "Building cgroup_skb BPF object"
        VERBATIM
      )
      add_custom_target(tinyebpf_bpf ALL DEPENDS ${BPF_OBJ})
      add_dependencies(tinyebpf tinyebpf_bpf)
      target_compile_definitions(tinyebpf PRIVATE TINY_HAVE_LIBBPF=1)
      target_include_directories(tinyebpf PRIVATE ${LIBBPF_INCLUDE_DIRS})
      target_link_libraries(tinyebpf PRIVATE ${LIBBPF_LIBRARIES})
      set(TINY_HAVE_LIBBPF 1)
      message(STATUS "tinyebpf: libbpf enabled")
    endif()
  endif()
endif()

if(NOT TINY_HAVE_LIBBPF)
  message(STATUS "tinyebpf: building without libbpf (cgroup stats + soft skip)")
endif()

install(TARGETS tinyebpf RUNTIME DESTINATION bin)
