---
version: 1.1.20260825
last-updated: 2026-08-25
title: Ly's CLI Telegram Notify
description: CLI in Go per inviare messaggi e file Telegram tramite la Bot API
---

# Ly's CLI Telegram Notify

[English](README.md) | [Español](README.es.md) | **Italiano** | [Português](README.pt.md)

> **Repository principale**: https://gitlab.com/lyoneel/cli-tg-notify.
> Se stai leggendo questo su qualsiasi altro host, è un mirror. Per
> favore, apri issue e merge request su GitLab.

Invia notifiche Telegram con un unico, semplice comando. Progettato per
non intralciarti: nessun file di configurazione, solo variabili
d'ambiente o flag. Un comando consegna tutto:

- messaggi di testo
- foto, video, documenti e file
- file inviati tramite URL
- reinvio di un file già presente su Telegram
- album di foto/video (fino a 10 elementi)
- animazioni GIF e sticker
- rispondere a un messaggio specifico

Semplice da installare, semplice da usare, semplice da configurare. Il
binario è autonomo, con gestione integrata dei retry per i limiti di
velocità e i guasti transitori.

## Esempi

Inviare un messaggio:

```bash
tg-notify "Deploy finished"
```

Inviare un file (foto, PDF o video; il tipo è rilevato automaticamente):

```bash
tg-notify -f screenshot.png
tg-notify "Monthly report" -f report.pdf
tg-notify -f clip.mp4
```

Inviare un'animazione GIF o uno sticker:

```bash
tg-notify -f logo.gif -t animation
tg-notify -f sticker.webp -t sticker
```

Inviare un album di foto/video:

```bash
tg-notify --album a.jpg b.jpg c.jpg
```

Inviare un messaggio da stdin (con pipe):

```bash
echo "Build failed" | tg-notify
```

Rispondere a un messaggio specifico:

```bash
tg-notify --reply-to 42 "Acknowledged"
```

Stampare l'identità del tuo bot (valida il token):

```bash
tg-notify --whoami
```

Quando è presente un flag di file (`-f`, `--url` o `--file-id`), un
argomento di testo iniziale diventa la didascalia del file invece di un
messaggio.

## Installazione

### Scaricare una release

