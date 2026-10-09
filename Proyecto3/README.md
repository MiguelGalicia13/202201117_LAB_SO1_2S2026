# Proyecto 3

## Creacion de clusters
Para la creacion de clusters se creara por medio de una CLI, para esto sera necesario la instalación de el SDK de Google Cloud Platform, una vez descarga y e inicializada los el cluster se creara con el siguiente comando 
```bash
gcloud container clusters create cluster-p3-202201117 --num-nodes 2 --machine-type n1-standard-2 --zone us-central1-a
```

Esto lo que hara es crear un cluster con 2 nodos de maquina tipo n1-standard-2 en la zona us-central1-a con el nombre cluster-p3-202201117. Las especificaciones en cuanto a los recursos de cada nodo son las siguientes: Por cada maquina se asignaran 2 vCPUs y 7.5GB de RAM. Por lo tanto, el cluster tendra 2 nodos con 4 vCPUs y 15GB de RAM.

Para manejar el cluster se utilizara la CLI de Kubernetes, la cual se instala con el siguiente comando:
```bash
gcloud components install kubectl
```
e instalamos el plugin de gcloud, esto nos permitira autenticarnos con el cluster de Kubernetes de Google Cloud Platform.
```bash
gcloud components install gke-gcloud-auth-plugin
```
Una vez con estos componentes instalados, podremos ver nuestro cluster de Kubernetes con el comando `kubectl get nodes`.
![](imgs/get-nodes.png)

Actualizaremos el cluster para habilitar la Network Policy
```bash
gcloud container clusters update cluster-p3-202201117 --update-addons=NetworkPolicy=ENABLED
```
y posteriormente lo habilitaremos
```bash
gcloud container clusters update cluster-p3-202201117 --enable-network-policy
```

## Creacion de la VM para zot

Desde la consola de Google Cloud Platform, crearemos una VM con las siguientes especificaciones:
 - Nombre: zot-registry
 - Tipo de maquina: n1-standard-1
 - Zona: us-central1-a
 - Modelo de aprovisionamiento: Spot
 - Disco: 15GB
 - Imagen: Debian 12
 - Sin proteccion de datos
 - Permitir trafico HTTP y HTTPS
Para podenos conectarnos a la VM, ingresamos este comando:
```bash
gcloud compute ssh --zone "us-central1-a" "zot-registry" --project "sopes1-510723"
```

Una vez en la VM, actualizaremos el sistema operativo con los siguientes comandos:
```bash
sudo apt update
sudo apt upgrade -y
```
E instalaremos la ultima version de zot con el siguiente comando y le asignaremos permisos de ejecucion:
```bash
sudo curl -Lo /usr/local/bin/zot https://github.com/project-zot/zot/releases/download/v2.1.21/zot-linux-amd64
sudo chmod +x /usr/local/bin/zot
```
Despues creamos su archivo de configuracion:
```json
{
        "distSpecVersion": "1.1.0",
        "storage":{
                "rootDirectory": "/var/lib/zot"
        },
        "http":{
                "address":"127.0.0.1",
                "port": "5000"
        },
        "log":{
                "level": "info"
        }
}
```
### Servicio de Zot
Creamos un servicio de zot en `/etc/systemd/system/zot.service`:
```
[Unit]
Description=Zot OCI Registry
After=network.target

[Service]
User=zot
Group=zot
Type=simple
ExecStart=/usr/local/bin/zot serve /etc/zot/config.json
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```
Luego con `sudo systemctl daemon-reload` y `sudo systemctl start zot`, el servicio de zot se inicia, y podemos verificar su estado con `sudo systemctl status zot`.

Para el siguiente paso, descargamos nginx para proteger el acceso a zot.
```bash
sudo apt install -y nginx
```
Para resolver el DNS y acceder a Zot desde fuera de la VM, usaremos el dominio de `sslip.io`. Configuraremos Nginx como proxy inverso hacia Zot, que escucha localmente en el puerto 5000. El siguiente bloque permite subir capas de Docker de cualquier tamaño:

