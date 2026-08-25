---
version: 1.0.20260825
last-updated: 2026-08-25
title: Ly's CLI Telegram Notify
description: CLI en Go para enviar mensajes y archivos de Telegram mediante la Bot API
---

# Ly's CLI Telegram Notify

[English](README.md) | **Español** | [Italiano](README.it.md) | [Português](README.pt.md)

> **Repositorio principal**: https://gitlab.com/lyoneel/cli-tg-notify.
> Si estás leyendo esto en cualquier otro host, es un espejo. Por favor,
> abre issues y merge requests en GitLab.

Envía notificaciones de Telegram con un único y sencillo comando.
Diseñado para no estorbar: sin archivos de configuración, solo variables
de entorno o flags. Un comando lo entrega todo:

- mensajes de texto
- fotos, vídeos, documentos y archivos
- archivos enviados por URL
- reenvío de un archivo ya existente en Telegram
- álbumes de fotos/vídeos (hasta 10 elementos)
- animaciones GIF y stickers
- responder a un mensaje concreto

Fácil de instalar, fácil de usar, fácil de configurar. El binario es
autocontenido, con reintentos integrados para límites de velocidad y
fallos transitorios.

## Ejemplos

Enviar un mensaje:

```bash
tg-notify "Deploy finished"
```

Enviar un archivo (foto, PDF o vídeo; el tipo se detecta automáticamente):

```bash
tg-notify -f screenshot.png
tg-notify "Monthly report" -f report.pdf
tg-notify -f clip.mp4
```

Enviar una animación GIF o un sticker:

```bash
tg-notify -f logo.gif -t animation
tg-notify -f sticker.webp -t sticker
```

Enviar un álbum de fotos/vídeos:

```bash
tg-notify --album a.jpg b.jpg c.jpg
```

Enviar un mensaje desde stdin (con pipe):

```bash
echo "Build failed" | tg-notify
```

Responder a un mensaje concreto:

```bash
tg-notify --reply-to 42 "Acknowledged"
```

Mostrar la identidad de tu bot (valida el token):

```bash
tg-notify --whoami
```

Cuando hay un flag de archivo (`-f`, `--url` o `--file-id`), un
argumento de texto inicial se convierte en el caption del archivo en
lugar de un mensaje.

## Instalación

### Descargar una release

