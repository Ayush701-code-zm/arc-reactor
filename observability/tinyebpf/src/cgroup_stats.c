#include "cgroup_stats.h"

#include <ctype.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

static int read_u64_file(const char *path, unsigned long long *out) {
	FILE *f = fopen(path, "r");
	if (!f) return -1;
	unsigned long long v = 0;
	const int n = fscanf(f, "%llu", &v);
	fclose(f);
	if (n != 1) return -1;
	*out = v;
	return 0;
}

int tiny_read_cgroup_stats(const char *cgroup_path, struct tiny_cgroup_sample *out) {
	if (!cgroup_path || !out) return -1;
	memset(out, 0, sizeof(*out));
	char path[512];
	snprintf(path, sizeof(path), "%s/memory.current", cgroup_path);
	if (read_u64_file(path, &out->memory_bytes) != 0) {
		out->memory_bytes = 0;
	}
	snprintf(path, sizeof(path), "%s/cpu.stat", cgroup_path);
	FILE *f = fopen(path, "r");
	if (!f) return 0;
	char line[256];
	while (fgets(line, sizeof(line), f)) {
		unsigned long long v = 0;
		if (sscanf(line, "usage_usec %llu", &v) == 1) {
			out->cpu_usec = v;
			break;
		}
	}
	fclose(f);
	return 0;
}

int tiny_cgroup_path_from_pid(int pid, char *out, size_t out_len) {
	if (pid <= 0 || !out || out_len < 8) return -1;
	char proc_path[64];
	snprintf(proc_path, sizeof(proc_path), "/proc/%d/cgroup", pid);
	FILE *f = fopen(proc_path, "r");
	if (!f) return -1;
	char line[512];
	int rc = -1;
	while (fgets(line, sizeof(line), f)) {
		if (strncmp(line, "0::", 3) != 0) continue;
		char *rel = line + 3;
		size_t n = strlen(rel);
		while (n > 0 && (rel[n - 1] == '\n' || rel[n - 1] == '\r')) {
			rel[--n] = '\0';
		}
		if (n == 0 || (n == 1 && rel[0] == '/')) {
			snprintf(out, out_len, "/sys/fs/cgroup");
		} else if (rel[0] == '/') {
			snprintf(out, out_len, "/sys/fs/cgroup%s", rel);
		} else {
			snprintf(out, out_len, "/sys/fs/cgroup/%s", rel);
		}
		rc = 0;
		break;
	}
	fclose(f);
	return rc;
}
