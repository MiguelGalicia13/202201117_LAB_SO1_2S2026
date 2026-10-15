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
kubectl apply -f k8s/01-namcespace.yml
```
### ConfigMap

Para probar que el ConfigMap se aplica correctamente probamos ejectutarlo con
```bash
kubectl apply --dry-run=server -f k8s/02-configmap.yml
```
Si la validación termina correctamente, el archivo es aceptado por el servidor, pero todavía no se aplica. Para aplicarlo ejecutamos
```bash
kubectl apply -f k8s/02-configmap.yml
```
### Secrets

Para probar que el Secret se aplica correctamente probamos ejectutarlo con
```bash
kubectl apply --dry-run=server -f k8s/03-secrets.yml
```
Si la validación termina correctamente, el archivo es aceptado por el servidor, pero todavía no se aplica. Para aplicarlo ejecutamos
```bash
kubectl apply -f k8s/03-secrets.yml
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
#### gRPC Server

El servicio de gRPC server permite el acceso interno al servidor gRPC mediante el puerto 50051. El servidor se comunica con el writer del mismo Pod por la ruta `127.0.0.1:8081/publish`. El writer publica los mensajes en RabbitMQ mediante el Service `rabbitmq:5672`.

Despues de aplicar su configuracion, verficamos que el servicio exista y consultamos sus destinos:

```bash
kubectl get service go-grpc-server -n proyecto3
kubectl get endpointslices -n proyecto3 -l kubernetes.io/service-name=go-grpc-server
```

![](imgs/grpc-service-server.png)

#### Go-Rest Service

El servicio de go-rest-service es de tipo `ClusterIP` y expone el puerto 8080

Se aplica con el comando 

```bash
kubectl apply -f go-deployment-1/k8s/01-service.yml
```
#### api-rust-service

El servicio de api-rust-service es de tipo `ClusterIP` y expone el puerto 80 para redirigir a 8080.

Se aplica con el comando 

```bash
kubectl apply -f api-rust/k8s/01-service.yml
```

### StatefulSet
Este despliega una réplica de RabbitMQ en el namespace usando la imagen
de rabbitmq que esta en zot y exponiendo el puerto AMQP 5672. Al ser un `StatefulSet`, el pod conserva una identidad estable y obtiene un volumen persistente de 5 GiB mediante `volumeClaimTemplates`, montado en `/var/lib/rabbitmq` para conservar los datos del broker al reemplazar el pod.

El usuario y la contraseña iniciales se leen del Secret `rabbitmq-credentials`. El contenedor solicita `100m` de CPU y `256Mi` de memoria, con límites de `1` CPU y `1Gi` de memoria. La startup probecomprueba `rabbitmq-diagnostics ping` y la readiness probe espera a que el broker esté en ejecución antes de marcarlo listo.

El `StatefulSet` referencia mediante `serviceName: rabbitmq-headless` al Service headless ya creado, que proporciona la identidad de red estable del pod. El StatefulSet no crea Services: el Service `rabbitmq`, que expone el puerto `5672` para que el writer se conecte por AMQP, también está definido por separado en el manifiesto de Services. Configura `RABBITMQ_URL` con las credenciales del Secret y la dirección de ese Service.

![](imgs/rabbitmq-statefulset.png)

### Deployments

#### Deployment 1
El manifiesto `go-deployment-1/k8s/02-deployment.yml` crea el Deployment llamado `go-deployment-1` que cuenta con 2 contenedores, ambos usan la imagen de go-deployment-1 en su v1.

El primer contenedor lleva como nombre rest-receiver y como su nombre lo indica recibe las peticiones REST, también expone el puerto 8080 para recibir las peticiones. Aparte de asignarle recursos, para comprobar su estado como `readiness` y `liveness` se realiza un get al endpoint `/health`.

El segundo contenedor lleva como nombre grpc-client, su funcionalidad es recibir las peticiones que llegan por medio de la API REST y las procesa hacria el servidor GRPC. Por eso mismo en su configuración de `env` se hace referencia a la varible GRPC_SERVER_ADDR que contiene la dirección del servidor GRPC, este es tomado del configmap.

De igual manera este se aplica con el comando
```bash
kubectl apply -f go-deployment-1/k8s/02-deployment.yml
```
Para verificar que el deployment se haya creado correctamente se puede ejecutar el comando
```bash
kubectl rollout status deployment/go-deployment-1 -n proyecto3
```




#### Deployment 2

El manifiesto `go-deployment-2/k8s/deployment.yml` crea el Deployment llamado `go-deployment-2` que cuenta con 2 contenedores, ambos usan la imagen de go-deployment-2 en su v1.