```nginx
server {
    listen 80;
    server_name 34.44.185.177.sslip.io;

    # Permite subir capas grandes sin que Nginx responda con un error 413.
    client_max_body_size 0;
    chunked_transfer_encoding on;

    location / {
        proxy_pass http://127.0.0.1:5000;
        proxy_http_version 1.1;
        proxy_request_buffering off;

        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        proxy_read_timeout 900s;
    }
}
```


Guarda esta configuración en `/etc/nginx/sites-available/zot`, habilítala y valida la configuración antes de recargar Nginx:

```bash
sudo rm -f /etc/nginx/sites-enabled/default
sudo ln -sf /etc/nginx/sites-available/zot /etc/nginx/sites-enabled/
sudo nginx -t && sudo systemctl reload nginx  
```


La configuración inicial escucha en HTTP (puerto 80). Para habilitar HTTPS, instala Certbot y su complemento para Nginx:

```bash
sudo apt install -y certbot python3-certbot-nginx
```

Luego solicita e instala el certificado TLS para el dominio:

```bash
sudo certbot --nginx -d 34.44.185.177.sslip.io --non-interactive --agree-tos -m 3776080760101@ingenieria.usac.edu.gt
```

Certbot valida el dominio, obtiene un certificado de Let's Encrypt y modifica la configuración de Nginx para habilitar HTTPS. Las opciones `--non-interactive` y `--agree-tos` permiten ejecutarlo sin preguntas, y `-m` registra el correo para avisos importantes y de renovación. Para que la validación funcione, el dominio debe apuntar a la IP pública de la VM y el tráfico entrante por los puertos 80 y 443 debe estar permitido.

## Creación del servidor web en Rust

Para la creación del servidor web en Rust se realizaron los siguientes pasos utilizando Cargo:

1. **Inicializar el proyecto con Cargo**:
   Se inicializó el proyecto con el siguiente comando:
   ```bash
   cargo new api-rust
   ```
   *(o ejecutando `cargo init` dentro del directorio correspondiente)* y luego se ingresó a la carpeta:
   ```bash
   cd api-rust
   ```

2. **Añadir Actix Web con cargo add**:
   Se agregó la dependencia del framework web `actix-web` ejecutando:
   ```bash
   cargo add actix-web
   ```

Con esto, `actix-web` queda registrado en el archivo `Cargo.toml` y disponible para implementar el servidor web en `src/main.rs`.

### Configuración y funcionamiento de la API

La API escucha en `0.0.0.0:8080` y cuenta con los siguientes endpoints:

- `GET /health`: comprueba que la API está operativa y responde un JSON con el estado y un mensaje.
- `POST /grpc-202201117`: recibe un reporte en JSON, lo valida y, si es válido, lo reenvía al servicio Go.

El cuerpo esperado para `POST /grpc-202201117` tiene esta estructura:

```json
{
  "country": "ESP",
  "warplanes_in_air": 0,
  "warships_in_water": 0,
  "timestamp": "2026-10-06T20:15:30Z"
}
```

La validación comprueba que `country` sea uno de `USA`, `RUS`, `CHN`, `ESP` o `GTM`; que las cantidades de aviones y barcos sean enteros no negativos; y que `timestamp` sea una fecha RFC 3339 en UTC, terminada en `Z` o `+00:00`. Si el reporte no es válido, responde `400 Bad Request`.

La URL del servicio Go receptor se configura mediante la variable de entorno `GO_SERVICE_URL`. Como la API se ejecutará dentro de Kubernetes, define esta variable en el contenedor de su `Deployment`, apuntando al `Service` interno del servicio Go. Por ejemplo, si ambos servicios están en el mismo namespace:

```yaml
spec:
  template:
    spec:
      containers:
        - name: api-rust
          env:
            - name: GO_SERVICE_URL
              value: "http://<nombre-del-service-go>:<puerto>/<ruta>"
```

Sustituye los valores entre `< >` por el nombre del `Service` de Go, el puerto que expone y la ruta del endpoint receptor. Si los servicios están en namespaces distintos, usa el DNS completo del servicio: `http://<service-go>.<namespace>.svc.cluster.local:<puerto>/<ruta>`. Kubernetes inyectará la variable al iniciar el pod; no es necesario ejecutar `export` manualmente. La API usa un cliente HTTP de `reqwest` con un tiempo límite de cinco segundos para enviar el reporte. Si el servicio Go responde con éxito, la API responde `200 OK`; si devuelve un estado de error, responde `502 Bad Gateway`. Si falla la solicitud HTTP, la API responde `400` con el detalle del error.