Scarica il binario per la tua piattaforma dalla
[pagina delle release di GitLab](https://gitlab.com/lyoneel/cli-tg-notify/-/releases)
o dalla
[pagina delle release di GitHub](https://github.com/lyoneel/cli-tg-notify/releases).
I nomi degli asset seguono lo schema `tg-notify-<os>-<arch>`, ad
esempio `tg-notify-windows-amd64.exe`, `tg-notify-linux-amd64` o
`tg-notify-darwin-arm64`.

**Windows**

Scarica il `.exe` per la tua architettura, rinominalo in
`tg-notify.exe` e spostalo in una directory già presente nel `PATH`;
oppure tienilo in una sua cartella e aggiungi quella cartella al
`PATH`. In PowerShell:

```powershell
$bin = "$env:USERPROFILE\bin"
New-Item -ItemType Directory -Force $bin | Out-Null
Move-Item .\tg-notify-windows-amd64.exe "$bin\tg-notify.exe"
[Environment]::SetEnvironmentVariable("Path",
  [Environment]::GetEnvironmentVariable("Path", "User") + ";$bin",
  "User")
```

In alternativa, aggiungi la cartella da Impostazioni > Sistema >
Informazioni > Impostazioni di sistema avanzate > Variabili d'ambiente.

**Linux/macOS**

Rendi il binario eseguibile e aggiungi la sua directory al `PATH`
oppure spostala in una directory già nel `PATH`:

```bash
chmod +x tg-notify-linux-amd64
mkdir -p ~/bin && mv tg-notify-linux-amd64 ~/bin/tg-notify
```

Assicurati che `~/bin` (o la directory scelta) sia nel `PATH`.

### go install

```bash
go install gitlab.com/lyoneel/cli-tg-notify/cmd/tg-notify@latest
```

Il binario finisce in `$(go env GOPATH)/bin` (assicurati che quella
directory sia nel `PATH`).

### Compilare dai sorgenti

```bash
git clone https://gitlab.com/lyoneel/cli-tg-notify.git
cd cli-tg-notify
make build          # oppure: go build -o ./build/tg-notify ./cmd/tg-notify
```

Il Makefile fornisce target di build, run, test e qualità; `make help`
li elenca. La versione si auto-marca: `make build` inietta
`git describe --tags --always --dirty`, e i binari da `go build` o
`go install` semplici riportano la versione del modulo o la revisione
VCS tramite `-v`. Sovrascrivila con:

```bash
make build VERSION=v1.1.20260825
```

### Configurare

Imposta queste variabili d'ambiente prima di eseguire:

| Variabile            | Obbligatoria | Descrizione                          |
| -------------------- | ------------ | ------------------------------------ |
| `TELEGRAM_BOT_TOKEN` | sì           | Token del bot Telegram               |
| `TELEGRAM_CHAT_ID`   | sì           | Chat ID a cui inviare i messaggi     |

Entrambe possono essere sovrascritte a ogni invocazione con i flag
`--token` e `--chat-id`. I Chat ID vengono passati come stringhe, quindi
gli ID di gruppo grandi o negativi (es. `-1001234567890`) funzionano.
Nessun file di configurazione: impostale una volta nel profilo della
shell (`~/.bashrc`, `~/.zshrc`) o passale come flag a ogni invocazione.

## Avanzato

### Utilizzo

```bash
tg-notify "message text"                       # il testo posizionale invia un messaggio
tg-notify -m "message text" -p HTML            # messaggio con flag e formattazione
echo "text from stdin" | tg-notify             # messaggio letto da stdin quando non c'è testo
tg-notify -f path/to/file.png                  # inviare un file locale (tipo rilevato automaticamente)
tg-notify "caption" -f path/to/file.png        # file locale con didascalia posizionale
tg-notify -u https://example.com/a.pdf -t document
tg-notify -F AgAC... -t photo                  # reinviare un file esistente
tg-notify --album a.jpg b.jpg c.jpg            # inviare un album di foto/video (2-10 elementi)
tg-notify --reply-to 42 "done"                 # rispondere al messaggio 42
tg-notify --json "hello"                       # output JSON leggibile da una macchina
tg-notify -S -f report.pdf                     # inviare senza notifica sul telefono
tg-notify -D "hello"                           # anteprima della richiesta senza inviare
tg-notify -P socks5://127.0.0.1:1080 "x"       # instradare tramite un proxy
tg-notify -U http://localhost:8081 -f big.mp4  # Bot API self-hosted
tg-notify -A bash                              # stampare uno script di completamento bash
tg-notify --whoami                             # stampare l'identità del bot
tg-notify -d                                   # stampare l'ultimo chat ID
tg-notify -d -o 123                            # stampare il chat ID dagli aggiornamenti dopo l'ID 123
tg-notify -v                                   # stampare la versione
```

Tramite make in questo repo: `make run ARGS="\"caption\" -f path/to/file.png"`.

### Flag

| Flag | Abbreviazione | Descrizione | Obbligatorio |
| ---- | ----- | ----------- | ------------ |
| `--message` | `-m` | Testo del messaggio (1-4096 caratteri) | No; letto da stdin se assente |
| `--file` | `-f` | Percorso del file locale da caricare | Uno tra `-f`, `--url`, `--file-id` |
| `--url` | `-u` | URL del file remoto da inviare | Uno tra `-f`, `--url`, `--file-id` |
| `--file-id` | `-F` | `file_id` Telegram da reinviare | Uno tra `-f`, `--url`, `--file-id` |
| `--album` | `-a` | File o URL di foto/video da inviare come un album (2-10) | No |
| `--type` | `-t` | Sovrascrivere il rilevamento automatico: `photo`, `document`, `audio`, `video`, `voice`, `animation`, `sticker` | Obbligatorio con `--file-id` |
| `--caption` | `-c` | Testo della didascalia per i file (0-1024 caratteri) | No |
| `--parse-mode` | `-p` | `MarkdownV2`, `HTML`, o vuoto (default: testo semplice) | No |
| `--reply-to` | `-r` | ID del messaggio a cui rispondere | No |
| `--json` | `-j` | Stampare JSON leggibile da una macchina invece di testo | No |
| `--no-trim` | | Conservare gli spazi in un messaggio con pipe | No |
| `--whoami` | `-w` | Stampare l'identità del bot ed uscire | Cambio modalità |
| `--token` | `-T` | Token del bot (sovrascrive `TELEGRAM_BOT_TOKEN`) | No |
| `--chat-id` | `-C` | Chat ID (sovrascrive `TELEGRAM_CHAT_ID`) | No |
| `--no-retry` | `-n` | Disattivare il retry automatico sui limiti 429 e sugli errori transitori | No |
| `--retries` | `-R` | Numero massimo di retry sugli errori transitori (default 60, 0 disattiva) | No |
| `--base-wait` | `-B` | Prima attesa di backoff sugli errori transitori (default 2s) | No |
| `--discover-chat-id` | `-d` | Stampare il chat ID dall'ultimo aggiornamento del bot | Cambio modalità |
| `--offset` | `-o` | ID di aggiornamento precedente da saltare in `--discover-chat-id` | No |
| `--silent` | `-S` | Consegnare senza notifica sul telefono | No |
| `--proxy` | `-P` | URL del proxy (`http`, `https`, `socks5`, `socks5h`) che sovrascrive i proxy d'ambiente | No |
| `--base-url` | `-U` | URL base della Bot API per un server self-hosted | No |
| `--dry-run` | `-D` | Stampare la richiesta risolta senza inviare | No |
| `--completion` | `-A` | Stampare uno script di completamento (`bash`, `zsh`, `fish`) ed uscire | Cambio modalità |
| `--version` | `-v` | Stampare la versione ed uscire | Cambio modalità |
| `--help` | `-h` | Stampare l'utilizzo ed uscire | Cambio modalità |

Selezione della modalità: se è presente `-f`, `--url` o `--file-id`, si
invia un file e un argomento di testo posizionale diventa la didascalia
del file; `--album` invia un album di foto/video; senza flag di file o
album, `--discover-chat-id` stampa il chat ID, `--whoami` stampa
l'identità del bot, e il testo posizionale o `--message` viene inviato
come messaggio. I flag vengono analizzati ovunque nella riga di comando,
anche dopo il testo posizionale.

Il tipo di file è rilevato automaticamente dall'estensione e dal tipo
MIME; i tipi sconosciuti vengono inviati come `document`. Gli invii via
URL usano `document` per default quando `--type` è assente. I caricamenti
locali vengono verificati lato client contro i limiti di Telegram: 10 MB
per le foto, 50 MB per tutto il resto. Quando `--base-url` punta a un
server Bot API self-hosted, il limite sale a 2000 MB per ogni tipo.

### API self-hosted

Punta `--base-url` (o `TELEGRAM_BASE_URL`) a un server Bot API
self-hosted per instradare le richieste attraverso il tuo gateway invece
di `api.telegram.org`. Questo aggira il limite di caricamento del gateway
HTTP ufficiale, portando il limite lato client a 2000 MB per file. Il
server è il binario `telegram-bot-api` di `tdlib/telegram-bot-api`;
richiede comunque un token del bot e una rotta internet verso Telegram.

### Proxy e .env

`--proxy` (o `TELEGRAM_PROXY`) instrada ogni richiesta attraverso un
proxy HTTP, HTTPS, SOCKS5 o SOCKS5h, sovrascrivendo qualsiasi
impostazione `HTTP_PROXY`/`HTTPS_PROXY`/`NO_PROXY`. All'avvio, un file
`./.env` opzionale viene caricato prima di leggere l'ambiente, così i
valori forniti dal file riempiono qualsiasi variabile non impostata; le
variabili d'ambiente reali e i flag vincono sempre sul file.

### Output JSON

Passa `--json` per stampare una singola riga leggibile da una macchina
invece del riepilogo leggibile, per lo scripting:

```bash
tg-notify --json "hello"
# {"ok":true,"message_id":123}
```

Lo stesso flag funziona con gli invii di file e album (gli album
emettono un array `message_ids`) e con `--discover-chat-id` (che emette
`{"chat_id": …}`). Gli errori vengono comunque stampati su stderr ed
escono con codice diverso da zero.

### Comportamento

- In caso di successo il CLI stampa `Sent (message_id: <id>)` ed esce 0.
- In caso di errore stampa `Failed: <reason>` su stderr ed esce 1; i
  token del bot vengono mascherati da qualsiasi URL della Bot API
  nell'output di errore.
- Su un limite di velocità 429 attende `retry_after` secondi (default 5)
  e ritenta esattamente una volta, a meno che non sia impostato
  `--no-retry`.
- Su un guasto transitorio (timeout di rete, errore di connessione o
  HTTP 5xx) ritenta fino a `--retries` volte (default 60) con backoff
  esponenziale che parte da `--base-wait` (default 2s, raddoppiando fino
  a un tetto di 60s per attesa, con jitter), a meno che non sia
  impostato `--no-retry`. I file locali mancanti non sono transitori e
  falliscono sempre immediatamente.
- Senza messaggio, senza testo posizionale e con stdin collegato a un
  terminale, il CLI stampa l'utilizzo ed esce 1 invece di attendere
  input con pipe; usa un pipe per inviarlo (`echo "text" | tg-notify`).

#### Retry

Il CLI gestisce due tipi di guasto automaticamente, ognuno con il
proprio calendario.

**Limiti di velocità (429).** Quando Telegram risponde con HTTP 429, il
CLI dorme per l'intervallo `retry_after` specificato da Telegram
(default 5 secondi) e ritenta esattamente una volta. Qui non c'è un
tetto di retry da regolare: l'unico retry è tutto ciò che ottiene.

**Errori transitori.** I timeout di rete, gli errori di connessione e
le risposte HTTP 5xx vengono ritentati fino a `--retries` volte (default
60) con backoff esponenziale. Il primo retry attende `--base-wait`
(default 2s); ogni tentativo successivo raddoppia quell'attesa fino a un
tetto di 60s per attesa, con jitter di ±25% affinché molti client in
parallelo non ritentino all'unisono. Ad esempio, con i default le attese
sono circa 2s, 4s, 8s, 16s, 32s, poi 60s da lì in poi.

Entrambi i meccanismi sono disattivati da `--no-retry`, che fa sì che
ogni guasto ritorni immediatamente. Impostare `--retries 0` disattiva
solo i retry per errori transitori; i retry per limite 429 avvengono
comunque una volta a meno che non sia impostato anche `--no-retry`.
`--retries` deve essere zero o più e `--base-wait` maggiore di zero; i
valori non validi vengono rifiutati con un errore.

Ritentare una richiesta scaduta può raramente duplicare un messaggio,
perché una richiesta che ha già raggiunto Telegram ma è scaduta sulla
risposta può essere inviata due volte. I guasti di connessione che non
hanno mai raggiunto Telegram non possono causare duplicati.