El primer contenedor lleva como nombre grpc-server y como su nombre lo indica levanta el servidor gRPC en el puerto 50051. Del ConfigMap toma `RABBIT_WRITER_URL` para comunicarse con el writer. Su `readiness` comprueba el puerto mediante `tcpSocket`. De igual manera se le asignan recursos.

El segundo contenedor llamado rabbit-writer usa la imagen de go-deployment-2 e inicia el programa de rabbit-writer. Busca en el Secret la clave `amqp-url` y en el ConfigMap la clave `RABBITMQ_QUEUE`. Se le asignan recursos y por medio de la ruta `/ready` en el puerto 8081 se comprueba su `readiness`; para `liveness` se usa la ruta `/health`.

Este se aplica con el comando 
```bash
kubectl apply -f go-deployment-2/k8s/deployment.yml
```
Y para verificar su estado se ejecuta
```bash
kubectl rollout status deployment/go-deployment-2 -n proyecto3 --timeout=120s
```

Para observar el estado de los Pods en tiempo real, ejecuta el comando `kubectl get pods -n proyecto3 -w`.

#### Api-Rust

El deployment de api-rust se encuentra en la carpeta `api-rust/k8s/02-deployment.yml`. El deployment llamado `api-rust` maneja un solo contenedor que llama la imagen de `api-rust` desde el zot registry. Habilita el puerto http en el puerto 8080. Luego busca el servicio de go en el configmap. Su readiness y el liveness se verifican en la ruta `/health`

Para aplicar su configuración, ejecuta el comando 
```bash
kubectl apply -f api-rust/k8s/02-deployment.yml
```
Y para verificar su estado, ejecuta
```bash
kubectl rollout status deployment/api-rust -n proyecto3
```

Para ver los deployments creados se ejecuta el comando 
```bash
kubectl get deployments -n proyecto3
```

![](imgs/deployments.png)

### Preparación de Gateway API

Los manifiestos siguientes se aplicaron uno por uno después de habilitar Gateway API. Los comandos de esta sección permiten reproducir el despliegue desde la carpeta `Proyecto3`. Primero verificamos el contexto del clúster:

```bash
kubectl config current-context
kubectl get gatewayclass
kubectl api-resources --api-group=networking.gke.io
```

Gateway API requiere un clúster VPC-native y el complemento `HttpLoadBalancing` habilitado. Para consultar la configuración de este clúster ejecutamos:

```bash
gcloud container clusters describe cluster-p3-202201117 \
  --zone=us-central1-a --project=sopes1-510723 \
  --format='json(ipAllocationPolicy.useIpAliases,addonsConfig.httpLoadBalancing,networkConfig.gatewayApiConfig)'
```

Si todavía no aparecen GatewayClass ni HealthCheckPolicy, habilitamos Gateway API con el siguiente comando. Este comando actualiza GKE; crear los archivos YAML no lo ejecuta automáticamente.

```bash
gcloud container clusters update cluster-p3-202201117 \
  --zone=us-central1-a --project=sopes1-510723 \
  --gateway-api=standard
```

Si `addonsConfig.httpLoadBalancing.disabled` es `true`, también debemos habilitar ese complemento antes de continuar:

```bash
gcloud container clusters update cluster-p3-202201117 \
  --zone=us-central1-a --project=sopes1-510723 \
  --update-addons=HttpLoadBalancing=ENABLED
```

Después esperamos a que `kubectl get gatewayclass` muestre `gke-l7-global-external-managed` con `ACCEPTED=True` y que `kubectl api-resources --api-group=networking.gke.io` incluya `HealthCheckPolicy`. No se crea un GatewayClass propio ni se instalan CRDs ajenas al controlador de GKE.