El cliente HTTP y la URL se comparten con los handlers mediante `web::Data`. Además de `actix-web`, el proyecto utiliza `reqwest` para enviar solicitudes, `serde` para serializar y deserializar JSON, y `time` para validar las fechas.

### Construcción y publicación de la imagen Docker

El `Dockerfile` de `api-rust` compila la aplicación en modo release y crea una imagen de ejecución basada en Debian, que expone el puerto `8080`. Para construir la imagen y publicarla en el registry Zot disponible en `34.170.72.85.sslip.io`, se ejecutaron los siguientes comandos desde el directorio `Proyecto3/api-rust`:

```bash
docker build -t 34.170.72.85.sslip.io/api-rust:v1 .
docker push 34.170.72.85.sslip.io/api-rust:v1
```

El primer comando construye la imagen usando el directorio actual como contexto y la etiqueta como `api-rust:v1` en el registry. El segundo sube esa imagen al registry para que pueda ser utilizada en el despliegue de Kubernetes.

## Servicios en Go

### Construcción y publicación de la imagen Docker de `go-deployment-1`

El `Dockerfile` de `go-deployment-1` compila los binarios del receptor REST y del cliente gRPC, y los copia a una imagen de ejecución basada en Alpine. La imagen expone los puertos `8080` y `8081`; por defecto, inicia el receptor REST. Desde el directorio `Proyecto3`, se construyó y publicó con estos comandos:

```bash
docker build -t 34.170.72.85.sslip.io/go-deployment-1:v1 -f go-deployment-1/Dockerfile .
docker push 34.170.72.85.sslip.io/go-deployment-1:v1
```

En `docker push` solo se indica la referencia de la imagen; las opciones `-f` y el contexto `.` corresponden al comando `docker build`.

### Modelo y validación de reportes

El paquete `internals/report` contiene el tipo compartido `Report`, su validación y la función `Decode`, utilizada por el receptor HTTP. El JSON de entrada debe incluir `country`, `warplanes_in_air`, `warships_in_water` y `timestamp`.

La validación acepta los países `USA`, `RUS`, `CHN`, `ESP` y `GTM`; requiere que ambos contadores estén presentes, sean valores `int32` no negativos; y exige un timestamp RFC 3339 en UTC, terminado en `Z` o `+00:00`. `Decode` limita el cuerpo a 1 MiB, rechaza campos JSON desconocidos y permite un solo documento JSON.

### Receptor REST

El servidor `go-deployment-1/rest-receiver` escucha en el puerto `8080` y expone:

- `GET /health`: responde con estado HTTP `200 OK`.
- `POST /report`: decodifica y valida el reporte. Si es válido, lo serializa como JSON y lo reenvía mediante HTTP `POST` a `http://127.0.0.1:8081/send`, con `Content-Type: application/json`.

El cliente HTTP hacia `/send` tiene un timeout de cuatro segundos. Los errores al validar el reporte responden `400 Bad Request`; los errores de serialización o creación de la solicitud responden `500 Internal Server Error`; y los errores de conexión o las respuestas no exitosas del destino se convierten en `502 Bad Gateway`.

El receptor REST reenvía las solicitudes a `127.0.0.1:8081/send`; por tanto, el receptor y el cliente gRPC deben compartir el mismo pod y su red local. No se encontraron manifiestos de Kubernetes para confirmar cómo están agrupados los contenedores.

Cuando `/send` responde con éxito, el receptor REST copia el código HTTP, el `Content-Type` y el cuerpo de respuesta del cliente gRPC hacia quien llamó a `/report`.

### Cliente gRPC

El programa `go-deployment-1/grpc-client` cumple dos funciones: expone un puente HTTP local en `127.0.0.1:8081` y actúa como cliente del servicio gRPC `WarReportService`.

Expone los siguientes endpoints:

- `GET /health`: responde `200 OK`.
- `POST /send`: decodifica y valida el JSON del reporte, lo convierte al mensaje Protobuf `WarReportRequest` y llama al método `SendReport`. Si la llamada tiene éxito, responde con el resultado en JSON; si falla, responde `502 Bad Gateway`.

