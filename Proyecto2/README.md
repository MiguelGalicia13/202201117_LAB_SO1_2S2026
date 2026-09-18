# PROYECTO 2
## Miguel Galicia - 202201117


## 1. ARQUITECTURA DEL SISTEMA

Distribución de componentes y flujo de datos entre el espacio de kernel (Kernel Space), el daemon en espacio de usuario (User Space) y los servicios de almacenamiento/visualización:

```mermaid
graph TB
    subgraph KernelSpace["Kernel Space (Linux)"]
        direction TB
        KMod["Módulo de Kernel (sysinfo.ko)<br/><b>/proc/continfo_pr2_so1_202201117</b>"]
        Tracepoint["Tracepoint: syscalls/sys_enter_kill"]
        EBPFProg["Programa eBPF (bpf_kill.o)<br/><i>Filtro: SIGKILL (9) / SIGTERM (15)</i>"]
        RingBuf[("BPF RingBuffer<br/>64 KB")]

        Tracepoint --> EBPFProg
        EBPFProg --> RingBuf
    end

    subgraph UserSpace["User Space — Host Linux"]
        direction TB
        Cron["Cronjob del Sistema<br/><i>*/2 * * * * (add-container)</i>"]
        
        subgraph DaemonGo["Daemon en Go (daemon)"]
            direction TB
            KReader["Lector /proc<br/>(cada 20s)"]
            FIFOEngine["Gestor de Colas FIFO<br/>• Alto Consumo (Max: 2)<br/>• Bajo Consumo (Max: 3)<br/>• Purga de PIDs inactivos"]
            EBPFLoader["Lector RingBuffer (cilium/ebpf)<br/>Confirmación Real de Kill"]
            
            KReader --> FIFOEngine
            EBPFLoader -.->|"Evento confirmado"| FIFOEngine
        end

        subgraph DockerWorkload["Contenedores de Carga (Docker)"]
            C_High["Altos (2):<br/>• roldyoran/go-client<br/>• alpine (bucle bc)"]
            C_Low["Bajos (3):<br/>• alpine sleep 240"]
        end

        Cron -->|"Genera 5 contenedores"| DockerWorkload
        FIFOEngine -->|"syscall.Kill(pid, SIGKILL)"| DockerWorkload
        FIFOEngine -.->|"docker container prune -f"| DockerWorkload
    end

    subgraph TelemetryStack["Stack de Telemetría (Docker Compose)"]
        direction TB
        Valkey[("Valkey Server (Puerto 6379)<br/>• Strings: system:total_ram, free, used<br/>• Streams: stream:ram_history, stream:deleted_containers<br/>• ZSets: ranking:ram, ranking:cpu")]
        Grafana["Grafana Server (Puerto 3001)<br/>• Plugin: redis-datasource<br/>• Dashboard interactivo"]

        Valkey <-->|"Consulta métricas"| Grafana
    end

    %% Relaciones inter-componentes
    KMod -.->|"cat JSON"| KReader
    RingBuf -.->|"Lectura asíncrona"| EBPFLoader
    DaemonGo ==>|"Inserción de métricas y eventos"| Valkey
    Client["Usuario / Evaluador (Navegador)"] ==>|"HTTP :3001"| Grafana
```

---

## 2. DIAGRAMA DE FLUJO: CICLO PRINCIPAL DEL DAEMON Y DECISIONES FIFO

