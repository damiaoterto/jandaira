# 🐝 Jandaira Swarm OS

<p align="center">
  <img src="../jandaira.png" alt="Jandaira Logo"/>
</p>

Un marco de **múltiples agentes autónomos** escrito en Go, inspirado en la inteligencia colectiva de la abeja nativa sudamericana _Melipona subnitida_ — la **Jandaíra**.

---

> 🌐 [English](README.en.md) · [Português](../README.md) · [中文](README.zh.md) · [Русский](README.ru.md) · **Español**

---

## 📖 ¿Por qué "Jandaira"?

La **Jandaíra** es una abeja sin aguijón. Pequeña, resiliente y extraordinariamente cooperativa — no necesita de un líder centralizado para construir una colmena funcional. Cada obrera conoce su papel, ejecuta su tarea con autonomía y devuelve el resultado a la comunidad.

Este es exactamente el modelo de arquitectura que el proyecto implementa:

- La **Reina (`Queen`)** no ejecuta tareas — ella orquesta, valida políticas y garantiza la seguridad.
- Las **Especialistas (`Specialists`)** son agentes ligeros con herramientas restringidas, operando en silos aislados.
- El **Néctar** es la metáfora para el presupuesto de tokens: cada agente consume néctar; cuando se acaba, la colmena se detiene.
- El **Panal (`Honeycomb`)** es la memoria vectorial compartida — el conocimiento colectivo que persiste entre misiones, almacenado en ChromaDB.
- El **Apicultor** es el humano en el bucle: aprueba o bloquea cualquier acción antes de que la IA la ejecute.

---

## 🏗️ Arquitectura

### Visión general del flujo

```
┌─────────────────────────────────────────────────────────────────┐
│                   API REST + WebSocket (:8080)                   │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │  👤 El cliente envía el objetivo vía POST /api/dispatch  │   │
│  └─────────────────────────────────────────────────────────┘   │
└──────────────────────────┬──────────────────────────────────────┘
                           │ DispatchWorkflow()
                           ▼
┌─────────────────────────────────────────────────────────────────┐
│                    Reina (Orquestradora)                          │
│  ┌──────────────┐   ┌─────────────┐   ┌──────────────────────┐  │
│  │  GroupQueue  │   │   Política  │   │   NectarUsage ($$)   │  │
│  │  (FIFO, N=3) │   │ (aislado,   │   │   Presupuesto tokens │  │
│  │              │   │  aprobación)│   │   por enjambre       │  │
│  └──────────────┘   └─────────────┘   └──────────────────────┘  │
└──────────────────────────┬──────────────────────────────────────┘
                           │ Pipeline (Paso del Testigo)
          ┌────────────────┴─────────────────┐
          ▼                                  ▼
┌──────────────────────┐          ┌──────────────────────┐
│  Especialista #1     │  ctx     │  Especialista #2     │
│  "Desarrolladora"    │ ──────►  │  "Auditora"          │
│  Tools: write_file   │          │  Tools: execute_code │
│         search_mem   │          │         read_file    │
└──────────┬───────────┘          └──────────┬───────────┘
           │                                 │
           ▼                                 ▼
┌──────────────────────────────────────────────────────────┐
│                   🔐 Capa de Seguridad                    │
│   Carga útil cifrada (AES-GCM) entre cada paso           │
│   — el contexto nunca viaja en texto plano               │
└──────────────────────────────────────────────────────────┘
           │
           ▼
┌──────────────────────────────────────────────────────────┐
│              👨‍🌾 Apicultor (Humano en el Bucle)           │
│   RequiresApproval=true → WS envía approval_request      │
│   approved=true → autoriza │ approved=false → bloquea    │
└──────────────────────────────────────────────────────────┘
           │
           ▼
┌──────────────────────────────────────────────────────────┐
│                   🍯 Panal (ChromaDB)                     │
│   El resultado del flujo se inserta e indexa             │
│   Memoria a largo plazo compartida entre misiones        │
└──────────────────────────────────────────────────────────┘
```

### Mapa de paquetes

```
jandaira/
├── cmd/
│   └── api/
│       └── main.go          # Punto de entrada: servidor HTTP + WebSocket
│
└── internal/
    ├── brain/               # Contratos de IA (Brain, Honeycomb)
    │   ├── open_ai.go       # Implementación OpenAI (Chat + Embed)
    │   ├── memory.go        # Interfaz Honeycomb + LocalVectorDB
    │   └── chroma.go        # Implementación ChromaDB (ChromaHoneycomb)
    │
    ├── queue/               # Planificador FIFO con concurrencia limitada
    │   └── group_queue.go
    │
    ├── security/            # Cifrado de cargas entre agentes
    │   ├── crypto.go        # AES-GCM Seal/Open + generación de claves
    │   ├── vault.go         # Almacén local de secretos
    │   └── sandbox.go       # Sandbox de ejecución
    │
    ├── swarm/               # Núcleo del sistema de agentes
    │   └── queen.go         # Orquestradora: políticas, HIL, pipeline
    │
    ├── tool/                # Herramientas disponibles para los agentes
    │   ├── list_directory.go
    │   ├── search_memory.go # search_memory + store_memory
    │   └── wasm.go          # Sandbox de ejecución via wazero
    │
    ├── api/                 # Handlers HTTP y WebSocket
    ├── config/
    ├── database/
    ├── i18n/
    ├── model/
    ├── prompt/
    ├── repository/
    └── service/
```