El contrato de la llamada se define en `proto/wartweets.proto`: `SendReport` recibe `WarReportRequest` —país, cantidades y timestamp— y devuelve `WarReportResponse`.

El destino gRPC se configura con `GRPC_SERVER_ADDR`. Si la variable no está definida, se usa `dns:///go-grpc-service:50051`, que espera un Service de Kubernetes con el nombre `go-grpc-service` y puerto `50051`. Para configurar el destino explícitamente en el `Deployment`:

```yaml
env:
  - name: GRPC_SERVER_ADDR
    value: "dns:///go-grpc-service:50051"
```

El cliente utiliza el balanceador `round_robin`, credenciales de transporte inseguras (sin TLS) y un timeout de tres segundos para cada llamada gRPC. El valor `dns:///go-grpc-service:50051` debe resolverse desde el pod; las credenciales sin TLS son apropiadas solo si la comunicación ocurre en una red interna confiable.

Al recibir `SIGTERM` o `SIGINT`, el servidor deja de aceptar solicitudes nuevas y espera hasta cinco segundos para que terminen las solicitudes HTTP en curso antes de cerrar la conexión gRPC. Al desplegarlo en Kubernetes, configura `terminationGracePeriodSeconds` con un valor superior a cinco segundos para permitir que el apagado gradual finalice.

El flujo implementado es: API Rust → `POST /report` del receptor REST → `POST /send` local → RPC `WarReportService.SendReport` en el Service gRPC.

### Go Deployment 2: servidor gRPC y writer RabbitMQ

El servidor gRPC existente reenvía cada reporte a `http://127.0.0.1:8081/publish`.
El nuevo `go-deployment-2/rabbit-writer` valida el JSON con `internals/report`,
publica en RabbitMQ y devuelve `200` únicamente después del ACK del broker.
Un reporte inválido devuelve `400`; una publicación no confirmada devuelve `503`.
El servidor gRPC convierte ese fallo en `Unavailable` y solo devuelve
`REPORT_PUBLISHED_TO_RABBITMQ` cuando el writer tuvo éxito.

El writer escucha exclusivamente en `127.0.0.1:8081` y expone:

- `POST /publish`: publicación con un plazo de 1.5 segundos, inferior al timeout
  de 2 segundos usado por el servidor gRPC.
- `GET /health`: confirma que el proceso HTTP está activo.
- `GET /ready`: devuelve `200` cuando la conexión y el canal están disponibles;
  devuelve `503` durante una caída o reconexión.

`RABBITMQ_URL` es obligatoria y contiene la URL AMQP con las credenciales.
`RABBITMQ_QUEUE` es opcional y utiliza `military_reports` como valor predeterminado.
En GKE, configura las credenciales mediante un Secret y usa un usuario dedicado,
sin guardar contraseñas reales en Git ni utilizar `guest` entre Pods.

La conexión y el canal se reutilizan; un mutex serializa las publicaciones.
La cola es durable y los mensajes usan `DeliveryMode: Persistent` y
`ContentType: application/json`. Se publica al exchange predeterminado con el
nombre de la cola y `mandatory=true`, rechazando también mensajes sin ruta.
Ante una caída se recrean la conexión, el canal, la cola y las confirmaciones,
con esperas de 1, 2, 4, 8 y hasta 10 segundos entre intentos fallidos.

Un timeout de confirmación puede ocurrir después de que RabbitMQ haya recibido
el reporte. El writer no lo reenvía automáticamente para evitar duplicados;
no existe una garantía de entrega exactamente una vez.

#### Prueba local

Los siguientes valores de usuario y contraseña son únicamente para pruebas.
Inicia un broker local desde `Proyecto3/`:

```bash
docker run -d --name p3-rabbitmq \
  --publish 127.0.0.1:5672:5672 \
  -e RABBITMQ_DEFAULT_USER=reports -e RABBITMQ_DEFAULT_PASS=local-test \
  rabbitmq:4.3.6-alpine
docker exec p3-rabbitmq rabbitmq-diagnostics -q ping
```

Espera a que RabbitMQ esté disponible antes de ejecutar las pruebas de integración:

