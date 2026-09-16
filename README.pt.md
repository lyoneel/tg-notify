---
version: 1.3.20260916
last-updated: 2026-09-16
title: Ly's CLI Telegram Notify
description: CLI em Go para enviar mensagens e arquivos do Telegram pela Bot API
---

# Ly's CLI Telegram Notify

[English](README.md) | [Español](README.es.md) | [Italiano](README.it.md) | **Português**

> **Repositório principal**: https://gitlab.com/lyoneel/tgnotify.
> Se você está lendo isto em qualquer outro host, é um espelho. Por
> favor, abra issues e merge requests no GitLab.

Envie notificações do Telegram com um único e simples comando. Projetado
para não atrapalhar: sem arquivos de configuração, apenas variáveis de
ambiente ou flags. Um comando entrega tudo:

- mensagens de texto
- fotos, vídeos, documentos e arquivos
- arquivos enviados por URL
- reenvio de um arquivo já existente no Telegram
- álbuns de fotos/vídeos (até 10 itens)
- animações GIF e stickers
- responder a uma mensagem específica

Simples de instalar, simples de usar, simples de configurar. O binário é
autocontido, com tratamento de retry integrado para limites de
velocidade e falhas transitórias.

## Exemplos

Enviar uma mensagem:

```bash
tg-notify "Deploy finished"
```

Enviar um arquivo (foto, PDF ou vídeo; o tipo é detectado automaticamente):

```bash
tg-notify -f screenshot.png
tg-notify "Monthly report" -f report.pdf
tg-notify -f clip.mp4
```

Enviar uma animação GIF ou um sticker:

```bash
tg-notify -f logo.gif -t animation
tg-notify -f sticker.webp -t sticker
```

Enviar um álbum de fotos/vídeos:

```bash
tg-notify --album a.jpg b.jpg c.jpg
```

Enviar uma mensagem do stdin (com pipe):

```bash
echo "Build failed" | tg-notify
```

Responder a uma mensagem específica:

```bash
tg-notify --reply-to 42 "Acknowledged"
```

Imprimir a identidade do seu bot (valida o token):

```bash
tg-notify --whoami
```

Quando uma flag de arquivo (`-f`, `--url` ou `--file-id`) está presente,
um argumento de texto inicial se torna a legenda do arquivo em vez de
uma mensagem.

## Instalação

### Baixar uma release