Descarga el binario para tu plataforma desde la
[página de releases](https://gitlab.com/lyoneel/cli-tg-notify/-/releases).
Los nombres de los activos siguen el patrón `tg-notify-<os>-<arch>`,
por ejemplo `tg-notify-windows-amd64.exe`, `tg-notify-linux-amd64` o
`tg-notify-darwin-arm64`.

**Windows**

Descarga el `.exe` para tu arquitectura, renómbralo a `tg-notify.exe`
y muévelo a un directorio que ya esté en el `PATH`; o guárdalo en su
propia carpeta y añade esa carpeta al `PATH`. En PowerShell:

```powershell
$bin = "$env:USERPROFILE\bin"
New-Item -ItemType Directory -Force $bin | Out-Null
Move-Item .\tg-notify-windows-amd64.exe "$bin\tg-notify.exe"
[Environment]::SetEnvironmentVariable("Path",
  [Environment]::GetEnvironmentVariable("Path", "User") + ";$bin",
  "User")
```

Alternativamente, añade la carpeta desde Configuración > Sistema >
Información > Configuración avanzada del sistema > Variables de
entorno.

**Linux/macOS**

Haz el binario ejecutable y añade su directorio al `PATH` o muévelo a
un directorio que ya esté en el `PATH`:

```bash
chmod +x tg-notify-linux-amd64
mkdir -p ~/bin && mv tg-notify-linux-amd64 ~/bin/tg-notify
```

Asegúrate de que `~/bin` (o el directorio que elijas) esté en el `PATH`.

### go install

```bash
go install gitlab.com/lyoneel/cli-tg-notify/cmd/tg-notify@latest
```

El binario queda en `$(go env GOPATH)/bin` (asegúrate de que ese
directorio esté en `PATH`).

### Compilar desde el código fuente

```bash
git clone https://gitlab.com/lyoneel/cli-tg-notify.git
cd cli-tg-notify
make build          # o: go build -o ./build/tg-notify ./cmd/tg-notify
```

El Makefile proporciona targets de build, run, test y calidad; `make
help` los lista. La versión se autosella: `make build` inyecta
`git describe --tags --always --dirty`, y los binarios de `go build` o
`go install` simples informan su versión de módulo o revisión VCS
mediante `-v`. Sobrescríbela con:

```bash
make build VERSION=v1.0.20260825
```

### Configurar

Establece estas variables de entorno antes de ejecutar:

| Variable             | Obligatoria | Descripción                            |
| -------------------- | ----------- | -------------------------------------- |
| `TELEGRAM_BOT_TOKEN` | sí          | Token del bot de Telegram              |
| `TELEGRAM_CHAT_ID`   | sí          | Chat ID al que enviar los mensajes     |

Ambas pueden sobrescribirse en cada invocación con los flags `--token`
y `--chat-id`. Los Chat IDs se pasan como cadenas, por lo que los IDs
de grupo grandes o negativos (p. ej. `-1001234567890`) funcionan. Sin
archivo de configuración: establécelas una vez en tu perfil de shell
(`~/.bashrc`, `~/.zshrc`) o pásalas como flags en cada invocación.

## Avanzado

### Uso

```bash
tg-notify "message text"                       # el texto posicional envía un mensaje
tg-notify -m "message text" -p HTML            # mensaje con flag y formato
echo "text from stdin" | tg-notify             # mensaje leído de stdin cuando no hay texto
tg-notify -f path/to/file.png                  # enviar un archivo local (tipo autodetectado)
tg-notify "caption" -f path/to/file.png        # archivo local con caption posicional
tg-notify -u https://example.com/a.pdf -t document
tg-notify -F AgAC... -t photo                  # reenviar un archivo existente
tg-notify --album a.jpg b.jpg c.jpg            # enviar un álbum de fotos/vídeos (2-10 elementos)
tg-notify --reply-to 42 "done"                 # responder al mensaje 42
tg-notify --json "hello"                       # salida JSON legible por máquina
tg-notify -S -f report.pdf                     # enviar sin notificación en el teléfono
tg-notify -D "hello"                           # previsualizar la petición sin enviar
tg-notify -P socks5://127.0.0.1:1080 "x"       # enrutar a través de un proxy
tg-notify -U http://localhost:8081 -f big.mp4  # Bot API autoalojada
tg-notify -A bash                              # imprimir un script de autocompletado de bash
tg-notify --whoami                             # imprimir la identidad del bot
tg-notify -d                                   # imprimir el último chat ID
tg-notify -d -o 123                            # imprimir el chat ID de las actualizaciones tras el ID 123
tg-notify -v                                   # imprimir la versión
```

Mediante make en este repo: `make run ARGS="\"caption\" -f path/to/file.png"`.

### Flags

| Flag | Abreviatura | Descripción | Obligatorio |
| ---- | ----- | ----------- | ----------- |
| `--message` | `-m` | Texto del mensaje (1-4096 caracteres) | No; se lee de stdin si falta |
| `--file` | `-f` | Ruta del archivo local a subir | Uno de `-f`, `--url`, `--file-id` |
| `--url` | `-u` | URL del archivo remoto a enviar | Uno de `-f`, `--url`, `--file-id` |
| `--file-id` | `-F` | `file_id` de Telegram a reenviar | Uno de `-f`, `--url`, `--file-id` |
| `--album` | `-a` | Archivos o URLs de fotos/vídeos para enviar como un álbum (2-10) | No |
| `--type` | `-t` | Sobrescribir la autodetección: `photo`, `document`, `audio`, `video`, `voice`, `animation`, `sticker` | Obligatorio con `--file-id` |
| `--caption` | `-c` | Texto del caption para archivos (0-1024 caracteres) | No |
| `--parse-mode` | `-p` | `MarkdownV2`, `HTML`, o vacío (por defecto: texto plano) | No |
| `--reply-to` | `-r` | ID del mensaje al que responder | No |
| `--json` | `-j` | Imprimir JSON legible por máquina en lugar de texto | No |
| `--no-trim` | | Conservar los espacios en un mensaje con pipe | No |
| `--whoami` | `-w` | Imprimir la identidad del bot y salir | Cambio de modo |
| `--token` | `-T` | Token del bot (sobrescribe `TELEGRAM_BOT_TOKEN`) | No |
| `--chat-id` | `-C` | Chat ID (sobrescribe `TELEGRAM_CHAT_ID`) | No |
| `--no-retry` | `-n` | Desactivar el reintento automático en límites 429 y errores transitorios | No |
| `--retries` | `-R` | Máximo de reintentos en errores transitorios (por defecto 60, 0 lo desactiva) | No |
| `--base-wait` | `-B` | Primera espera de backoff en errores transitorios (por defecto 2s) | No |
| `--discover-chat-id` | `-d` | Imprimir el chat ID de la última actualización del bot | Cambio de modo |
| `--offset` | `-o` | ID de actualización anterior a omitir en `--discover-chat-id` | No |
| `--silent` | `-S` | Entregar sin notificación en el teléfono | No |
| `--proxy` | `-P` | URL del proxy (`http`, `https`, `socks5`, `socks5h`) que sobrescribe los proxies de entorno | No |
| `--base-url` | `-U` | URL base de la Bot API para un servidor autoalojado | No |
| `--dry-run` | `-D` | Imprimir la petición resuelta sin enviar | No |
| `--completion` | `-A` | Imprimir un script de autocompletado (`bash`, `zsh`, `fish`) y salir | Cambio de modo |
| `--version` | `-v` | Imprimir la versión y salir | Cambio de modo |
| `--help` | `-h` | Imprimir el uso y salir | Cambio de modo |

Selección de modo: si está presente `-f`, `--url` o `--file-id`, se
envía un archivo y un argumento de texto posicional se convierte en el
caption del archivo; `--album` envía un álbum de fotos/vídeos; sin
flag de archivo o álbum, `--discover-chat-id` imprime el chat ID,
`--whoami` imprime la identidad del bot, y el texto posicional o
`--message` se envía como mensaje. Los flags se analizan en cualquier
posición de la línea de comandos, incluso después del texto posicional.

El tipo de archivo se autodetecta a partir de la extensión y el tipo
MIME; los tipos desconocidos se envían como `document`. Los envíos por
URL usan `document` por defecto cuando falta `--type`. Las subidas
locales se comprueban en el cliente contra los límites de Telegram:
10 MB para fotos, 50 MB para todo lo demás. Cuando `--base-url` apunta
a un servidor de Bot API autoalojado, el límite sube a 2000 MB para
todos los tipos.

### API autoalojada

Apunta `--base-url` (o `TELEGRAM_BASE_URL`) a un servidor de Bot API
autoalojado para enrutar las peticiones a través de tu propia puerta de
enlace en lugar de `api.telegram.org`. Esto evita el límite de subida
de la puerta de enlace HTTP oficial, elevando el límite del cliente a
2000 MB por archivo. El servidor es el binario `telegram-bot-api` de
`tdlib/telegram-bot-api`; sigue requiriendo un token de bot y una ruta
de internet hacia Telegram.

### Proxy y .env

`--proxy` (o `TELEGRAM_PROXY`) enruta cada petición a través de un
proxy HTTP, HTTPS, SOCKS5 o SOCKS5h, sobrescribiendo cualquier ajuste
de `HTTP_PROXY`/`HTTPS_PROXY`/`NO_PROXY`. Al arrancar, se carga un
archivo `./.env` opcional antes de leer el entorno, de modo que los
valores del archivo rellenan cualquier variable no establecida; las
variables de entorno reales y los flags siempre ganan sobre el archivo.

### Salida JSON

Pasa `--json` para imprimir una única línea legible por máquina en
lugar del resumen legible por humanos, para scripting:

```bash
tg-notify --json "hello"
# {"ok":true,"message_id":123}
```

El mismo flag funciona con envíos de archivos y álbumes (los álbumes
emiten un array `message_ids`) y con `--discover-chat-id` (que emite
`{"chat_id": …}`). Los errores se siguen imprimiendo en stderr y salen
con código distinto de cero.

### Comportamiento

- En caso de éxito, el CLI imprime `Sent (message_id: <id>)` y sale 0.
- En caso de fallo, imprime `Failed: <reason>` en stderr y sale 1; los
  tokens del bot se enmascaran en cualquier URL de la Bot API en la
  salida de error.
- Ante un límite de velocidad 429, espera `retry_after` segundos (por
  defecto 5) y reintenta exactamente una vez, salvo que se establezca
  `--no-retry`.
- Ante un fallo transitorio (timeout de red, error de conexión o HTTP
  5xx), reintenta hasta `--retries` veces (por defecto 60) con backoff
  exponencial que empieza en `--base-wait` (por defecto 2s, duplicando
  hasta un tope de 60s por espera, con jitter), salvo que se establezca
  `--no-retry`. Los archivos locales que faltan no son transitorios y
  siempre fallan de inmediato.
- Sin mensaje, sin texto posicional y con stdin conectado a una
  terminal, el CLI imprime el uso y sale 1 en lugar de esperar entrada
  con pipe; usa un pipe para enviarlo (`echo "text" | tg-notify`).

#### Reintentos

El CLI maneja dos tipos de fallo automáticamente, cada uno con su
propio calendario.

**Límites de velocidad (429).** Cuando Telegram responde con HTTP 429,
el CLI duerme durante el intervalo `retry_after` que Telegram especifica
(por defecto 5 segundos) y reintenta exactamente una vez. Aquí no hay
tope de reintentos que ajustar: el único reintento es todo lo que hay.

**Errores transitorios.** Los timeouts de red, errores de conexión y
respuestas HTTP 5xx se reintentan hasta `--retries` veces (por defecto
60) con backoff exponencial. El primer reintento espera `--base-wait`
(por defecto 2s); cada intento posterior duplica esa espera hasta un
tope de 60s por espera, con jitter de ±25% para que muchos clientes en
paralelo no reintenten al unísono. Por ejemplo, con los valores por
defecto las esperas son aproximadamente 2s, 4s, 8s, 16s, 32s, y luego
60s en adelante.

Ambos mecanismos se desactivan con `--no-retry`, que hace que cada
fallo devuelva de inmediato. Establecer `--retries 0` desactiva solo
los reintentos de errores transitorios; los reintentos por límite 429
siguen ocurriendo una vez salvo que también se establezca `--no-retry`.
`--retries` debe ser cero o más y `--base-wait` mayor que cero; los
valores inválidos se rechazan con un error.

Reintentar una petición que agotó el tiempo de espera rara vez puede
duplicar un mensaje, porque una petición que ya llegó a Telegram pero
agotó el tiempo en la respuesta puede enviarse dos veces. Los fallos de
conexión que nunca llegaron a Telegram no pueden causar duplicados.