```bash
RABBITMQ_TEST_URL=amqp://reports:local-test@127.0.0.1:5672/ go test -race ./...
```

Sin `RABBITMQ_TEST_URL`, las pruebas de validación y del contrato gRPC se ejecutan,
y la prueba de integración con el broker se omite. Esta última utiliza una cola
temporal propia y comprueba publicación persistente, concurrencia, reconexión y
rechazo de mensajes sin ruta; elimina esa cola al terminar.

Para ejecutar el writer y el servidor gRPC, usa dos terminales:

```bash
RABBITMQ_URL=amqp://reports:local-test@127.0.0.1:5672/ \
  go run ./go-deployment-2/rabbit-writer
```

```bash
go run ./go-deployment-2/grpc-server
```

Comprueba el writer en otra terminal:

```bash
curl --fail http://127.0.0.1:8081/ready
curl --fail -X POST http://127.0.0.1:8081/publish \
  -H 'Content-Type: application/json' \
  -d '{"country":"ESP","warplanes_in_air":42,"warships_in_water":14,"timestamp":"2026-10-08T12:00:00Z"}'
```

Go Deployment 1 también utiliza `127.0.0.1:8081` para su cliente gRPC. Al probar
ambos deployments simultáneamente, deben tener espacios de red separados, como
los Pods previstos; no ejecutes ambos componentes locales en ese mismo puerto
del host.

#### Imagen de Go Deployment 2

Construye desde `Proyecto3/`, usando ese directorio como contexto:

```bash
docker build -f go-deployment-2/Dockerfile -t go-deployment-2:local .
```

La imagen incluye `/app/grpc-server` y `/app/rabbit-writer` y ejecuta el servidor
gRPC por defecto. Ambos procesos corren con usuario sin privilegios. Para probar
los dos contenedores con un loopback compartido y el broker local anterior:

```bash
docker run -d --name p3-grpc-server --network container:p3-rabbitmq go-deployment-2:local
docker run -d --name p3-rabbit-writer --network container:p3-rabbitmq \
  -e RABBITMQ_URL=amqp://reports:local-test@127.0.0.1:5672/ \
  --entrypoint /app/rabbit-writer go-deployment-2:local
docker exec p3-rabbit-writer wget -qO- http://127.0.0.1:8081/ready
```

Usa este ejemplo Docker después de detener los procesos de `go run`.
Esta prueba comparte también la red del broker para facilitar la conexión local;
en Kubernetes solo `grpc-server` y `rabbit-writer` compartirán el Pod.
Los manifiestos y la publicación de la imagen en Zot quedan pendientes.

