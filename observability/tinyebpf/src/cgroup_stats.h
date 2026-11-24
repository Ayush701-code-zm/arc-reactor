#pragma once

#include <stddef.h>

struct tiny_cgroup_sample {
	unsigned long long cpu_usec;
	unsigned long long memory_bytes;
};

int tiny_read_cgroup_stats(const char *cgroup_path, struct tiny_cgroup_sample *out);
int tiny_cgroup_path_from_pid(int pid, char *out, size_t out_len);