```mermaid
flowchart TD
    Start(["Inicio del Daemon (sudo ./run.sh)"]) --> InitServices["1. Compilar y cargar módulo kernel (sysinfo.ko)<br/>2. Compilar e inicializar sonda eBPF (bpf_kill.o)<br/>3. Levantar contenedores Valkey y Grafana (docker compose)<br/>4. Programar Cronjob (*/2 * * * *)<br/>5. Generar carga inicial (2 altos, 3 bajos)"]
    
    InitServices --> Loop["Inicio de Loop Principal (cada 20 segundos)"]
    
    Loop --> ReadProc["Leer /proc/continfo_pr2_so1_202201117"]
    ReadProc --> ParseJSON["Deserializar JSON de métricas y procesos"]
    
    ParseJSON --> Purge["PurgeInactive(activePIDs):<br/>Remover de las colas procesos muertos externamente"]
    
    Purge --> FilterGrafana["Filtrar procesos del sistema y excluir 'grafana' y 'valkey-server'"]
    
    FilterGrafana --> ForEach["Iterar cada contenedor reportado"]
    
    ForEach --> CheckHigh{"¿Es de Alto Consumo?<br/>(client, sh, CPU > 10% o RAM > 5%)"}
    
    %% Rama Alto Consumo
    CheckHigh -->|"Sí"| HighContains{"¿Ya está en cola highUsage?"}
    HighContains -->|"No"| HighEnqueue["highUsage.Enqueue(container)"]
    HighEnqueue --> HighFull{"¿Cola llena? (size == 2)"}
    HighFull -->|"Sí"| HighDequeue["highUsage.Dequeue():<br/>syscall.Kill(PID_cabeza, SIGKILL)"]
    HighFull -->|"No"| PushHigh["Insertar en cola highUsage"]
    HighDequeue --> PushHigh
    HighContains -->|"Sí"| NextProc
    
    %% Rama Bajo Consumo
    CheckHigh -->|"No"| LowContains{"¿Ya está en cola lowUsage?"}
    LowContains -->|"No"| LowEnqueue["lowUsage.Enqueue(container)"]
    LowEnqueue --> LowFull{"¿Cola llena? (size == 3)"}
    LowFull -->|"Sí"| LowDequeue["lowUsage.Dequeue():<br/>syscall.Kill(PID_cabeza, SIGKILL)"]
    LowFull -->|"No"| PushLow["Insertar en cola lowUsage"]
    LowDequeue --> PushLow
    LowContains -->|"Sí"| NextProc
    
    PushHigh --> NextProc{"¿Hay más procesos?"}
    PushLow --> NextProc
    
    NextProc -->|"Sí"| ForEach
    NextProc -->|"No"| PushValkey["Enviar telemetría a Valkey:<br/>• Total, Free, Used RAM (Keys)<br/>• stream:ram_history (Stream)<br/>• ranking:ram y ranking:cpu (ZSets)"]
    
    PushValkey --> Sleep["time.Sleep(20 segundos)"]
    Sleep --> Loop

    %% eBPF Asíncrono
    SubKill["syscall.Kill ejecutado"] -.-> TrapKill["eBPF intercepta sys_enter_kill"]
    TrapKill --> CheckSig{"¿sig == 9 o 15?"}
    CheckSig -->|"Sí"| EmitRing["Enviar kill_event a BPF RingBuffer"]
    EmitRing --> GoReader["EBPFLoader procesa evento en Go"]
    GoReader --> IncrValkey["Registrar confirmación real en Valkey:<br/>• Incr ebpf:total_events<br/>• Incr system:total_containers_deleted<br/>• XAdd stream:deleted_containers"]
```

---

## 3. MANUAL TÉCNICO (DETALLES DE IMPLEMENTACIÓN)

### 3.1 Módulo de Kernel en C (`sysinfo.c`)
* **Estructura base:** Itera el árbol de procesos con `for_each_process(task)`.
* **Aislamiento de contenedores:** Filtra instancias donde `task->comm` coincide con `containerd-shim` y navega hacia `task->children` para obtener el proceso hijo real (contenedor).
* **Métricas de memoria:** Extrae datos mediante `si_meminfo(&si)` (`totalram`, `freeram`) y del `mm_struct` (`total_vm` para VSZ, `get_mm_rss` para RSS).
* **Métricas de CPU:** Calcula el tiempo acumulado `utime + stime` en relación con los `jiffies` globales y el número de núcleos en línea (`num_online_cpus()`).

