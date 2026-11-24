#include "ebpf_attach.h"

#include <errno.h>
#include <fcntl.h>
#include <stdio.h>
#include <string.h>
#include <sys/stat.h>
#include <unistd.h>

#if defined(__linux__) && defined(TINY_HAVE_LIBBPF)
#include <bpf/bpf.h>
#include <bpf/libbpf.h>
#include <linux/bpf.h>
#endif

struct watch_entry {
	char instance_id[64];
	char cgroup_path[512];
	int cgroup_fd;
	uint64_t cgroup_id;
	int used;
};

static struct watch_entry g_watches[64];
#if defined(__linux__) && defined(TINY_HAVE_LIBBPF)
static struct bpf_object *g_obj;
static struct bpf_link *g_links[128];
static int g_link_count;
static int g_map_fd = -1;
static int g_ready;
#endif

static struct watch_entry *find_watch(const char *instance_id) {
	for (int i = 0; i < 64; i++) {
		if (g_watches[i].used && strcmp(g_watches[i].instance_id, instance_id) == 0) {
			return &g_watches[i];
		}
	}
	return NULL;
}

static struct watch_entry *alloc_watch(void) {
	for (int i = 0; i < 64; i++) {
		if (!g_watches[i].used) return &g_watches[i];
	}
	return NULL;
}

int tiny_ebpf_available(void) {
#if defined(__linux__) && defined(TINY_HAVE_LIBBPF)
	return g_ready;
#else
	return 0;
#endif
}

int tiny_ebpf_init(const char *bpf_object_path, char *err, size_t err_len) {
#if !(defined(__linux__) && defined(TINY_HAVE_LIBBPF))
	(void)bpf_object_path;
	if (err && err_len) snprintf(err, err_len, "libbpf not built into tinyebpf");
	return -1;
#else
	if (g_obj) return 0;
	libbpf_set_strict_mode(LIBBPF_STRICT_ALL);
	g_obj = bpf_object__open_file(bpf_object_path, NULL);
	if (!g_obj) {
		if (err && err_len) snprintf(err, err_len, "bpf_object__open_file failed: %s", strerror(errno));
		return -1;
	}
	if (bpf_object__load(g_obj)) {
		if (err && err_len) snprintf(err, err_len, "bpf_object__load failed: %s", strerror(errno));
		bpf_object__close(g_obj);
		g_obj = NULL;
		return -1;
	}
	struct bpf_map *map = bpf_object__find_map_by_name(g_obj, "traffic_map");
	if (!map) {
		if (err && err_len) snprintf(err, err_len, "traffic_map missing");
		bpf_object__close(g_obj);
		g_obj = NULL;
		return -1;
	}
	g_map_fd = bpf_map__fd(map);
	g_ready = 1;
	return 0;
#endif
}

void tiny_ebpf_shutdown(void) {
#if defined(__linux__) && defined(TINY_HAVE_LIBBPF)
	for (int i = 0; i < g_link_count; i++) {
		if (g_links[i]) bpf_link__destroy(g_links[i]);
		g_links[i] = NULL;
	}
	g_link_count = 0;
	for (int i = 0; i < 64; i++) {
		if (g_watches[i].used && g_watches[i].cgroup_fd >= 0) close(g_watches[i].cgroup_fd);
		g_watches[i].used = 0;
	}
	if (g_obj) bpf_object__close(g_obj);
	g_obj = NULL;
	g_map_fd = -1;
	g_ready = 0;
#endif
}

int tiny_ebpf_watch(const char *instance_id, const char *cgroup_path, char *err, size_t err_len) {
	if (!instance_id || !cgroup_path) return -1;
	if (find_watch(instance_id)) return 0;
	struct watch_entry *w = alloc_watch();
	if (!w) {
		if (err && err_len) snprintf(err, err_len, "watch table full");
		return -1;
	}
	memset(w, 0, sizeof(*w));
	snprintf(w->instance_id, sizeof(w->instance_id), "%s", instance_id);
	snprintf(w->cgroup_path, sizeof(w->cgroup_path), "%s", cgroup_path);
	w->cgroup_fd = -1;
	w->used = 1;

#if !(defined(__linux__) && defined(TINY_HAVE_LIBBPF))
	if (err && err_len) snprintf(err, err_len, "ebpf unavailable; cgroup stats only");
	return 1; /* soft: tracked for stats, no skb attach */
#else
	if (!g_ready) {
		if (err && err_len) snprintf(err, err_len, "ebpf not initialized");
		return 1;
	}
	int cfd = open(cgroup_path, O_RDONLY);
	if (cfd < 0) {
		if (err && err_len) snprintf(err, err_len, "open cgroup: %s", strerror(errno));
		w->used = 0;
		return -1;
	}
	w->cgroup_fd = cfd;
	struct bpf_program *ing = bpf_object__find_program_by_name(g_obj, "tiny_cgroup_ingress");
	struct bpf_program *egr = bpf_object__find_program_by_name(g_obj, "tiny_cgroup_egress");
	if (!ing || !egr) {
		if (err && err_len) snprintf(err, err_len, "missing skb programs");
		close(cfd);
		w->used = 0;
		return -1;
	}
	struct bpf_link *li = bpf_program__attach_cgroup(ing, cfd);
	if (!li) {
		if (err && err_len) snprintf(err, err_len, "attach ingress: %s", strerror(errno));
		close(cfd);
		w->used = 0;
		return -1;
	}
	struct bpf_link *le = bpf_program__attach_cgroup(egr, cfd);
	if (!le) {
		bpf_link__destroy(li);
		if (err && err_len) snprintf(err, err_len, "attach egress: %s", strerror(errno));
		close(cfd);
		w->used = 0;
		return -1;
	}
	if (g_link_count + 2 > 128) {
		bpf_link__destroy(li);
		bpf_link__destroy(le);
		close(cfd);
		w->used = 0;
		if (err && err_len) snprintf(err, err_len, "link table full");
		return -1;
	}
	g_links[g_link_count++] = li;
	g_links[g_link_count++] = le;
	struct stat st;
	if (fstat(cfd, &st) == 0) {
		w->cgroup_id = (uint64_t)st.st_ino;
	}
	return 0;
#endif
}

int tiny_ebpf_unwatch(const char *instance_id, char *err, size_t err_len) {
	(void)err;
	(void)err_len;
	struct watch_entry *w = find_watch(instance_id);
	if (!w) return 0;
	if (w->cgroup_fd >= 0) close(w->cgroup_fd);
	w->used = 0;
	return 0;
}

int tiny_ebpf_sample_traffic(const char *instance_id, struct tiny_traffic *out) {
	memset(out, 0, sizeof(*out));
	struct watch_entry *w = find_watch(instance_id);
	if (!w) return -1;
#if defined(__linux__) && defined(TINY_HAVE_LIBBPF)
	if (!g_ready || g_map_fd < 0 || w->cgroup_id == 0) {
		/* Without stored cgroup_id, return zeros; counters still accumulate under live cg id. */
		return 0;
	}
	if (bpf_map_lookup_elem(g_map_fd, &w->cgroup_id, out) != 0) {
		memset(out, 0, sizeof(*out));
	}
	return 0;
#else
	(void)w;
	return 0;
#endif
}
