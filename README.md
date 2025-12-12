# Introdução

Este projeto implementa uma simulação de comunicação espacial entre uma **Nave-Mãe (Mothership)** e vários **Rovers**, monitorizados por um **Ground Control** (interface web). O sistema utiliza protocolos personalizados sobre TCP (`TelemetryStream`) e UDP (`MissionLink`).

# 1\. Guia de Utilização

## 1.1 Pré-requisitos

Antes de iniciar, garanta que tem as seguintes ferramentas instaladas:

  * **Go (Golang):** Versão 1.24 ou superior.
  * **Node.js & npm:** Necessário para compilar a interface gráfica (Ground Control).
  * **CORE Emulator:** Necessário apenas para a simulação de rede realista (Ambiente VM/Linux).
  * **Make:** Para automação da compilação.
  * **Python 3:** Para execução dos scripts de automação do cenário CORE.

## 1.2 Compilação (Build)

O projeto utiliza um `Makefile` para gerir a compilação de todos os componentes (backend e frontend).

Para limpar artefactos antigos e compilar os binários da Mothership, Rover e Ground Control (incluindo o frontend React):

```bash
make all
```

**Compilar Componentes Individualmente:**

  * Apenas Mothership: `make mothership`
  * Apenas Rover: `make rover`
  * Apenas Ground Control (Go + React): `make groundcontrol`

Os binários compilados serão colocados na pasta `bin/`.

## 1.3 Ajuste de Configurações

O sistema pode ser configurado de duas formas: via **Flags de linha de comando** (runtime) ou alterando **Constantes no código** (compile-time).

### A. Configurações de Runtime (Flags)

Ao executar os binários manualmente, pode alterar endereços e portas:

**Mothership:**

  * `-ts-addr`: Endereço do Telemetry Stream (TCP). *Padrão: :9001*
  * `-ml-addr`: Endereço do Mission Link (UDP). *Padrão: :9002*
  * `-api-addr`: Endereço da API HTTP. *Padrão: :9003*

**Rover:**

  * `-id`: ID único do Rover.
  * `-m-ts-addr`: Endereço TCP da Mothership.
  * `-m-ml-addr`: Endereço UDP da Mothership.
  * `-r-ml-addr`: Endereço UDP local do Rover.

**Ground Control:**

  * `-port`: Porta para servir o website. *Padrão: :3000*
  * `-api-addr`: URL da API da Mothership. *Padrão: 10.0.0.21:8003*

### B. Configurações Internas (Código)

Para ajustes profundos no comportamento dos protocolos ou simulação, edite os seguintes ficheiros (ou opte por instanciar configurações sob medida):

**Protocolo UDP (MissionLink)** (`pkg/transport/udplink/config.go`):

  * **Timeouts:** Define prazos de leitura/escrita (`Read`, `Write`), tempo de vida de pacotes fragmentados (`RecvTTL`) e tempo máximo de espera por pacotes fora de ordem (`InOrder`).
  * **Retransmission:** Controla o número máximo de tentativas (`MaxRetries`), o tempo de espera inicial (`InitialBackoff`) e o multiplicador exponencial (`BackoffMultiplier`).
  * **FEC (Forward Error Correction):** Define o MTU, o mínimo de fragmentos de dados e o rácio de redundância (`ParityShardRatio`) para reconstrução de pacotes perdidos sem retransmissão.
  * **Workers:** Limites de concorrência (`MaxRecvWorkers`, `MaxDeliveryWorkers`).
  * **DefaultLoopTick:** Frequência dos loops de verificação de retransmissão e limpeza.

**Protocolo TCP (TelemetryStream)** (`pkg/transport/tcpstream/config.go`):

  * **DefaultClientTimeout:** Timeouts de conexão (`Dial`) e envio (`Write`).
  * **DefaultReconnect:** Estratégia de reconexão automática, incluindo `InitialBackoff`, `MaxBackoff` e `BackoffMultiplier`.
  * **DefaultServerTimeout:** Prazos para aceitar conexões (`Listen`) e ler dados (`Read`), permitindo tolerar silêncios momentâneos na rede.

**Frequência dos ciclos do Rover** (`cmd/rover/main.go`):

  * `telemetryUpdateFrequency`: Frequência com que o Rover envia dados de telemetria via TCP (padrão: 100ms).
  * `missionRequestFrequency`: Frequência com que o Rover, estando livre, solicita novas missões via UDP (padrão: 5s).

**Lógica da Mothership** (`cmd/mothership/main.go`):

  * `staleMission`: Número de ciclos de atualização falhados antes de considerar uma missão como "Unknown".
  * `staleRover`: Tempo de silêncio (sem telemetria) antes de marcar um rover como "Unknown".

## 1.4 Execução do Projeto

Existem dois modos principais de execução: **Localhost** (desenvolvimento) e **CORE Emulator** (simulação de rede).

### A. Localhost (Sem emulação de rede)

Este modo executa todos os componentes na máquina local (`127.0.0.1`) sem emulação de rede. É ideal para testar a lógica da aplicação, a interface gráfica e a comunicação básica sem a complexidade de perdas de pacotes ou latência.

