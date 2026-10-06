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

# Recursos

https://www.youtube.com/watch?v=7s5X7GoYj8k
