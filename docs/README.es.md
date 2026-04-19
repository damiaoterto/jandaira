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
- El **Panal (`Honeycomb`)** es el sistema de memoria persistente de dos niveles: `ShortTermMemory` mantiene el contexto reciente en RAM con expiración automática por TTL; Qdrant archiva el conocimiento consolidado a largo plazo como vectores.
- El **Grafo de Conocimiento (`KnowledgeGraph`)** mapea relaciones entre agentes, temas y herramientas — la Reina lo consulta antes de cada misión para reutilizar perfiles de especialistas que ya tuvieron éxito en objetivos similares.
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
│                   🍯 Panal (Qdrant)                     │
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
    ├── brain/               # Sistema nervioso del enjambre
    │   ├── open_ai.go       # Brain: Chat + Embed vía OpenAI
    │   ├── memory.go        # Honeycomb: interfaz vectorial + LocalVectorDB
    │   ├── qdrant.go        # QdrantHoneycomb: backend Qdrant
    │   ├── graph.go         # KnowledgeGraph: grafo agente ↔ tema (GraphRAG)
    │   ├── short_term.go    # ShortTermMemory: buffer TTL + compactación automática
    │   └── document.go      # Extracción de texto + chunking (PDF, DOCX, XLSX…)
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

## 🧠 Arquitectura de Memoria

`internal/brain/` va mucho más allá de un almacén vectorial: implementa una jerarquía de memoria de dos niveles con un grafo de conocimiento que crece con cada misión.

### Memoria a Corto Plazo — `ShortTermMemory`

`brain/short_term.go` es un buffer de mensajes con TTL por entrada. Resuelve el problema de desbordamiento de contexto en enjambres de larga duración:

- Cada mensaje recibe un timestamp de expiración al insertarse
- Las entradas expiradas se descartan silenciosamente en el siguiente acceso
- **Compactación automática**: cuando el buffer alcanza `maxEntries`, el LLM resume el historial acumulado en un párrafo denso → el resumen se incrusta y archiva en Qdrant como `short_term_archive` → el buffer RAM se vacía
- `Flush(ctx)` debe llamarse al final de cada sesión para garantizar el archivado completo

```
 Nuevo mensaje insertado
         │
         ▼
┌──────────────────────────────────┐
│      ShortTermMemory (RAM)       │
│  [msg₁ · expira: +30min]        │
│  [msg₂ · expira: +30min]        │
│  ...                             │
│  [msgN · expira: +30min]        │ ← overflow: compact() se dispara
└──────────────────────────────────┘
         │
         ▼
   LLM resume el historial
         │
         ▼
┌──────────────────────────────────┐
│  Qdrant  (Memoria a Largo Plazo)│
│  type: "short_term_archive"      │
└──────────────────────────────────┘
```

### Grafo de Conocimiento — `KnowledgeGraph` (GraphRAG)

`brain/graph.go` implementa un grafo de conocimiento persistido en JSON (`~/.config/jandaira/knowledge_graph.json`) que acumula expertise automáticamente tras cada workflow completado.

**Modelo de datos**

| Elemento | Tipo | Ejemplo |
|---|---|---|
| Perfil de especialista | nodo `agent` | `"Analista de Datos"` |
| Dominio de la misión | nodo `topic` | `"análisis de informe financiero"` |
| Vínculo de expertise | arista `expert_in` | `agent → topic` |

**Ciclo de aprendizaje automático de la Reina**

Después de cada workflow, `registerWorkflowInGraph` registra:
1. Crea/actualiza un nodo `topic` con el objetivo de la misión
2. Para cada especialista del pipeline, crea/actualiza un nodo `agent`
3. Crea aristas `expert_in` vinculando cada agente al tema

Antes de montar el siguiente enjambre, `graphContextForGoal` consulta el grafo e inyecta un bloque **"PAST SPECIALIST KNOWLEDGE"** en el prompt de meta-planificación de la Reina.

Resultado: la Reina diseña enjambres progresivamente mejores con el tiempo, sin llamadas LLM adicionales.

---

## 🚀 Tutorial de Uso

### Requisitos Previos

```bash
# Go 1.22 o superior
go version

# Docker (para Qdrant)
docker --version

# Clave de OpenAI
export OPENAI_API_KEY="sk-..."
```

### Iniciar Qdrant

```bash
# Directamente con Docker
docker run -d --name qdrant -p 6334:6334 qdrant/qdrant:latest

# O usando el docker-compose del proyecto
docker compose up -d
```

Por defecto el servidor se conecta a `localhost:6334`. Para usar otra dirección:

