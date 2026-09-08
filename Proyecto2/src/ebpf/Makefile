CFLAGS := -target bpf -O2 -g -Wall -I/usr/include

all: bpf_kill.o

bpf_kill.o: ebpf.c
	clang $(CFLAGS) -c $< -o $@

clean:
	rm -f bpf_kill.o