Referencias: [cliente AMQP para Go](https://pkg.go.dev/github.com/rabbitmq/amqp091-go)
y [publisher confirms de RabbitMQ](https://www.rabbitmq.com/docs/confirms).

## Generación de tráfico con Locust

El cliente `locust/locustfile.py` envía reportes mediante `POST /grpc-202201117`.
Cada solicitud genera un país aleatorio entre `USA`, `RUS`, `CHN`, `ESP` y `GTM`,
entre 0 y 50 aviones, entre 0 y 30 barcos y un timestamp actual en UTC terminado en `Z`.
La asignación `ESP` del carnet corresponde al dashboard; se generan los cinco países
para alimentar también los rankings generales.

Desde `Proyecto3/`, con `uv` instalado y Python 3.11 o posterior:

```bash
uv sync --project locust --locked
uv run --project locust --locked python -m unittest discover -s locust -p 'test_*.py'
```

`uv` administra el entorno `locust/.venv` y las dependencias de `pyproject.toml`.
El archivo `locust/uv.lock` fija también las dependencias transitivas y debe
versionarse. Usa `uv add --project locust <paquete>` para agregar dependencias.

Para usar la interfaz web con una API Rust local:

```bash
uv run --project locust --locked locust -f locust/locustfile.py --host http://127.0.0.1:8080
```

Abre `http://localhost:8089` y configura los usuarios concurrentes y la cantidad de
usuarios nuevos por segundo. Cada usuario espera entre 1 y 2 segundos después de
enviar un reporte; la tasa de creación de usuarios no es la cantidad de requests/s.
El timeout de cada solicitud es de 10 segundos. Locust registra las respuestas HTTP
de error y los fallos de conexión como solicitudes fallidas.

Para una corrida sin interfaz, con 50 usuarios, incorporación de 5 usuarios/s y
duración total de 2 minutos, incluyendo el aumento inicial de usuarios:

```bash
mkdir -p locust/results
uv run --project locust --locked locust -f locust/locustfile.py \
  --host http://127.0.0.1:8080 \
  --headless --users 50 --spawn-rate 5 --run-time 2m --stop-timeout 10 \
  --csv locust/results/baseline --csv-full-history
```

Para probar el flujo desplegado, sustituye `--host` por la URL pública del Gateway,
sin añadir `/grpc-202201117`: el cliente agrega esa ruta. La ejecución requiere que
Rust y sus dependencias estén disponibles; el cliente no inicia los servicios.
Los CSV guardan estadísticas, evolución temporal, errores y excepciones; pueden
utilizarse como evidencia de throughput, latencia y porcentaje de fallos. Conserva
los mismos parámetros de carga y usa prefijos distintos al comparar réplicas.
Los resultados y el entorno virtual quedan excluidos de Git.

Referencias: [gestión de proyectos con uv](https://docs.astral.sh/uv/guides/projects/)
y [ejecución sin interfaz de Locust](https://docs.locust.io/en/stable/running-without-web-ui.html).

## Kubernetes

### Namespace
Aplicacion de namespace
```bash
kubectl apply -f k8s/01-namespace.yaml
```
### ConfigMap

Para probar que el ConfigMap se aplica correctamente probamos ejectutarlo con
```bash
kubectl apply --dry-run=server -f k8s/02-configmap.yaml
```
Y si recibe un mensaje de confirmación, el ConfigMap se aplica correctamente. Por lo cual podemos aplicarlo con
```bash
kubectl apply -f k8s/02-configmap.yaml
```
### Secrets

Para probar que el Secret se aplica correctamente probamos ejectutarlo con
```bash
kubectl apply --dry-run=server -f k8s/03-secret.yaml
```
Y si recibe un mensaje de confirmación, el Secret se aplica correctamente. Por lo cual podemos aplicarlo con
```bash
kubectl apply -f k8s/03-secret.yaml
```
Para verificar nuestras configuraciones fueron aplicadas correctamente usaremos los comandos
```bash
kubectl get namespace proyecto3
kubectl get configmaps -n proyecto3
kubectl get secrets -n proyecto3
```
![](imgs/global-configs.png)

### Servicios

#### RabbitMQ

Para rabbitmq se crearon 2 servicios: rabbitmq y rabbitmq-headless. Rabbitmq es el servicio principal que expone el puerto 5672, mientras que rabbitmq-headless es un servicio sin IP propia que se utiliza para la comunicación interna entre los pods.

```bash
kubectl get services -n proyecto3
```
![](imgs/rabbitmq-services.png)

### StatefulSet
Este despliega una réplica de RabbitMQ en el namespace usando la imagen
de rabbitmq que esta en zot y exponiendo el puerto AMQP 5672. Al ser un `StatefulSet`, el pod conserva una identidad estable y obtiene un volumen persistente de 5 GiB mediante `volumeClaimTemplates`, montado en `/var/lib/rabbitmq` para conservar los datos del broker al reemplazar el pod.

El usuario y la contraseña iniciales se leen del Secret `rabbitmq-credentials`. El contenedor solicita `100m` de CPU y `256Mi` de memoria, con límites de `1` CPU y `1Gi` de memoria. La startup probecomprueba `rabbitmq-diagnostics ping` y la readiness probe espera a que el broker esté en ejecución antes de marcarlo listo.

El `StatefulSet` referencia mediante `serviceName: rabbitmq-headless` al Service headless ya creado, que proporciona la identidad de red estable del pod. El StatefulSet no crea Services: el Service `rabbitmq`, que expone el puerto `5672` para que el writer se conecte por AMQP, también está definido por separado en el manifiesto de Services. Configura `RABBITMQ_URL` con las credenciales del Secret y la dirección de ese Service.

![](imgs/rabbitmq-statefulset.png)

# Recursos

https://www.youtube.com/watch?v=7s5X7GoYj8k