Recomenda-se iniciar os componentes na seguinte ordem para garantir que as conexões TCP são estabelecidas corretamente:

1.  **Iniciar a Mothership:** Execute o script auxiliar que levanta a mothership nas portas locais 9001-9003.
    ```bash
    ./scripts/mothership.sh
    ```
2.  **Iniciar o Ground Control:** Este script inicia o servidor web e abre o browser (Firefox) automaticamente na interface de controlo.
    ```bash
    ./scripts/groundcontrol.sh
    ```
3.  **Iniciar a Frota de Rovers:** Este script inicia 6 rovers, ligando-os à mothership local (`127.0.0.1`).
    ```bash
    ./scripts/rover.sh
    ```

### B. CORE Emulator (Com topologia de rede)

Este é o cenário principal do trabalho prático, onde a topologia de rede (satélites, perdas, latência) é aplicada.

**Opção 1: Execução Automática (Script)**
Para simplificar o processo, existe um script em Python que orquestra a simulação. Este script é válido apenas para a topologia fornecida no projeto.

**Executar o Script:** Num terminal, execute o seguinte comando (requer privilégios de root para interagir com o CORE Daemon):

```bash
sudo python3 scripts/run_simulation.py
```

**Fluxo de Execução:**

1.  O script abrirá automaticamente o **CORE GUI**.
2.  No terminal, serão apresentadas instruções para carregar o ficheiro `coreemu/topology.xml` na interface gráfica e pressionar o botão **Start**.
3.  Após o arranque da emulação, pressione **Enter** no terminal para que o script inicie os binários (mothership, rover, groundcontrol) dentro dos respetivos nós virtuais.

> **Nota:** Para análise de tráfego, pode utilizar o script alternativo `scripts/run-sim-with-wireshark.py`, que lança automaticamente o Wireshark no nó rover1.

**Opção 2: Execução Manual (Acesso direto aos Nós)**
Caso pretenda depurar um componente específico ou correr o sistema passo-a-passo, pode aceder individualmente a cada nó virtual e executar os binários manualmente.

Para preparar o ambiente, comece por iniciar o `core-daemon` (como root/sudo) e a interface gráfica do emulador executando o comando `core-gui`. De seguida, carregue o ficheiro de topologia localizado em `coreemu/topology.xml` e inicie a simulação.

Para aceder e executar os componentes, clique com o botão direito no nó desejado na grelha do CORE e selecione **Shell** (ou CSH/Bash). Execute os comandos abaixo dentro das janelas de terminal que se abrirem.

**Nó mothership (Servidor Central):**

```bash
cd /home/space-mission  # Ajuste o caminho conforme necessário
./bin/mothership/mothership \
    -ts-addr=:9001 -ml-addr=:9002 -api-addr=:9003 \
    load assets/missions/file1.txt
```

**Nó ground (Interface de Controlo):**

```bash
cd /home/space-mission
./bin/groundcontrol/groundcontrol \
    -port=:3000 -api-addr=10.0.0.21:9003 \
    -dist=./bin/groundcontrol/dist
```

**Nós rover1, etc. (Clientes):**

```bash
cd /home/space-mission
./bin/rover/rover \
    -id=1 -m-ts-addr=10.0.1.20:9001 \
    -m-ml-addr=10.0.1.20:9002 -r-ml-addr=:9000
```

## 1.5 Utilização da Interface (Ground Control)

Aceda à interface via browser (geralmente `http://localhost:3000`).

**Funcionalidades:**

  * **Dashboard Principal (Grid Map):**
      * Visualização em tempo real da posição dos Rovers.
      * Visualização das áreas de missão (Círculos ou Retângulos).
  * **Painel Esquerdo (Rover Status & Available Fleet):**
      * Lista todos os rovers detetados e a sua telemetria.
      * Permite filtrar por estado (Idle, On Mission, Error, Unknown).
  * **Painel Direito (Mission Log & Control):**
      * Lista as missões e o seu progresso (barra de percentagem).
      * Permite enviar novas missões para a Mothership.

## 1.6 CLI da Mothership

A Mothership possui uma interface de linha de comandos (CLI) interativa que corre no terminal onde o binário foi iniciado.

  * `help`: Lista comandos disponíveis.
  * `add <id> <task> …`: Adiciona uma missão manualmente.
  * `load <filename>`: Carrega missões a partir de um ficheiro de texto (ex: `assets/missions/file1.txt`).

## 1.7 Resolução de Problemas

  * Cada componente gera ficheiros de log detalhados na pasta de execução (ex: `mothership.log`, `rover_1.log`, `mission_link_1.log`). Verifique estes ficheiros se houver falhas de comunicação.
  * Se obtiver erros de *"address already in use"*, certifique-se de que não tem instâncias antigas a correr (`killall mothership rover groundcontrol`).
  * Se a interface não conectar, verifique se o endereço da API passado ao Ground Control (`-api-addr`) corresponde ao IP onde a Mothership está a correr (Localhost vs IP da rede CORE `10.0.0.21`).
