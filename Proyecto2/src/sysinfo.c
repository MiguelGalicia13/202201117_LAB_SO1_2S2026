#include <linux/fscrypt.h>
#include <linux/module.h>
#include <linux/kernel.h>
#include <linux/string.h> 
#include <linux/init.h>
#include <linux/proc_fs.h> 
#include <linux/seq_file.h> 
#include <linux/mm.h> 
#include <linux/sched.h> 
#include <linux/timer.h> 
#include <linux/jiffies.h> 
#include <linux/uaccess.h>
#include <linux/tty.h>
#include <linux/sched/signal.h>
#include <linux/fs.h>        
#include <linux/slab.h>      
#include <linux/sched/mm.h>
#include <linux/binfmts.h>
#include <linux/timekeeping.h>
//#include <time.h>

MODULE_LICENSE("GPL");
MODULE_AUTHOR("Miguel Galicia");
MODULE_DESCRIPTION("Modulo para leer informacion de memoria y CPU de contenedores en JSON");
MODULE_VERSION("1.1");

#define PROC_NAME "continfo_pr2_so1_202201117"
#define MAX_CMDLINE_LENGTH 256

// Obtiene la línea de comandos de un proceso y sanitiza comillas para JSON
static char *get_process_cmdline(struct task_struct *task) {
    struct mm_struct *mm;
    char *cmdline, *p;
    unsigned long arg_start, arg_end;
    int i, len;

    cmdline = kmalloc(MAX_CMDLINE_LENGTH, GFP_KERNEL);
    if (!cmdline)
        return NULL;

    mm = get_task_mm(task);
    if (!mm) {
        kfree(cmdline);
        return NULL;
    }

    down_read(&mm->mmap_lock);
    arg_start = mm->arg_start;
    arg_end = mm->arg_end;
    up_read(&mm->mmap_lock);

    len = arg_end - arg_start;
    if (len <= 0) {
        mmput(mm);
        kfree(cmdline);
        return NULL;
    }

    if (len > MAX_CMDLINE_LENGTH - 1)
        len = MAX_CMDLINE_LENGTH - 1;

    if (access_process_vm(task, arg_start, cmdline, len, 0) != len) {
        mmput(mm);
        kfree(cmdline);
        return NULL;
    }

    cmdline[len] = '\0';

    // Reemplaza caracteres nulos por espacios y comillas dobles para no romper el JSON
    p = cmdline;
    for (i = 0; i < len; i++) {
        if (p[i] == '\0')
            p[i] = ' ';
        else if (p[i] == '"')
            p[i] = '\'';
    }

    mmput(mm);
    return cmdline;
}

// Función para mostrar la información en /proc en formato JSON
static int sysinfo_show(struct seq_file *m, void *v) {
    struct sysinfo si;
    struct task_struct *task;
    int first_process = 1;
    unsigned long totalram_kb;
    unsigned long freeram_kb;
    u64 now_ns = ktime_get_ns();
    unsigned long total_jiffies = jiffies;

    si_meminfo(&si);
    totalram_kb = si.totalram << (PAGE_SHIFT - 10);
    freeram_kb = si.freeram << (PAGE_SHIFT -10);
    unsigned int totalram_mb = (si.totalram << (PAGE_SHIFT - 10)) / 1024;
    unsigned int freeram_mb  = (si.freeram << (PAGE_SHIFT - 10)) / 1024;
    unsigned int usedram_mb =  ((si.totalram << (PAGE_SHIFT - 10)) / 1024)- ((si.freeram << (PAGE_SHIFT - 10)) / 1024);
    seq_printf(m, "{\n");
    seq_printf(m, "  \"Totalram\": %u,\n", totalram_mb);
    seq_printf(m, "  \"Freeram\": %u,\n", freeram_mb);
    seq_printf(m, "  \"Usedram\": %u,\n", usedram_mb);
    seq_printf(m, "  \"Procs\": %d,\n", si.procs);
    seq_printf(m, "  \"Processes\": [\n");

    for_each_process(task) {
        // Identificar el supervisor de containerd
        if (strcmp(task->comm, "containerd-shim") == 0) {
            unsigned long total_vsz = 0;
            unsigned long total_rss = 0;
            u64 total_cpu_ns = 0;
            u64 start_time_ns = 0;
            char *cmdline = NULL;
            unsigned long mem_usage = 0;
            unsigned long cpu_usage = 0;
            if (task->mm) {
                // Obtenemos el uso de vsz haciendo un shift de PAGE_SHIFT - 10, un PAGE_SHIFT es la cantidad de bits que se necesitan para representar un byte
                // dividimos entre 1024 para obtener los datos en MB
                total_vsz = (task->mm->total_vm << (PAGE_SHIFT - 10))/1024;
                // Obtenemos el uso de rss haciendo un shift de PAGE_SHIFT - 10
                total_rss = (get_mm_rss(task->mm) << (PAGE_SHIFT - 10))/1024;
                // Obtenemos el uso de memoria en porcentaje
                mem_usage = (total_rss * 100) / totalram_mb;
            }
            unsigned long total_time = task->utime + task->stime;
            cpu_usage = (total_time * 10000) /  total_jiffies;
            cmdline = get_process_cmdline(task);
            if(!first_process){
                seq_printf(m, ",\n");
            }else{
                first_process = 0;
            }
            struct task_struct *child;
            list_for_each_entry(child, &task->children, sibling){
                if(child->mm){
                    total_rss =( get_mm_rss(child->mm)<<(PAGE_SHIFT-10))/1024;
                    mem_usage = (total_rss*1000)/totalram_mb;

                    unsigned long total_time_child = child->utime + child->stime;
                    u64 time_since_start_jiffies = get_jiffies_64();

                    if(time_since_start_jiffies > 0 && total_jiffies >0){
                        cpu_usage = (total_time_child *100)/total_jiffies;
                        cpu_usage = cpu_usage/num_online_cpus();
                    }else{
                        cpu_usage = 67; //evita valores no validos
                    }
                    break; //  salimos despues del primer hijo
                    
                }
            }

            seq_printf(m, "     {\n");
            seq_printf(m, "        \"PID\": %d,\n", task->pid);
            seq_printf(m, "        \"Name\": \"%s\",\n", task->comm);
            seq_printf(m, "        \"Cmdline\": \"%s\",\n", cmdline ? cmdline : "N/A");
            seq_printf(m, "        \"vsz\": %lu,\n", total_vsz);
            seq_printf(m, "        \"rss\": %lu,\n", total_rss);
            seq_printf(m, "        \"Memory_Usage\": %lu.%lu,\n", mem_usage / 10, mem_usage % 10);
            seq_printf(m, "        \"CPU_Usage\": %lu.%lu\n", cpu_usage / 100, cpu_usage % 100);
            seq_printf(m, "     }");
            
        }
    }

    seq_printf(m, "\n  ]\n}\n");
    return 0;
}

static int sysinfo_open(struct inode *inode, struct file *file) {
    return single_open(file, sysinfo_show, NULL);
}

static const struct proc_ops sysinfo_ops = {
    .proc_open = sysinfo_open,
    .proc_read = seq_read,
};

static int __init sysinfo_init(void) {
    proc_create(PROC_NAME, 0, NULL, &sysinfo_ops);
    printk(KERN_INFO "sysinfo_json modulo cargado correctamente\n");
    return 0;
}

static void __exit sysinfo_exit(void) {
    remove_proc_entry(PROC_NAME, NULL);
    printk(KERN_INFO "sysinfo_json modulo desinstalado\n");
}

module_init(sysinfo_init);
module_exit(sysinfo_exit);