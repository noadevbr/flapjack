# Flapjack
[![wakatime](https://wakatime.com/badge/user/8d7b071b-f3ff-4a46-83c1-f929d6e39e35/project/1ae6eefa-6f24-41ca-953c-110cc09a5d9b.svg)](https://wakatime.com/badge/user/8d7b071b-f3ff-4a46-83c1-f929d6e39e35/project/1ae6eefa-6f24-41ca-953c-110cc09a5d9b)

CLI para Pterodactyl com IA integrada (Groq).

## Funcionalidades

- Chat com IA (Groq via Llama 3.3 70B)
- Gerenciamento de servidores Pterodactyl (listar, power, console)
- Dashboard HTML com HTMX
- Cache criptografado de chaves (AES-256-GCM + DPAPI no Windows)
- Salvamento e restauração de conversas
- Autocomplete de comandos

## Comandos

```
<texto>          enviar mensagem para IA
:config set      salvar chave (criptografada)
:config get      exibir chave
:config list     listar chaves salvas
:config delete   remover chave
:conv save       salvar conversa atual
:conv list       listar conversas salvas
:conv load       restaurar conversa
:conv new        nova conversa
:conv delete     remover conversa
:v               alternar verbose
:help            ajuda
exit             sair
```

## Setup

```bash
make local
./build/flapjack.exe
```

Na primeira execução, configure as chaves:

```
:config set GROQ_API_KEY gsk_sua_chave
:config set PTERODACTYL_URL https://painel.exemplo.com
:config set PTERODACTYL_API_KEY ptlc_sua_chave
```

Se houver um `.env` na raiz, as chaves são migradas automaticamente para o cache criptografado na primeira execução.

## Build

```bash
make local        # windows
make linux        # cross-compile linux amd64
make mac          # cross-compile darwin amd64
make mac-arm      # cross-compile darwin arm64
make all          # todos
```

## Segurança

- Chaves armazenadas em `~/.flapjack/store.dat` — criptografadas com AES-256-GCM
- Master key protegida via DPAPI (Windows) ou permissão 0600 (Unix)
- Conversas salvas em `~/.flapjack/conversations/<id>.dat` — também criptografadas
- O índice de conversas (`index.json`) é texto plano (contém apenas metadados)