---

## 🚀 Tutorial de Uso

### Requisitos Previos

```bash
# Go 1.22 o superior
go version

# Docker (para ChromaDB)
docker --version

# Clave de OpenAI
export OPENAI_API_KEY="sk-..."
```

### Iniciar ChromaDB

```bash
# Directamente con Docker
docker run -d --name chroma -p 8000:8000 chromadb/chroma:latest

# O usando el docker-compose del proyecto
docker compose up -d
```

Por defecto el servidor se conecta a `http://localhost:8000`. Para usar otra dirección:

```bash
export CHROMA_URL="http://mi-chroma:8000"
```

### Instalación

#### Opción 1 — Compilar desde el código fuente

```bash
git clone https://github.com/damiaoterto/jandaira.git
cd jandaira
go mod tidy
go build -o jandaira-api ./cmd/api/
```

#### Opción 2 — Ejecutar directamente

```bash
go run ./cmd/api/main.go --port 8080
```

### Ejecutar la colmena

```bash
./jandaira-api --port 8080
```

El servidor estará disponible en `http://localhost:8080`. Monitorea eventos en tiempo real via WebSocket en `ws://localhost:8080/ws`.

### Ejemplo: crear y probar un archivo Go

1. Envía el objetivo via `POST /api/dispatch`:

   ```bash
   curl -X POST http://localhost:8080/api/dispatch \
     -H "Content-Type: application/json" \
     -d '{"goal": "Crea un archivo Go llamado suma.go que sume dos números", "group_id": "enxame-alfa"}'
   ```

2. La Reina distribuye la tarea a la pipeline de Especialistas.

3. Sigue el progreso via WebSocket:

   ```json
   { "type": "agent_change", "agent": "Desarrolladora Wasm" }
   { "type": "tool_start",   "tool": "write_file", "args": "{...}" }
   { "type": "result",       "message": "# Informe Final\n..." }
   ```

4. Si `RequiresApproval: true`, el **modo Apicultor** se activa via WebSocket:

   ```json
   // Servidor envía:
   { "type": "approval_request", "id": "req-1712345678901", "tool": "write_file", "args": "{...}" }

   // Cliente responde:
   { "type": "approve", "id": "req-1712345678901", "approved": true }
   ```

### Herramientas disponibles

| Herramienta | Descripción |
|---|---|
| `list_directory` | Lista archivos y carpetas de un directorio |
| `read_file` | Lee el contenido de un archivo |
| `write_file` | Crea o sobreescribe un archivo |
| `execute_code` | Ejecuta código en sandbox Wasm aislado |
| `search_memory` | Búsqueda semántica en la memoria vectorial (ChromaDB) |
| `store_memory` | Guarda conocimiento en la memoria vectorial |

---

## 🔐 Seguridad y Vault

Cada "paso del testigo" entre Especialistas está **cifrado de extremo a extremo con AES-GCM**.
Además, las claves y accesos son gestionados localmente usando el paquete `internal/security/vault.go`, protegiendo tu clave API de accesos por otros procesos o usuarios no autorizados.

---

## 🌐 API Reference

### Rutas REST

| Método | Ruta | Descripción |
|---|---|---|
| `POST` | `/api/dispatch` | Envía un objetivo al enjambre |
| `GET` | `/api/tools` | Lista todas las herramientas disponibles |
| `GET` | `/api/agents` | Lista los especialistas del flujo configurado |
| `GET` | `/ws` | Abre una conexión WebSocket para eventos en tiempo real |

### Eventos WebSocket (`/ws`)

#### Servidor → Frontend

| `type` | Cuándo se dispara | Campos relevantes |
|---|---|---|
| `status` | Mensajes de progreso de la Reina | `message` |
| `agent_change` | Un especialista toma el control | `agent` |
| `tool_start` | Una herramienta está a punto de ejecutarse | `agent`, `tool`, `args` |
| `approval_request` | La IA quiere usar una herramienta bloqueada | `id`, `tool`, `args` |
| `result` | Informe final del flujo | `message` |
| `error` | Fallo o timeout | `message` |

#### Frontend → Servidor

```json
{ "type": "approve", "id": "req-1712345678901", "approved": true }
{ "type": "approve", "id": "req-1712345678901", "approved": false }
```

---

## ⚖️ Licencia y Uso Comercial (Licencia Dual)

**Jandaira Swarm OS** se distribuye bajo un modelo de licencia dual.

* **Uso de Código Abierto (AGPLv3):** El código fuente está disponible gratuitamente bajo la licencia [GNU Affero General Public License v3.0](../LICENCE).
* **Uso Comercial Corporativo:** Para empresas que deseen integrar Jandaira en productos propietarios sin obligación de abrir su código fuente, ofrecemos una **Licencia Comercial**.

---

## 🤝 Contribuyendo

¡Pull Requests son bienvenidos! Abre un caso (issue) describiendo la funcionalidad antes de iniciar grandes cambios.

---

_Jandaira: Autonomía, Seguridad y la Fuerza del Enjambre._ 🐝
