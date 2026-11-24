#include <linux/types.h>
#include <linux/bpf.h>
#include <bpf/bpf_helpers.h>

struct traffic {
	__u64 rx_packets;
	__u64 rx_bytes;
	__u64 tx_packets;
	__u64 tx_bytes;
};

struct {
	__uint(type, BPF_MAP_TYPE_HASH);
	__type(key, __u64);
	__type(value, struct traffic);
	__uint(max_entries, 1024);
} traffic_map SEC(".maps");

static __always_inline int bump(__u64 *pkts, __u64 *bytes, __u32 len) {
	__sync_fetch_and_add(pkts, 1);
	__sync_fetch_and_add(bytes, len);
	return 1;
}

SEC("cgroup_skb/ingress")
int tiny_cgroup_ingress(struct __sk_buff *skb) {
	__u64 cg = bpf_get_current_cgroup_id();
	struct traffic *t = bpf_map_lookup_elem(&traffic_map, &cg);
	struct traffic zero = {};
	if (!t) {
		bpf_map_update_elem(&traffic_map, &cg, &zero, BPF_ANY);
		t = bpf_map_lookup_elem(&traffic_map, &cg);
		if (!t) return 1;
	}
	return bump(&t->rx_packets, &t->rx_bytes, skb->len);
}

SEC("cgroup_skb/egress")
int tiny_cgroup_egress(struct __sk_buff *skb) {
	__u64 cg = bpf_get_current_cgroup_id();
	struct traffic *t = bpf_map_lookup_elem(&traffic_map, &cg);
	struct traffic zero = {};
	if (!t) {
		bpf_map_update_elem(&traffic_map, &cg, &zero, BPF_ANY);
		t = bpf_map_lookup_elem(&traffic_map, &cg);
		if (!t) return 1;
	}
	return bump(&t->tx_packets, &t->tx_bytes, skb->len);
}

char LICENSE[] SEC("license") = "GPL";