Referencia: [habilitar y desplegar Gateway API en GKE](https://docs.cloud.google.com/kubernetes-engine/docs/how-to/deploying-gateways).

### HealthCheckPolicy de Rust

El manifiesto `api-rust/k8s/03-healthcheck-policy.yml` crea la política `rust-health` en el namespace `proyecto3`. Se asocia al Service `api-rust-service` y configura la comprobación del balanceador con HTTP en la ruta `/health` y el puerto 8080 del Pod. Se usa esa ruta porque la API Rust devuelve 200 en `/health`, mientras que no tiene un endpoint GET en `/`. Esta política configura la salud del backend del balanceador; las probes del Deployment siguen comprobando el estado del contenedor.

Una vez disponibles las APIs de GKE, validamos y aplicamos el manifiesto:

```bash
kubectl apply --dry-run=server -f api-rust/k8s/03-healthcheck-policy.yml
kubectl apply -f api-rust/k8s/03-healthcheck-policy.yml
```

Para consultar la política ejecutamos:

```bash
kubectl describe healthcheckpolicy rust-health -n proyecto3
```

El controlador asocia esta política al backend cuando el Service está referenciado por el HTTPRoute. El puerto 8080 corresponde al proceso Rust; el puerto de entrada del Service sigue siendo 80.

Referencia: [HealthCheckPolicy en GKE](https://docs.cloud.google.com/kubernetes-engine/docs/how-to/configure-gateway-resources).

### Gateway

El manifiesto `k8s/04-gateway.yml` crea `reports-gateway` en `proyecto3`. Usa la clase `gke-l7-global-external-managed` para que GKE cree el balanceador externo y su dirección pública. Define un listener llamado `http` en el puerto 80 y permite asociar rutas únicamente del mismo namespace. Esta configuración inicial utiliza HTTP; HTTPS requiere un certificado y la configuración de un listener TLS.

Para validar y aplicar ejecutamos:

```bash
kubectl apply --dry-run=server -f k8s/04-gateway.yml
kubectl apply -f k8s/04-gateway.yml
```

Para verificar su estado ejecutamos:

```bash
kubectl get gateway reports-gateway -n proyecto3
kubectl describe gateway reports-gateway -n proyecto3
```

Esperamos la condición `Programmed=True` y una dirección en `status.addresses`. El aprovisionamiento puede tardar varios minutos. Antes de asociar el HTTPRoute no existe una ruta hacia Rust. Aplicar este Gateway puede crear un balanceador facturable en GCP.

### HTTPRoute

El manifiesto `k8s/05-httproute.yml` crea la ruta `reports-route` en `proyecto3`. Su `parentRefs` apunta a `reports-gateway` y al listener `http`. La regla coincide exactamente con `/grpc-202201117` y envía las peticiones al Service `api-rust-service` por su puerto 80. No cambia la ruta, ya que Rust espera ese mismo endpoint. Aunque el path se llama grpc, la entrada pública usa HTTP y JSON; la comunicación gRPC ocurre entre los servicios Go.

Para validar y aplicar ejecutamos:

```bash
kubectl apply --dry-run=server -f k8s/05-httproute.yml
kubectl apply -f k8s/05-httproute.yml
```

Para verificar su asociación al Gateway y al Service ejecutamos:

```bash
kubectl describe httproute reports-route -n proyecto3
```

Esperamos `Accepted=True` y `ResolvedRefs=True` dentro del estado de su parent. La coincidencia `Exact` no incluye `/grpc-202201117/` ni `/health` en la entrada pública. Para revisar la salud de Rust antes del Gateway podemos usar port-forward al Service en una terminal y consultar `/health` en otra:

```bash
kubectl port-forward service/api-rust-service 8080:80 -n proyecto3
```

```bash
curl --fail http://127.0.0.1:8080/health
```

Después cerramos el port-forward con `Ctrl+C`.

### HPA de Rust

El manifiesto `api-rust/k8s/04-hpa.yml` crea `api-rust-hpa`. Su `scaleTargetRef` apunta al Deployment `api-rust` y mantiene entre 1 y 3 réplicas, con un objetivo de utilización media de CPU del 30 %. El porcentaje se calcula respecto al `requests.cpu` de cada Pod: con `100m` de request, el objetivo equivale aproximadamente a `30m` de CPU por Pod, no al 30 % de la CPU del nodo.

El YAML del Deployment Rust ya no fija `spec.replicas`, para que volver a aplicarlo no sobrescriba el número de réplicas que decida el HPA. Un Deployment nuevo sin ese campo inicia con una réplica; el HPA toma el control al aplicarse.

Antes de aplicar el HPA comprobamos que GKE entregue métricas de recursos:

```bash
kubectl api-resources --api-group=metrics.k8s.io
kubectl get --raw /apis/metrics.k8s.io
kubectl top pods -n proyecto3
```

Si la API no existe o las métricas no están disponibles, el HPA no podrá calcular la CPU. Primero hay que revisar el componente de métricas de GKE; aumentar manualmente las réplicas no verifica el autoscalado.

Para validar y aplicar ejecutamos:

```bash
kubectl apply --dry-run=server -f api-rust/k8s/04-hpa.yml
kubectl apply -f api-rust/k8s/04-hpa.yml
```

Para consultar sus métricas y observar el escalado ejecutamos:

```bash
kubectl get hpa api-rust-hpa -n proyecto3
kubectl describe hpa api-rust-hpa -n proyecto3
kubectl get hpa,pods -n proyecto3 -w
```

La utilización debe aparecer con un valor de CPU y no quedar como `<unknown>`. Crear el HPA no demuestra que haya escalado: hace falta generar carga y registrar las réplicas antes, durante y después de la prueba. Durante una prueba es normal que el HPA tarde en reducir las réplicas por su ventana de estabilización.

Referencia: [Horizontal Pod Autoscaling](https://kubernetes.io/docs/concepts/workloads/autoscaling/horizontal-pod-autoscale/).

### Validación del flujo desplegado

Los nuevos recursos se aplican en este orden: habilitación de Gateway API, HealthCheckPolicy, Gateway, HTTPRoute y HPA cuando las métricas estén disponibles. Los Deployments, Services y el broker de la fase anterior deben estar listos. La validación `--dry-run=server` acepta o rechaza el YAML sin desplegarlo; no comprueba tráfico ni escalado.

Para comprobar los recursos ejecutamos:

```bash
kubectl get deployments,pods,services,pvc,hpa -n proyecto3
kubectl get gateway,httproute,healthcheckpolicy -n proyecto3
kubectl get endpointslices -n proyecto3
```

Una vez el Gateway esté programado y el HTTPRoute aceptado, obtenemos su IP:

```bash
http://136.68.255.122/=$(kubectl get gateway reports-gateway -n proyecto3 -o jsonpath='{.status.addresses[0].value}')
```

Para enviar un reporte de prueba ejecutamos el siguiente comando. Esta operación debe crear un mensaje en RabbitMQ si todo el flujo funciona:

```bash
curl --fail-with-body -X POST "http://${http://136.68.255.122/}/grpc-202201117" \
  -H 'Content-Type: application/json' \
  --data "{\"country\":\"ESP\",\"warplanes_in_air\":1,\"warships_in_water\":1,\"timestamp\":\"$(date -u +%Y-%m-%dT%H:%M:%SZ)\"}"
```

Esperamos HTTP 200 y una respuesta de Rust con `status: success`. El estado `REPORT_PUBLISHED_TO_RABBITMQ` pertenece a la respuesta gRPC interna, que Rust no devuelve directamente. Para consultar la cola sin imprimir credenciales ejecutamos antes y después de la prueba:

```bash
kubectl exec rabbitmq-0 -n proyecto3 -- rabbitmqctl list_queues name messages_ready messages_unacknowledged consumers
```

Para generar la carga inicial usamos Locust con uv. Primero creamos la carpeta de resultados y usamos un prefijo diferente en cada ejecución:

```bash
mkdir -p locust/results
uv run --project locust --locked locust -f locust/locustfile.py \
  --host "http://${http://136.68.255.122/}" --headless --users 50 --spawn-rate 5 \
  --run-time 60s --stop-timeout 10 --csv locust/results/gke-baseline
```

Las solicitudes fallidas, latencias, mensajes en RabbitMQ y cambios de réplicas deben registrarse como evidencia. La comparación de 1 y 2 réplicas Go necesita demostrar distribución real de llamadas gRPC; tener dos Pods Ready no basta. Esta primera fase llega hasta RabbitMQ; Consumer, Operator, Valkey y Grafana continúan pendientes de implementación.

#### Resultado de aplicación y comprobaciones

Se habilitó Gateway API Standard en `cluster-p3-202201117` y se aplicaron, en orden, HealthCheckPolicy, Gateway, HTTPRoute, el ajuste del Deployment Rust y HPA. La API `metrics.k8s.io` ya está disponible y `kubectl top pods` entrega métricas.

- `rust-health`: condición `Attached=True`, comprobación HTTP `/health` en 8080.
- `reports-gateway`: `Accepted=True`, `Programmed=True`, `Ready=True` y `GatewayHealthy=True`. Dirección pública asignada: `136.68.255.122`.
- `reports-route`: `Accepted=True` y `ResolvedRefs=True`, con backend `api-rust-service:80`.
- El backend de Rust en GCP aparece `HEALTHY`, con endpoint del Pod en 8080.
- `api-rust-hpa`: CPU disponible, objetivo 30 %, mínimo 1 y máximo 3; `AbleToScale=True` y `ScalingActive=True`. Rust quedó con una réplica disponible. No se realizó una prueba de escalado bajo carga.

La prueba de un POST válido a `http://136.68.255.122/grpc-202201117` devolvió HTTP 500 con el mensaje `Requested application data is not configured correctly`. Una petición interna al mismo endpoint mediante `api-rust-service` también devolvió 500. Esto sitúa el problema en la aplicación Rust desplegada; el Gateway tiene su regla de ruta y backend configurados correctamente. La cola `military_reports` permaneció con cero mensajes.

Queda pendiente revisar la configuración de datos de Actix de la imagen `api-rust:v1` y su correspondencia con el código actual antes de repetir el POST y ejecutar Locust. Las comprobaciones `/health` no detectan este error del handler de reportes; no se considera validado todavía el flujo completo hasta RabbitMQ.

# Recursos

https://www.youtube.com/watch?v=7s5X7GoYj8k

