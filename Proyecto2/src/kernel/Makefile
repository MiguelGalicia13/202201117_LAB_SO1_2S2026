obj-m += sysinfo.o

all:
	make -C /lib/modules/$(shell uname -r)/build M=$(PWD) modules

clean:
	make -C /lib/modules/$(shell uname -r)/build M=$(PWD) clean

load:
	sudo insmod sysinfo.ko

unload:
	sudo rmmod sysinfo

update:
	$(MAKE) unload || true
	$(MAKE) clean
	$(MAKE) all
	$(MAKE) load
log:
	sudo dmesg | tail -20
show:
	cat /proc/continfo_pr2_so1_202201117