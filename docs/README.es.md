# 🐝 Jandaira Swarm OS

<p align="center">
  <img src="../jandaira.png" alt="Jandaira Logo"/>
</p>

Un framework simple y poderoso de **múltiples agentes autónomos** escrito en Go. Inspirado en la abeja nativa brasileña **Jandaíra**, permite crear "colmenas" de IAs que trabajan juntas de forma segura y eficiente.

> [English](README.en.md) · [Português](../README.md) · **Español** · [中文](README.zh.md) · [Русский](README.ru.md)

---

## 🚀 Configuración e Instalación (¡Empieza aquí!)

¡Ejecutar Jandaira es muy fácil! El sistema ya viene con su propia base de datos incorporada, por lo que **no necesitas Docker** si solo vas a ejecutar la API.

### 1. Requisitos Previos
* Tener [Go](https://go.dev/) (versión 1.22 o superior) instalado.
* Una clave de API de OpenAI (o compatible).



### 2. Elige cómo instalar

**Opción A: Instalación Automática (Linux/macOS - Más Fácil)**
Descarga y configura todo por ti automáticamente.
```bash
curl -fsSL https://github.com/damiaoterto/jandaira/releases/latest/download/install.sh | sudo bash
```
*Panel Frontend: `http://localhost:9000` | API: `http://localhost:8080`*

**Opción B: Vía Docker (Sistema Completo)**
Ideal si quieres el Backend + Frontend ejecutándose juntos sin instalar nada en tu PC.
```bash
docker pull ghcr.io/damiaoterto/jandaira:latest
docker run -d -p 8080:8080/tcp -p 9000:9000/tcp ghcr.io/damiaoterto/jandaira:latest
```

**Opción C: Compilando desde el Código Fuente**
Para quienes quieren modificar o contribuir al proyecto.
```bash
git clone https://github.com/damiaoterto/jandaira.git
cd jandaira
go mod tidy
go run ./cmd/api/main.go --port 8080
```

**Opción D: Instalación en Windows**
Descarga el instalador desde la [página de releases](https://github.com/damiaoterto/jandaira/releases/latest) y ejecútalo como Administrador en PowerShell:
```powershell
powershell.exe -ExecutionPolicy Bypass -File .\install-windows.ps1
```

### 3. Probando tu Colmena
Después de iniciar el servidor (estará ejecutándose en el puerto 8080), puedes enviar un objetivo a la IA:

```bash
curl -X POST http://localhost:8080/api/dispatch \
  -H "Content-Type: application/json" \
  -d '{"goal": "Crea un archivo Go llamado suma.go que sume dos números", "group_id": "enjambre-alfa"}'
```
Puedes monitorear lo que hace la IA en tiempo real a través del WebSocket: `ws://localhost:8080/ws`.

---

## ⚖️ Licencia de Uso (Explicada de Forma Simple)

El **Jandaira Swarm OS** posee un modelo de licencia doble para ser justo con la comunidad y con las empresas.

1. **Para la Comunidad (100% Gratis - AGPLv3):**
   Puedes descargar, usar, modificar y distribuir Jandaira gratis. 
   ⚠️ **La Regla:** Si usas Jandaira para crear un producto, proyecto o servicio web, **estás obligado a dejar el código fuente de tu proyecto abierto y público** para todo el mundo.

2. **Para Empresas (Licencia Comercial):**
   ¿Quieres usar Jandaira en tu empresa o crear un producto cerrado, pero **no quieres** compartir el código fuente de tu sistema? 
   ✅ **La Solución:** Vendemos una **Licencia Comercial**. Con ella, puedes usar Jandaira en proyectos privados sin la obligación de abrir tu código. ¡Contáctanos!

---

## 📖 ¿Qué es Jandaira?

Inspirado en la abeja brasileña que trabaja en conjunto sin necesitar un líder central, nuestro sistema divide el trabajo entre varios "agentes IA":

- **Reina (`Queen`):** No ejecuta tareas. Ella solo organiza, administra el "néctar" (sus tokens/dinero) y garantiza la seguridad.
- **Especialistas (`Specialists`):** Son las obreras. Cada agente tiene una función específica (ej. desarrollador, auditor) y herramientas limitadas para ejecutar su trabajo.
- **Apicultor (¡Tú!):** El humano en control. La IA puede pedir tu aprobación antes de ejecutar acciones peligrosas.

---

## 🏗️ Cómo Funciona la Arquitectura

### El Flujo Principal

```mermaid
sequenceDiagram
    participant C as 👤 Cliente
    participant Q as 👑 Queen (API)
    participant E1 as 🐝 Especialista 1
    participant E2 as 🐝 Especialista 2
    participant H as 👨‍🌾 Apicultor
    participant BD as 🍯 VectorEngine

    C->>Q: Envía Objetivo (POST /api/dispatch)
    Q->>Q: Valida Política y Néctar (Tokens)
    Q->>E1: Pasa el relevo (Inicia pipeline)
    E1->>E1: Ejecuta Tarea (ej. escribe código)
    E1-->>E2: Envía resultado (Payload Encriptado AES-GCM)
    E2->>E2: Ejecuta Tarea (ej. audita código)
    E2->>H: ¿Pide aprobación? (Approval Request)
    H-->>E2: Humano aprueba acción
    E2->>BD: Guarda en Base Vectorial
    Q-->>C: Devuelve Resultado Final
```

### Cómo Funciona la Memoria (Corto y Largo Plazo)

Para no gastar muchos tokens y mantener a la IA inteligente a lo largo del tiempo, dividimos la memoria en dos:

```mermaid
graph TD
    M1[💬 Nueva Mensaje] --> STM[🧠 Memoria a Corto Plazo RAM]
    STM -->|¿Llegó al límite?| COMPACT[LLM hace un Resumen]
    COMPACT --> LTM[(🍯 Base Vectorial Largo Plazo)]
    
    style STM fill:#f9f,stroke:#333,stroke-width:2px
    style LTM fill:#bbf,stroke:#333,stroke-width:2px
```

### Grafo de Conocimiento (IA aprendiendo sola)

¡La Reina aprende de misiones pasadas! Si un agente fue bueno "analizando ventas", lo llamará nuevamente en el futuro.

```mermaid
graph LR
    O[Objetivo: Analizar Ventas] --> R{Reina consulta Grafo}
    R -->|Encuentra Perfil| A1((Analista de Ventas))
    A1 -->|Experto en| T[Tema: datos de ventas]
    R -->|Arma equipo según experiencia| E[Enjambre Final]
```

---

## 🔌 Integraciones MCP (Model Context Protocol)

Jandaira admite de forma nativa la conexión de cada colmena a uno o más servidores MCP externos. La relación es **muchos a muchos**: una colmena puede usar varios servidores MCP y un servidor puede compartirse entre varias colmenas.

**Transportes compatibles:**
- **Stdio** — lanza el servidor MCP como subproceso (ej. `npx -y @mcp/server-postgres`). Ideal para bases de datos, sistemas de archivos y herramientas locales.
- **SSE** — conecta a servidores MCP remotos vía HTTP+SSE. Ideal para integraciones en la nube.

```bash
# 1. Registra un servidor MCP de PostgreSQL
curl -X POST http://localhost:8080/api/mcp-servers \
  -H "Content-Type: application/json" \
  -d '{
    "name": "postgres-analytics",
    "transport": "stdio",
    "command": "npx -y @modelcontextprotocol/server-postgres postgres://user:pass@localhost/db",
    "active": true
  }'

# 2. Asócialo a una colmena
curl -X POST http://localhost:8080/api/colmeias/{id}/mcp-servers \
  -H "Content-Type: application/json" \
  -d '{"mcp_server_id": "{server-id}"}'

# 3. Despacha — las herramientas MCP se cargan automáticamente
curl -X POST http://localhost:8080/api/colmeias/{id}/dispatch \
  -H "Content-Type: application/json" \
  -d '{"goal": "Lista los pedidos del último mes y calcula el ingreso total"}'
```

> Documentación completa (en inglés): [`docs/mcp-engine.md`](mcp-engine.md)

---

## 🪝 Webhook Engine (Integraciones fáciles)

Puedes conectar Jandaira a GitHub, Slack, etc. La IA se activa automáticamente cuando ocurre un evento.

```mermaid
graph LR
    GH[GitHub / CI-CD] -->|POST Payload JSON| W[Webhook Engine]
    W -->|Valida Contraseña y Transforma JSON| Q[👑 Reina]
    Q -->|Ejecuta la Misión| E[🐝 Enjambre]
    E -->|Outbound Webhook| D[Discord / Slack]
```

---

## ⚡ ¿Por qué elegir Go en lugar de Python?

| Comparación                 | NanoClaw (Python)         | Jandaira (Go) 🏆                       |
| --------------------------- | ------------------------- | -------------------------------------- |
| **Rendimiento**             | Pesado, exige hilos       | Súper ligero con Goroutines nativas    |
| **Instalación**             | Requiere dependencias/Docker| ¡Un solo archivo ejecutable!           |
| **Seguridad entre Agentes** | No existe                 | Encriptación nativa AES-GCM            |
| **Base de Datos IA**        | Requiere serv. externos   | ¡Base Vectorial (HNSW) incorporada!    |
| **Aprobación Humana**       | Arreglos externos         | Nativo vía WebSocket en tiempo real    |

---

## 🌐 Referencia Rápida de la API

| Acción | Ruta HTTP | Descripción |
| --- | --- | --- |
| **Despachar Misión** | `POST /api/dispatch` | Envía un trabajo a la colmena. |
| **Listar Herramientas** | `GET /api/tools` | Mira lo que las IAs pueden hacer. |
| **Tiempo Real** | `GET /ws` | WebSocket para seguir a las IAs y aprobar acciones. |
| **Webhooks** | `POST /api/webhooks/:slug` | Dispara un evento externo. |
| **Servidores MCP** | `GET/POST /api/mcp-servers` | Gestiona configuraciones de servidores MCP. |
| **MCP de la colmena** | `GET/POST /api/colmeias/:id/mcp-servers` | Asocia / desconecta servidores MCP de una colmena. |

---

## 🤝 Contribuir

¡Los Pull Requests son muy bienvenidos! Por favor, abre un issue describiendo lo que deseas mejorar antes de empezar a programar.

_Jandaira: Autonomía, Seguridad y la Fuerza del Enjambre Brasileño._ 🐝
