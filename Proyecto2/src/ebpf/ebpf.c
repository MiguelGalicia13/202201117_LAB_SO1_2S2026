#include <linux/bpf.h>
#include <bpf/bpf_helpers.h>
struct sys_call_kill{
    unsigned long long unused;
    long syscall_nr;
    long pid;
    long sig;
};

struct kill_event{
    __u32 pid_emisor;
    __s32 pid_receptor;
    __s32 sig;
};

struct {
    __uint(type, BPF_MAP_TYPE_RINGBUF);
    __uint(max_entries, 1 << 16); // 64 KB de búfer circular
} events SEC(".maps");
SEC("tracepoint/syscalls/sys_enter_kill")
int detect_kill(struct sys_call_kill *ctx){
    // if flag is not SYSKILL -> 9 and is not SIGTERM -> 15 skip
    if (ctx->sig != 9 && ctx->sig != 15) {
            return 0;
    }

    // e-> syscall executed valid
    struct kill_event *e = bpf_ringbuf_reserve(&events, sizeof(*e), 0);
    if (!e) {
        return 0;
    }
    
    e->pid_emisor = bpf_get_current_pid_tgid() >> 32;
    e->pid_receptor = (__s32)ctx->pid;
    e->sig = (__s32)ctx->sig;

    bpf_ringbuf_submit(e, 0);
    return 0;
}
char __license[] SEC("license") = "Dual MIT/GPL";