### 3.2 Sonda eBPF (`ebpf.c` y `loader.go`)
* **Punto de enganche:** Tracepoint estático del kernel `tracepoint/syscalls/sys_enter_kill`.
* **Filtrado in-kernel:** Descarta cualquier señal que no sea `SIGKILL` (9) o `SIGTERM` (15).
* **RingBuffer circular:** Buffer de 64 KB de memoria bloqueada que envía estructuras `kill_event` (`pid_emisor`, `pid_receptor`, `sig`) a espacio de usuario sin copias redundantes.
* **Cliente en Go:** Usa la librería `github.com/cilium/ebpf` para el acoplamiento y despacho reactivo del callback.

### 3.3 Daemon Gestor en Go y Manejo FIFO (`main.go` y `queue.go`)
* **Gestión FIFO:** Estructura de lista enlazada simple con apuntadores `head`, `tail` y contador `size`.
* **Capacidades máximas fijas:** Cola de alto consumo limitada a 2 elementos; cola de bajo consumo limitada a 3 elementos.
* **Purga de procesos zombis (`PurgeInactive`):** Recorre las colas para desvincular PIDs que hayan terminado su ciclo de vida naturalmente antes de ser expulsados por la FIFO.
* **Persistencia en Valkey:** 
  - `system:total_ram`, `system:used_ram`, `system:free_ram` (valores escalares).
  - `stream:ram_history` y `stream:deleted_containers` (series de tiempo Redis Streams).
  - `ranking:ram` y `ranking:cpu` (Sorted Sets clasificados por porcentaje de consumo).

---

## 4. MANUAL DE USUARIO Y GUÍA DE INSTALACIÓN

### Paso 1. PRERREQUISITOS DEL SISTEMA
Instalar dependencias necesarias para compilar código de kernel, objetos BPF y ejecutar Docker:

```bash
# En Arch Linux
sudo pacman -S base-devel linux-headers clang llvm go docker docker-compose

# Habilitar y arrancar Docker
sudo systemctl enable --now docker
```

### Paso 2. DESPLIEGUE Y EJECUCIÓN DEL SERVICIO
Todo el flujo de compilación, carga del módulo, levantamiento de contenedores y arranque del daemon está automatizado:

```bash
cd Proyecto2/src/daemon
sudo ./run.sh
```

### Paso 3. ACCESO AL DASHBOARD DE GRAFANA
1. Abrir el navegador en: `http://localhost:3001`
2. Credenciales por defecto:
   - **Usuario:** `admin`
   - **Contraseña:** `12345`
3. Navegar a **Dashboards** > **Daemon** para visualizar las métricas en tiempo real.

### Paso 4. DETENCIÓN LIMPIA DEL SISTEMA
Para detener el daemon de forma segura sin dejar residuos en el sistema:
1. Presionar `Ctrl + C` en la terminal del daemon.
2. El manejador de señales ejecutará automáticamente:
   - Cierre de la sonda eBPF.
   - Detención de los contenedores activos en las colas.
   - Eliminación del cronjob (`crontab`).
   - Descarga del módulo de kernel (`rmmod sysinfo`).
   - Limpieza de contenedores huérfanos (`docker container prune -f`).

---

## 5. VERIFICACIÓN Y PRUEBAS OPERATIVAS

### 1. Verificación del Archivo en `/proc`
```bash
cat /proc/continfo_pr2_so1_202201117
```

### 2. Verificación de Contenedores en Ejecución
```bash
docker ps
```

### 3. Inspección de Registros en Valkey
```bash
# Conectarse a la base de datos Valkey
docker exec -it valkey-db valkey-cli

# Consultar métricas escalares
GET system:total_ram
GET system:used_ram
GET ebpf:total_events

# Consultar stream de contenedores eliminados
XRANGE stream:deleted_containers - +
```

---

## 6. EVIDENCIAS Y CAPTURAS DE PANTALLA

### 6.1 Módulo de Kernel y Salida JSON en `/proc`
![](imgs/proc.png)

### 6.2 Logs del Daemon y Confirmación eBPF
![](imgs/daemon.png)

### 6.3 Dashboard Completo en Grafana
![](imgs/grafana.png)
