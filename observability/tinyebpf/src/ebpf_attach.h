#pragma once

#include <stddef.h>
#include <stdint.h>

struct tiny_traffic {
	uint64_t rx_packets;
	uint64_t rx_bytes;
	uint64_t tx_packets;
	uint64_t tx_bytes;
};

int tiny_ebpf_init(const char *bpf_object_path, char *err, size_t err_len);
void tiny_ebpf_shutdown(void);
int tiny_ebpf_available(void);

int tiny_ebpf_watch(const char *instance_id, const char *cgroup_path, char *err, size_t err_len);
int tiny_ebpf_unwatch(const char *instance_id, char *err, size_t err_len);
int tiny_ebpf_sample_traffic(const char *instance_id, struct tiny_traffic *out);