```bash
export QDRANT_HOST="qdrant"  # hostname only, port 6334 (gRPC) used by default
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
| `web_search` | Busca en internet via DuckDuckGo (respuestas directas, definiciones, resúmenes) |
| `search_memory` | Búsqueda semántica en la memoria vectorial (Qdrant) |
| `store_memory` | Guarda conocimiento en la memoria vectorial |

---

## 🔐 Seguridad y Vault

Cada "paso del testigo" entre Especialistas está **cifrado de extremo a extremo con AES-GCM**.
Además, las claves y accesos son gestionados localmente usando el paquete `internal/security/vault.go`, protegiendo tu clave API de accesos por otros procesos o usuarios no autorizados.

---

## 🌐 API Reference

### Rutas REST

#### Configuración y Despacho

| Método | Ruta | Descripción |
|---|---|---|
| `POST` | `/api/setup` | Configura la colmena en la primera ejecución |
| `POST` | `/api/dispatch` | Envía un objetivo al enjambre (sin estado) |
| `GET` | `/api/tools` | Lista todas las herramientas disponibles |
| `GET` | `/ws` | Abre una conexión WebSocket para eventos en tiempo real |

#### Sesiones

| Método | Ruta | Descripción |
|---|---|---|
| `GET` | `/api/sessions` | Lista todas las sesiones |
| `POST` | `/api/sessions` | Crea una nueva sesión |
| `GET` | `/api/sessions/:id` | Obtiene sesión con agentes |
| `DELETE` | `/api/sessions/:id` | Elimina sesión (cascada) |
| `POST` | `/api/sessions/:id/dispatch` | Despacha workflow para la sesión |
| `GET` | `/api/sessions/:id/agents` | Lista agentes de la sesión |
| `POST` | `/api/sessions/:id/documents` | Sube e indexa un documento |

#### Colmenas Persistentes (Colmeias)

Las colmenas son entidades persistentes y con nombre. A diferencia de las sesiones, una colmena puede recibir **múltiples mensajes a lo largo del tiempo**, manteniendo el historial de conversaciones como contexto. Los agentes pueden ser **predefinidos por el usuario** (con prompts y herramientas personalizables, solo cuando `queen_managed=false`) o **ensamblados automáticamente por la Reina** (`queen_managed=true`). Intentar agregar agentes predefinidos a una colmena `queen_managed=true` devuelve `409 Conflict`.

| Método | Ruta | Descripción |
|---|---|---|
| `GET` | `/api/colmeias` | Lista todas las colmenas |
| `POST` | `/api/colmeias` | Crea colmena (`queen_managed: true/false`) |
| `GET` | `/api/colmeias/:id` | Obtiene colmena con agentes |
| `PUT` | `/api/colmeias/:id` | Actualiza colmena |
| `DELETE` | `/api/colmeias/:id` | Elimina colmena (cascada) |
| `POST` | `/api/colmeias/:id/dispatch` | Envía mensaje a la colmena |
| `GET` | `/api/colmeias/:id/historico` | Lista historial de conversaciones |
| `GET` | `/api/colmeias/:id/agentes` | Lista agentes de la colmena |
| `POST` | `/api/colmeias/:id/agentes` | Agrega agente predefinido (`queen_managed=false` requerido) |
| `GET` | `/api/colmeias/:id/agentes/:agentId` | Obtiene agente por ID |
| `PUT` | `/api/colmeias/:id/agentes/:agentId` | Edita nombre, prompt y herramientas |
| `DELETE` | `/api/colmeias/:id/agentes/:agentId` | Elimina agente de la colmena |

**Ejemplo — crear colmena con agentes definidos por el usuario:**

```bash
# 1. Crear colmena
curl -X POST http://localhost:8080/api/colmeias \
  -H "Content-Type: application/json" \
  -d '{"name": "Colmena de Investigación", "queen_managed": false}'

# 2. Agregar agente con prompt personalizado
curl -X POST http://localhost:8080/api/colmeias/{id}/agentes \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Investigador Web",
    "system_prompt": "Eres un especialista en investigación. Usa web_search para recopilar información actualizada.",
    "allowed_tools": ["web_search", "search_memory", "store_memory"]
  }'

# 3. Enviar primer mensaje
curl -X POST http://localhost:8080/api/colmeias/{id}/dispatch \
  -H "Content-Type: application/json" \
  -d '{"goal": "Busca las principales noticias sobre IA de esta semana"}'

# 4. Enviar seguimiento (historial anterior inyectado automáticamente como contexto)
curl -X POST http://localhost:8080/api/colmeias/{id}/dispatch \
  -H "Content-Type: application/json" \
  -d '{"goal": "Con base en la investigación anterior, escribe un resumen ejecutivo"}'
```

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
