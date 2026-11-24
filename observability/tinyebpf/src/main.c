#include "cgroup_stats.h"
#include "ebpf_attach.h"

#include <errno.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/socket.h>
#include <sys/stat.h>
#include <sys/un.h>
#include <unistd.h>

struct watched {
	char instance_id[64];
	char cgroup_path[512];
	int pid;
	int used;
};

static struct watched g_tracked[64];
static char g_sock_path[256];

static struct watched *find_tracked(const char *id) {
	for (int i = 0; i < 64; i++) {
		if (g_tracked[i].used && strcmp(g_tracked[i].instance_id, id) == 0) return &g_tracked[i];
	}
	return NULL;
}

static struct watched *alloc_tracked(void) {
	for (int i = 0; i < 64; i++) {
		if (!g_tracked[i].used) return &g_tracked[i];
	}
	return NULL;
}

static const char *json_get_string(const char *body, const char *key, char *out, size_t out_len) {
	char pat[64];
	snprintf(pat, sizeof(pat), "\"%s\"", key);
	const char *p = strstr(body, pat);
	if (!p) return NULL;
	p = strchr(p + strlen(pat), '"');
	if (!p) return NULL;
	p++;
	const char *end = strchr(p, '"');
	if (!end) return NULL;
	size_t n = (size_t)(end - p);
	if (n >= out_len) n = out_len - 1;
	memcpy(out, p, n);
	out[n] = '\0';
	return out;
}

static int json_get_int(const char *body, const char *key, int *out) {
	char pat[64];
	snprintf(pat, sizeof(pat), "\"%s\"", key);
	const char *p = strstr(body, pat);
	if (!p) return -1;
	p = strchr(p, ':');
	if (!p) return -1;
	*out = atoi(p + 1);
	return 0;
}

static void reply(int fd, const char *json) {
	dprintf(fd, "%s\n", json);
}

static void handle_line(int fd, const char *line) {
	char op[32] = {0};
	if (!json_get_string(line, "op", op, sizeof(op))) {
		reply(fd, "{\"ok\":false,\"error\":\"missing op\"}");
		return;
	}
	if (strcmp(op, "ping") == 0) {
		char buf[128];
		snprintf(buf, sizeof(buf), "{\"ok\":true,\"ebpf\":%s}", tiny_ebpf_available() ? "true" : "false");
		reply(fd, buf);
		return;
	}
	char instance_id[64] = {0};
	json_get_string(line, "instance_id", instance_id, sizeof(instance_id));
	if (strcmp(op, "watch") == 0) {
		int pid = 0;
		json_get_int(line, "pid", &pid);
		char cgroup_path[512] = {0};
		json_get_string(line, "cgroup_path", cgroup_path, sizeof(cgroup_path));
		if (cgroup_path[0] == '\0' && pid > 0) {
			if (tiny_cgroup_path_from_pid(pid, cgroup_path, sizeof(cgroup_path)) != 0) {
				reply(fd, "{\"ok\":false,\"error\":\"cgroup path from pid failed\"}");
				return;
			}
		}
		if (instance_id[0] == '\0' || cgroup_path[0] == '\0') {
			reply(fd, "{\"ok\":false,\"error\":\"instance_id and cgroup_path/pid required\"}");
			return;
		}
		struct watched *t = find_tracked(instance_id);
		if (!t) t = alloc_tracked();
		if (!t) {
			reply(fd, "{\"ok\":false,\"error\":\"track table full\"}");
			return;
		}
		memset(t, 0, sizeof(*t));
		snprintf(t->instance_id, sizeof(t->instance_id), "%s", instance_id);
		snprintf(t->cgroup_path, sizeof(t->cgroup_path), "%s", cgroup_path);
		t->pid = pid;
		t->used = 1;
		char err[256] = {0};
		const int wr = tiny_ebpf_watch(instance_id, cgroup_path, err, sizeof(err));
		char buf[512];
		if (wr < 0) {
			snprintf(buf, sizeof(buf), "{\"ok\":false,\"ebpf\":false,\"error\":\"%s\"}", err[0] ? err : "watch failed");
			reply(fd, buf);
			return;
		}
		snprintf(buf, sizeof(buf),
			 "{\"ok\":true,\"ebpf\":%s,\"error\":\"%s\"}",
			 tiny_ebpf_available() && wr == 0 ? "true" : "false",
			 err);
		reply(fd, buf);
		return;
	}
	if (strcmp(op, "unwatch") == 0) {
		if (instance_id[0] == '\0') {
			reply(fd, "{\"ok\":false,\"error\":\"instance_id required\"}");
			return;
		}
		struct watched *t = find_tracked(instance_id);
		if (t) t->used = 0;
		char err[128] = {0};
		tiny_ebpf_unwatch(instance_id, err, sizeof(err));
		reply(fd, "{\"ok\":true}");
		return;
	}
	if (strcmp(op, "sample") == 0) {
		if (instance_id[0] == '\0') {
			reply(fd, "{\"ok\":false,\"error\":\"instance_id required\"}");
			return;
		}
		struct watched *t = find_tracked(instance_id);
		if (!t) {
			reply(fd, "{\"ok\":false,\"error\":\"not watched\"}");
			return;
		}
		struct tiny_cgroup_sample cg = {0};
		tiny_read_cgroup_stats(t->cgroup_path, &cg);
		struct tiny_traffic tr = {0};
		tiny_ebpf_sample_traffic(instance_id, &tr);
		char buf[512];
		snprintf(buf, sizeof(buf),
			 "{\"ok\":true,\"ebpf\":%s,\"cpu_usec\":%llu,\"memory_bytes\":%llu,"
			 "\"rx_packets\":%llu,\"rx_bytes\":%llu,\"tx_packets\":%llu,\"tx_bytes\":%llu}",
			 tiny_ebpf_available() ? "true" : "false",
			 (unsigned long long)cg.cpu_usec,
			 (unsigned long long)cg.memory_bytes,
			 (unsigned long long)tr.rx_packets,
			 (unsigned long long)tr.rx_bytes,
			 (unsigned long long)tr.tx_packets,
			 (unsigned long long)tr.tx_bytes);
		reply(fd, buf);
		return;
	}
	reply(fd, "{\"ok\":false,\"error\":\"unknown op\"}");
}

