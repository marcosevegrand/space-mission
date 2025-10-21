# Protocolo MissionLink (ML)

Na implementação do **MissionLink (ML)**, o foco esteve em criar um **protocolo aplicacional fiável sobre UDP** capaz de gerir comunicações críticas entre a Nave-Mãe e os rovers. A arquitetura seguiu um **modelo cliente-servidor**, em que a **Nave-Mãe** funciona como servidor UDP, centralizando a atribuição e acompanhamento das missões, enquanto cada **rover** atua como cliente.

No fluxo principal, os rovers solicitam missões à Nave-Mãe, que responde com os parâmetros definidos. Cada rover confirma a receção, executa a missão e envia atualizações periódicas de progresso. Criámos um ciclo completo de mensagens com confirmações explícitas (ACKs), retransmissões automáticas e números de sequência únicos para garantir integridade e ordenação mesmo sobre um protocolo não confiável.

As mensagens foram estruturadas em **formato JSON** por motivos de legibilidade e integração simples com a API de Observação. Cada pacote inclui:
- *seq_num* para sequência e controlo de duplicação;
- *type* para distinguir o tipo de mensagem (solicitação, atribuição, atualização, confirmação, etc.);
- Dados da missão (id, área, tarefa, duração, frequência de atualização).

Um exemplo de missão enviada pela Nave-Mãe:

```json
{
  "type": "MISSION_ASSIGNED",
  "seq_num": 42,
  "mission": {
    "id": "M-001",
    "area": {"x1":100,"y1":200,"x2":300,"y2":400},
    "task": "CAPTURA_IMAGENS",
    "duration_sec": 1800,
    "update_interval_sec": 120
  }
}
```

Nos mecanismos de fiabilidade, desenvolvemos:
- Retransmissão adaptativa com timeouts progressivos baseados em valores médios de RTT;
- ACKs explícitos para mensagens críticas;
- Verificação de integridade com checksum CRC32;
- Cache de sequência recente no recetor para deteção de duplicados.
