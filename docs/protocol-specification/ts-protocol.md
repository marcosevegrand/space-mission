# Protocolo TelemetryStream (TL)

Na implementação do **TelemetryStream (TS)**, o objetivo foi criar um **protocolo aplicacional fiável sobre TCP** que assegurasse a transmissão contínua e estruturada dos dados de telemetria dos rovers para a Nave-Mãe. Este protocolo complementa o MissionLink, fornecendo visibilidade em tempo real sobre o estado operativo de cada unidade.

Adotámos uma **arquitetura cliente-servidor persistente**, com a **Nave-Mãe** a atuar como servidor TCP e cada **rover** como cliente. Assim que um rover inicia operação, estabelece uma conexão estável com o servidor e envia os primeiros dados de identificação e estado. A ligação permanece aberta enquanto o rover estiver ativo, assegurando comunicação contínua e bidirecional.

As mensagens foram estruturadas em **formato JSON**, pensadas para integração direta com a API de Observação. Cada mensagem contém dados essenciais:

- *rover_id*: identificação única;
- *timestamp*: instante da amostragem;
- *position*: coordenadas (x, y, z);
- *operational_state:* estado atual (em missão, parado, erro, etc.);
- *battery_percent*, *velocity*, *temperature_celsius* e outros parâmetros adicionais de estado ou ambientais.

Exemplo de mensagem TS:

```json
{
  "rover_id": "R-003",
  "timestamp": 1729503550,
  "position": {"x": 512.3, "y": 108.7, "z": 0},
  "operational_state": "IN_MISSION",
  "battery_percent": 83,
  "velocity_mps": 1.2,
  "temperature_celsius": 46
}
```

Optámos por **intervalos fixos de envio** de 10 segundos, ajustáveis conforme o estado do rover. A Nave-Mãe processa cada mensagem recebida, atualiza o estado e armazena a última telemetria válida por rover. Para permitir **framing eficiente** no TCP, acrescentámos um campo de tamanho em bytes antes do payload JSON, garantindo delimitação correta das mensagens e leitura contínua sem ambiguidades.

A gestão de múltiplas ligações foi resolvida através de **programação concorrente**: um *thread pool* no servidor aceita conexões simultâneas e trata cada rover de modo independente, atualizando estruturas de dados em memória partilhada sincronizada. Em caso de falha na ligação, o rover tenta reconectar automaticamente com *backoff exponencial*, preservando o contexto anterior.
