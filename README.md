# 🧙‍♂️ Dota 2 Smart Coach (Mid Lane)

Este projeto é um assistente em tempo real para Dota 2 focado na Mid Lane. Ele lê os dados do jogo via **Game State Integration (GSI)**, processa estratégias em um backend **Golang** e exibe alertas visuais em um frontend **Next.js**.

## 🚀 Pré-requisitos

Para rodar este projeto, você precisa ter apenas uma ferramenta instalada:

* **[Docker Desktop](https://www.docker.com/products/docker-desktop/)** (Windows/Mac/Linux)
    * *Nota:* No Windows, certifique-se de que o Docker está rodando com integração ao WSL 2.

---

## 🛠️ Como Rodar o Projeto

1.  Abra o terminal na pasta do projeto.
2.  Execute o comando para construir e subir os containers:

```bash
docker-compose up --build -d
```

O processo pode levar alguns minutos na primeira vez para baixar as imagens do Go e do Node.js.

Para parar o projeto:

```bash
docker-compose down
```

## 🖥️ Acessando o Painel

Após subir os containers, abra seu navegador em: 👉 http://localhost:3006

## ⚙️ Configurando o Dota 2 (GSI)

Para que o jogo envie os dados para o nosso servidor Docker, você precisa criar um arquivo de configuração.

### 1. Descobrindo seu Endereço IP

Embora localhost geralmente funcione, o método mais robusto é usar o IP da sua máquina na rede local.

**No Windows (PowerShell/CMD):**

```powershell
ipconfig
# Procure por "Endereço IPv4" (ex: 192.168.0.15 ou 172.x.x.x se usar WSL)
```

**No Linux / WSL (Terminal):**

```bash
hostname -I
# Pegue o primeiro número que aparecer (ex: 172.25.x.x)
```

### 2. Criando o Arquivo de Configuração

Vá até a pasta de instalação do Dota 2:

Geralmente: `C:\Program Files (x86)\Steam\steamapps\common\dota 2 beta\game\dota\cfg\gamestate_integration\`

Se a pasta `gamestate_integration` não existir, crie-a.

Crie um arquivo chamado `gamestate_integration_coach.cfg`

Cole o seguinte conteúdo (substitua SEU_IP_AQUI pelo IP que você pegou no passo 1, ou tente localhost):

```
"dota2-coach-mid"
{
    "uri"           "http://localhost:8080/"  
    "timeout"       "5.0"
    "buffer"        "0.1"
    "throttle"      "0.1"
    "heartbeat"     "30.0"
    "data"
    {
        "provider"      "1"
        "map"           "1"
        "player"        "1"
        "hero"          "1"
        "abilities"     "1"
        "items"         "1"
    }
}
```

Nota: Se localhost não funcionar, troque para `http://192.168.x.x:8080/` (seu IP real).

### 3. Opções de Inicialização (Launch Options)

Abra a Steam.

Clique com o botão direito no Dota 2 > Propriedades.

Na aba Geral, em "Opções de Inicialização", adicione:

```
-gamestateintegration
```

## 🎮 Como Usar

Certifique-se que o projeto está rodando (`docker-compose up`).

Abra o navegador em http://localhost:3006.

Deverá aparecer: "🟡 Aguardando Partida...".

Abra o Dota 2 e inicie uma partida (Lobby, Bot ou Ranked).

Assim que o herói carregar no mapa, o painel atualizará automaticamente com:

- Alertas de Runas e Stacks.
- Monitor de HP/Mana.
- Status de Buyback.
- Avisos de falta de TP ou Regeneração.

## 📂 Estrutura do Projeto

- `/dota-dashboard`: Frontend em Next.js (Porta 3006).
- `main.go`: Backend em Golang (Porta 8080) que processa a lógica.
- `estratégias.md`: Documentação das táticas de Mid Lane usadas pelo Coach.
- `docker-compose.yml`: Orquestrador dos serviços (Go, Next, Redis).