Baixe o binário para a sua plataforma na
[página de releases do GitLab](https://gitlab.com/lyoneel/tgnotify/-/releases)
ou na
[página de releases do GitHub](https://github.com/lyoneel/tgnotify/releases).
Os nomes dos ativos seguem o padrão `tg-notify-<os>-<arch>`, por
exemplo `tg-notify-windows-amd64.exe`, `tg-notify-linux-amd64` ou
`tg-notify-darwin-arm64`.

**Windows**

Baixe o `.exe` para a sua arquitetura, renomeie-o para `tg-notify.exe`
e mova-o para um diretório que já esteja no `PATH`; ou mantenha-o em
sua própria pasta e adicione essa pasta ao `PATH`. No PowerShell:

```powershell
$bin = "$env:USERPROFILE\bin"
New-Item -ItemType Directory -Force $bin | Out-Null
Move-Item .\tg-notify-windows-amd64.exe "$bin\tg-notify.exe"
[Environment]::SetEnvironmentVariable("Path",
  [Environment]::GetEnvironmentVariable("Path", "User") + ";$bin",
  "User")
```

Alternativamente, adicione a pasta em Configurações > Sistema > Sobre
> Configurações avançadas do sistema > Variáveis de ambiente.

**Linux/macOS**

Torne o binário executável e adicione o diretório dele ao `PATH` ou
mova-o para um diretório já no `PATH`:

```bash
chmod +x tg-notify-linux-amd64
mkdir -p ~/bin && mv tg-notify-linux-amd64 ~/bin/tg-notify
```

Certifique-se de que `~/bin` (ou o diretório escolhido) esteja no `PATH`.

### go install

```bash
go install gitlab.com/lyoneel/tgnotify/cmd/tg-notify@latest
```

O binário fica em `$(go env GOPATH)/bin` (certifique-se de que esse
diretório esteja no `PATH`).

### Compilar a partir do código-fonte

```bash
git clone https://gitlab.com/lyoneel/tgnotify.git
cd tgnotify
make build          # ou: go build -o ./build/tg-notify ./cmd/tg-notify
```

O Makefile fornece alvos de build, run, test e qualidade; `make help`
os lista. A versão se auto-marca: `make build` injeta
`git describe --tags --always --dirty`, e binários de `go build` ou
`go install` simples reportam a versão do módulo ou a revisão VCS via
`-v`. Sobrescreva com:

```bash
make build VERSION=v1.3.20260916
```

### Configurar

Defina estas variáveis de ambiente antes de executar:

| Variável             | Obrigatória | Descrição                            |
| -------------------- | ----------- | ------------------------------------ |
| `TELEGRAM_BOT_TOKEN` | sim         | Token do bot do Telegram             |
| `TELEGRAM_CHAT_ID`   | sim         | Chat ID para onde enviar mensagens   |

Ambas podem ser sobrescritas a cada invocação com as flags `--token` e
`--chat-id`. Os Chat IDs são passados como strings, então IDs de grupo
grandes ou negativos (ex. `-1001234567890`) funcionam. Sem arquivo de
configuração: defina-as uma vez no perfil da shell (`~/.bashrc`,
`~/.zshrc`) ou passe-as como flags a cada invocação.

## Avançado

### Uso

```bash
tg-notify "message text"                       # o texto posicional envia uma mensagem
tg-notify -m "message text" -p HTML            # mensagem com flag e formatação
echo "text from stdin" | tg-notify             # mensagem lida do stdin quando não há texto
tg-notify -f path/to/file.png                  # enviar um arquivo local (tipo detectado automaticamente)
tg-notify "caption" -f path/to/file.png        # arquivo local com legenda posicional
tg-notify -u https://example.com/a.pdf -t document
tg-notify -F AgAC... -t photo                  # reenviar um arquivo existente
tg-notify --album a.jpg b.jpg c.jpg            # enviar um álbum de fotos/vídeos (2-10 itens)
tg-notify --reply-to 42 "done"                 # responder à mensagem 42
tg-notify --json "hello"                       # saída JSON legível por máquina
tg-notify -S -f report.pdf                     # enviar sem notificação no telefone
tg-notify -D "hello"                           # pré-visualizar a requisição sem enviar
tg-notify -P socks5://127.0.0.1:1080 "x"       # rotear por um proxy
tg-notify -U http://localhost:8081 -f big.mp4  # Bot API auto-hospedada
tg-notify -A bash                              # imprimir um script de autocompletar do bash
tg-notify --whoami                             # imprimir a identidade do bot
tg-notify -d                                   # imprimir o último chat ID
tg-notify -d -o 123                            # imprimir o chat ID das atualizações após o ID 123
tg-notify -v                                   # imprimir a versão
```

Via make neste repo: `make run ARGS="\"caption\" -f path/to/file.png"`.

### Flags

| Flag | Forma curta | Descrição | Obrigatória |
| ---- | ----- | --------- | ----------- |
| `--message` | `-m` | Texto da mensagem (1-4096 caracteres) | Não; lido do stdin se ausente |
| `--file` | `-f` | Caminho do arquivo local a enviar | Uma de `-f`, `--url`, `--file-id` |
| `--url` | `-u` | URL do arquivo remoto a enviar | Uma de `-f`, `--url`, `--file-id` |
| `--file-id` | `-F` | `file_id` do Telegram a reenviar | Uma de `-f`, `--url`, `--file-id` |
| `--album` | `-a` | Arquivos ou URLs de fotos/vídeos para enviar como um álbum (2-10) | Não |
| `--type` | `-t` | Sobrescrever a detecção automática: `photo`, `document`, `audio`, `video`, `voice`, `animation`, `sticker` | Obrigatória com `--file-id` |
| `--caption` | `-c` | Texto da legenda para arquivos (0-1024 caracteres) | Não |
| `--parse-mode` | `-p` | `MarkdownV2`, `HTML`, ou vazio (padrão: texto simples) | Não |
| `--reply-to` | `-r` | ID da mensagem a responder | Não |
| `--json` | `-j` | Imprimir JSON legível por máquina em vez de texto | Não |
| `--no-trim` | | Preservar os espaços em uma mensagem com pipe | Não |
| `--whoami` | `-w` | Imprimir a identidade do bot e sair | Troca de modo |
| `--token` | `-T` | Token do bot (sobrescreve `TELEGRAM_BOT_TOKEN`) | Não |
| `--chat-id` | `-C` | Chat ID (sobrescreve `TELEGRAM_CHAT_ID`) | Não |
| `--no-retry` | `-n` | Desativar o retry automático em limites 429 e erros transitórios | Não |
| `--retries` | `-R` | Máximo de retries em erros transitórios (padrão 60, 0 desativa) | Não |
| `--base-wait` | `-B` | Primeira espera de backoff em erros transitórios (padrão 2s) | Não |
| `--discover-chat-id` | `-d` | Imprimir o chat ID da última atualização do bot | Troca de modo |
| `--offset` | `-o` | ID de atualização anterior a pular em `--discover-chat-id` | Não |
| `--silent` | `-S` | Entregar sem notificação no telefone | Não |
| `--proxy` | `-P` | URL do proxy (`http`, `https`, `socks5`, `socks5h`) que sobrescreve os proxies de ambiente | Não |
| `--base-url` | `-U` | URL base da Bot API para um servidor auto-hospedado | Não |
| `--dry-run` | `-D` | Imprimir a requisição resolvida sem enviar | Não |
| `--completion` | `-A` | Imprimir um script de autocompletar (`bash`, `zsh`, `fish`) e sair | Troca de modo |
| `--version` | `-v` | Imprimir a versão e sair | Troca de modo |
| `--help` | `-h` | Imprimir o uso e sair | Troca de modo |

Seleção de modo: se `-f`, `--url` ou `--file-id` estiver presente, um
arquivo é enviado e um argumento de texto posicional se torna a legenda
do arquivo; `--album` envia um álbum de fotos/vídeos; sem flag de
arquivo ou álbum, `--discover-chat-id` imprime o chat ID, `--whoami`
imprime a identidade do bot, e o texto posicional ou `--message` é
enviado como mensagem. As flags são analisadas em qualquer lugar da
linha de comando, inclusive após o texto posicional.

O tipo de arquivo é detectado automaticamente pela extensão e pelo tipo
MIME; tipos desconhecidos são enviados como `document`. Envios por URL
usam `document` por padrão quando `--type` está ausente. Uploads locais
são verificados no cliente contra os limites do Telegram: 10 MB para
fotos, 50 MB para todo o resto. Quando `--base-url` aponta para um
servidor Bot API auto-hospedado, o limite sobe para 2000 MB para todos
os tipos.

### API auto-hospedada

Aponte `--base-url` (ou `TELEGRAM_BASE_URL`) para um servidor Bot API
auto-hospedado para rotear as requisições pelo seu próprio gateway em
vez de `api.telegram.org`. Isso contorna o limite de upload do gateway
HTTP oficial, elevando o limite do cliente para 2000 MB por arquivo. O
servidor é o binário `telegram-bot-api` de `tdlib/telegram-bot-api`;
ainda requer um token de bot e uma rota de internet até o Telegram.

### Proxy e .env

`--proxy` (ou `TELEGRAM_PROXY`) roteia cada requisição por um proxy
HTTP, HTTPS, SOCKS5 ou SOCKS5h, sobrescrevendo qualquer configuração de
`HTTP_PROXY`/`HTTPS_PROXY`/`NO_PROXY`. Na inicialização, um arquivo
`./.env` opcional é carregado antes de ler o ambiente, de modo que os
valores fornecidos pelo arquivo preencham qualquer variável não
definida; variáveis de ambiente reais e flags sempre vencem sobre o
arquivo.

### Saída JSON

Passe `--json` para imprimir uma única linha legível por máquina em vez
do resumo legível, para scripting:

```bash
tg-notify --json "hello"
# {"ok":true,"message_id":123}
```

A mesma flag funciona com envios de arquivo e álbum (álbuns emitem um
array `message_ids`) e com `--discover-chat-id` (que emite
`{"chat_id": …}`). Erros ainda são impressos no stderr e saem com
código diferente de zero.

### Comportamento

- Em caso de sucesso, o CLI imprime `Sent (message_id: <id>)` e sai 0.
- Em caso de falha, imprime `Failed: <reason>` no stderr e sai 1; os
  tokens do bot são mascarados de qualquer URL da Bot API na saída de
  erro.
- Em um limite de velocidade 429, ele espera `retry_after` segundos
  (padrão 5) e tenta novamente exatamente uma vez, a menos que
  `--no-retry` esteja definido.
- Em uma falha transitória (timeout de rede, erro de conexão ou HTTP
  5xx), ele tenta novamente até `--retries` vezes (padrão 60) com
  backoff exponencial que começa em `--base-wait` (padrão 2s, dobrando
  até um teto de 60s por espera, com jitter), a menos que `--no-retry`
  esteja definido. Arquivos locais ausentes não são transitórios e
  sempre falham imediatamente.
- Sem mensagem, sem texto posicional e com stdin conectado a um
  terminal, o CLI imprime o uso e sai 1 em vez de esperar entrada com
  pipe; use um pipe para enviá-la (`echo "text" | tg-notify`).

#### Retries

O CLI trata dois tipos de falha automaticamente, cada um com seu próprio
cronograma.

**Limites de velocidade (429).** Quando o Telegram responde com HTTP
429, o CLI dorme pelo intervalo `retry_after` que o Telegram especifica
(padrão 5 segundos) e tenta novamente exatamente uma vez. Não há teto de
retry para ajustar aqui: o único retry é tudo o que ele obtém.

**Erros transitórios.** Timeouts de rede, erros de conexão e respostas
HTTP 5xx são tentados novamente até `--retries` vezes (padrão 60) com
backoff exponencial. O primeiro retry espera `--base-wait` (padrão 2s);
cada tentativa subsequente dobra essa espera até um teto de 60s por
espera, com jitter de ±25% para que muitos clientes em paralelo não
tentem novamente em sincronia. Por exemplo, com os padrões as esperas
são aproximadamente 2s, 4s, 8s, 16s, 32s, e então 60s daí em diante.

Ambos os mecanismos são desativados por `--no-retry`, que faz com que
cada falha retorne imediatamente. Definir `--retries 0` desativa apenas
os retries de erros transitórios; os retries por limite 429 ainda
ocorrem uma vez, a menos que `--no-retry` também esteja definido.
`--retries` deve ser zero ou mais e `--base-wait` maior que zero;
valores inválidos são rejeitados com um erro.

Tentar novamente uma requisição que expirou raramente pode duplicar uma
mensagem, porque uma requisição que já chegou ao Telegram mas expirou na
resposta pode ser enviada duas vezes. Falhas de conexão que nunca
chegaram ao Telegram não podem causar duplicatas.