static int serve(const char *sock_path) {
	unlink(sock_path);
	int s = socket(AF_UNIX, SOCK_STREAM, 0);
	if (s < 0) return -1;
	struct sockaddr_un addr;
	memset(&addr, 0, sizeof(addr));
	addr.sun_family = AF_UNIX;
	snprintf(addr.sun_path, sizeof(addr.sun_path), "%s", sock_path);
	if (bind(s, (struct sockaddr *)&addr, sizeof(addr)) != 0) {
		close(s);
		return -1;
	}
	chmod(sock_path, 0660);
	if (listen(s, 16) != 0) {
		close(s);
		return -1;
	}
	fprintf(stderr, "tinyebpf listening on unix:%s ebpf=%s\n", sock_path,
		tiny_ebpf_available() ? "ready" : "optional-skipped");
	for (;;) {
		int c = accept(s, NULL, NULL);
		if (c < 0) continue;
		char buf[1024];
		ssize_t n = read(c, buf, sizeof(buf) - 1);
		if (n > 0) {
			buf[n] = '\0';
			char *nl = strchr(buf, '\n');
			if (nl) *nl = '\0';
			handle_line(c, buf);
		}
		close(c);
	}
}

int main(int argc, char **argv) {
	const char *sock = getenv("TINY_EBPF_SOCK");
	if (!sock || !sock[0]) sock = "/tmp/tiny-aws-ebpf.sock";
	snprintf(g_sock_path, sizeof(g_sock_path), "%s", sock);

	const char *obj = getenv("TINY_EBPF_OBJECT");
	char default_obj[512];
	if (!obj || !obj[0]) {
		snprintf(default_obj, sizeof(default_obj), "%s/cgroup_skb.bpf.o",
			 getenv("TINY_EBPF_OBJECT_DIR") ? getenv("TINY_EBPF_OBJECT_DIR") : ".");
		obj = default_obj;
	}
	char err[256] = {0};
	if (tiny_ebpf_init(obj, err, sizeof(err)) != 0) {
		fprintf(stderr, "tinyebpf: bpf init skipped: %s\n", err);
	}
	(void)argc;
	(void)argv;
	return serve(g_sock_path) == 0 ? 0 : 1;
}
