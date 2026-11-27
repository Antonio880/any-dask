# AnyDask — Especificação do Protocolo Binário de Input (`DataChannel: input`)

Canal configurado como **Unordered** (`ordered: false`, `maxRetransmits: 0`) para eliminar *Head-of-Line Blocking* e manter latência de input abaixo de `5ms`.

Todos os inteiros multi-byte usam **Big Endian (Network Byte Order)**.

## Layout dos Pacotes

| Byte 0 (`type`) | Evento | Tamanho | Layout dos Bytes Restantes |
| :--- | :--- | :--- | :--- |
| `0x01` | `MOUSE_MOVE_REL` | `6 bytes` | `int16 dx` (bytes 1..2), `int16 dy` (bytes 3..4), `uint8 flags` (byte 5) |
| `0x02` | `MOUSE_MOVE_ABS` | `6 bytes` | `uint16 x_norm` (`0..65535`, bytes 1..2), `uint16 y_norm` (`0..65535`, bytes 3..4), `uint8 flags` (byte 5) |
| `0x03` | `MOUSE_BUTTON` | `6 bytes` | `uint8 button` (`1=Left, 2=Right, 3=Middle`), `uint8 action` (`0=Up, 1=Down, 2=Click, 3=DblClick`), `uint16 x_norm` (opcional `0xFFFF` = usar posição atual), `uint16 y_norm` |
| `0x04` | `MOUSE_SCROLL` | `6 bytes` | `int16 dx` (bytes 1..2), `int16 dy` (bytes 3..4), `uint8 flags` (byte 5) |

## Mapeamento de Coordenadas Absolutas
- `x_norm = round(clamp(x_ratio, 0, 1) * 65535)`
- `y_norm = round(clamp(y_ratio, 0, 1) * 65535)`
- No Host, `CoordinateMapper` projeta `0..65535` para os limites lógicos/físicos da tela capturada.
