# 🐝 Jandaira 蜂群操作系统

<p align="center">
  <img src="../jandaira.png" alt="Jandaira Logo"/>
</p>

一个用 Go 语言编写的**多智能体自主框架**，灵感来自巴西本土蜂 *Melipona subnitida*——**Jandaíra** 蜂的集体智慧。

---

> 🌐 [English](README.en.md) · [Português](../README.md) · **中文** · [Русский](README.ru.md)

---

## 📖 为什么叫"Jandaira"？

**Jandaíra**（*Melipona subnitida*）是一种特产于巴西 Caatinga 生物群落的无刺蜂。它小巧、坚韧、协作能力出众——无需中央领导者便能建造出高效的蜂巢。每只工蜂都知道自己的职责，自主执行任务，并将结果反馈给集体。

这正是本项目所实现的架构模型：

- **蜂王（`Queen`）** 不执行任务——她负责编排、验证策略和确保安全。
- **专家智能体（`Specialists`）** 是轻量级智能体，工具受限，在隔离的沙箱中执行。
- **花蜜（Nectar）** 是 Token 预算的隐喻：每个智能体消耗花蜜；花蜜耗尽，蜂巢停止运作。
- **蜂巢（`Honeycomb`）** 是共享的向量记忆——在任务之间持久化的集体知识，存储于 ChromaDB。
- **养蜂人（Beekeeper）** 是回路中的人类：可以在 AI 执行任何操作之前批准或阻止它。

---

## 🏗️ 架构

### 流程概览

```
┌─────────────────────────────────────────────────────────────────┐
│                   API REST + WebSocket (:8080)                   │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │  👤 客户端通过 POST /api/dispatch 发送目标               │   │
│  └─────────────────────────────────────────────────────────┘   │
└──────────────────────────┬──────────────────────────────────────┘
                           │ DispatchWorkflow()
                           ▼
┌─────────────────────────────────────────────────────────────────┐
│                    蜂王（编排者）                                  │
│                                                                  │
│  ┌──────────────┐   ┌─────────────┐   ┌──────────────────────┐  │
│  │  GroupQueue  │   │   策略      │   │   NectarUsage ($$)   │  │
│  │  (FIFO, N=3) │   │ (隔离,      │   │   Token 预算         │  │
│  │              │   │  审批)      │   │   每个蜂群           │  │
│  └──────────────┘   └─────────────┘   └──────────────────────┘  │
└──────────────────────────┬──────────────────────────────────────┘
                           │ 管道（接力赛）
          ┌────────────────┴─────────────────┐
          ▼                                  ▼
┌──────────────────────┐          ┌──────────────────────┐
│  专家 #1             │  ctx     │  专家 #2             │
│  "开发者"            │ ──────►  │  "审计员"            │
│  工具: write_file    │          │  工具: execute_code  │
│         search_mem   │          │         read_file    │
└──────────┬───────────┘          └──────────┬───────────┘
           │                                 │
           ▼                                 ▼
┌──────────────────────────────────────────────────────────┐
│                   🔐 安全层                               │
│   每次接力之间的加密载荷（AES-GCM）——上下文从不以         │
│   纯文本传输                                             │
└──────────────────────────────────────────────────────────┘
           │
           ▼
┌──────────────────────────────────────────────────────────┐
│              👨‍🌾 养蜂人（人在回路中）                    │
│   RequiresApproval=true → WS 发送 approval_request       │
│   approved=true → 授权 │ approved=false → 阻止           │
└──────────────────────────────────────────────────────────┘
           │
           ▼
┌──────────────────────────────────────────────────────────┐
│                   🍯 蜂巢（ChromaDB）                     │
│   工作流结果被嵌入并索引                                  │
│   任务之间共享的长期记忆                                  │
└──────────────────────────────────────────────────────────┘
```

### 包结构

```
jandaira/
├── cmd/
│   └── api/
│       └── main.go          # 入口点：HTTP + WebSocket 服务器
│
└── internal/
    ├── brain/               # AI 契约（Brain, Honeycomb）
    │   ├── open_ai.go       # OpenAI 实现（Chat + Embed）
    │   ├── memory.go        # Honeycomb 接口 + LocalVectorDB
    │   └── chroma.go        # ChromaDB 实现（ChromaHoneycomb）
    │
    ├── queue/               # 具有有限并发的 FIFO 调度器
    │   └── group_queue.go   # GroupQueue：每组 N 个 worker
    │
    ├── security/            # 智能体间载荷加密
    │   ├── crypto.go        # AES-GCM 封装/解封 + 密钥生成
    │   ├── vault.go         # 本地密钥库
    │   └── sandbox.go       # 执行沙箱
    │
    ├── swarm/               # 智能体系统核心
    │   └── queen.go         # 编排者：策略、HIL、管道
    │
    ├── tool/                # 智能体可用工具
    │   ├── list_directory.go
    │   ├── search_memory.go # search_memory + store_memory
    │   └── wasm.go          # 通过 wazero 的执行沙箱
    │
    ├── api/                 # HTTP 处理器和 WebSocket
    ├── config/              # 应用配置
    ├── database/            # SQLite 连接
    ├── i18n/                # 国际化
    ├── model/               # 数据模型
    ├── prompt/              # 提示词模板
    ├── repository/          # 数据访问
    └── service/             # 业务逻辑
```

---

## ⚡ 与 NanoClaw 的差异对比

| 特性 | NanoClaw (Python) | Jandaira (Go) |
|---|---|---|
| **语言** | Python | Go 1.22+ |
| **并发** | `asyncio` / 线程 | 原生 Goroutines + channels |
| **智能体隔离** | Docker 容器 | 通过 `wazero` 的 Wasm（无需 Docker） |
| **IPC 通信** | 磁盘上的 JSON / Redis | 类型化共享内存 |
| **智能体间加密** | ❌ 不存在 | ✅ 每次接力之间的 AES-GCM |
| **人在回路中** | 可选 / 外部 | ✅ 原生：养蜂人模式（通过 WebSocket） |
| **Token 预算** | 手动 | ✅ 每个蜂群自动 `NectarUsage` |
| **向量记忆** | Pinecone / 外部 | ✅ ChromaDB via Docker |
| **界面** | 不存在 | ✅ REST API + WebSocket |
| **IPC 延迟** | 高（磁盘/网络 I/O） | 最小（内存） |

### 为什么 Go 在这里优于 Python？

1. **Goroutine 比线程更便宜** — 运行 100 个并发智能体的成本远低于使用 Python `asyncio` 或 `threading` 的成本。
2. **静态二进制** — 零运行时依赖。`go build` 生成的可执行文件可在任何 Linux 上运行，无需安装任何东西。
3. **没有 GIL** — Python 有全局解释器锁；Go 真正地在多核上并行化。
4. **`wazero` 是 100% Go** — Wasm 运行时不需要 CGo、Docker 或外部系统。智能体在同一进程内的沙箱中运行。

---

## 🚀 使用教程

### 前提条件

```bash
# Go 1.22 或更高版本
go version

# Docker（用于 ChromaDB）
docker --version

# OpenAI API 密钥
export OPENAI_API_KEY="sk-..."
```

### 启动 ChromaDB

```bash
# 直接通过 Docker
docker run -d --name chroma -p 8000:8000 chromadb/chroma:latest

# 或使用项目的 docker-compose
docker compose up -d
```

服务器默认连接到 `http://localhost:8000`。使用其他地址：

```bash
export CHROMA_URL="http://my-chroma:8000"
```

### 安装

#### 方式 1 — 从源码构建

```bash
git clone https://github.com/damiaoterto/jandaira.git
cd jandaira

# 下载依赖
go mod tidy

# 构建 API 服务器
go build -o jandaira-api ./cmd/api/
```

#### 方式 2 — 直接运行

```bash
go run ./cmd/api/main.go --port 8080
```

### 运行蜂巢

```bash
./jandaira-api --port 8080
```

服务器将在 `http://localhost:8080` 上可用。通过 WebSocket `ws://localhost:8080/ws` 实时监控事件。

### 示例：创建并测试一个 Go 文件

1. 通过 `POST /api/dispatch` 发送目标：

   ```bash
   curl -X POST http://localhost:8080/api/dispatch \
     -H "Content-Type: application/json" \
     -d '{"goal": "创建一个名为 sum.go 的 Go 文件，将两个数字相加", "group_id": "enxame-alfa"}'
   ```

2. 蜂王将任务分配给专家管道：
   - **Wasm 开发者** → 使用 `write_file` 编写 `sum.go`
   - **质量审计员** → 使用 `execute_code` 执行代码并生成报告

3. 通过 WebSocket 跟踪进度：

   ```json
   { "type": "agent_change", "agent": "Wasm 开发者" }
   { "type": "tool_start",   "agent": "Wasm 开发者", "tool": "write_file", "args": "{...}" }
   { "type": "result",       "message": "# 最终报告\n..." }
   ```

4. 如果 `RequiresApproval: true`，**养蜂人模式** 激活。服务器通过 WebSocket 发送 `approval_request` 并等待响应：

   ```json
   // 服务器发送：
   { "type": "approval_request", "id": "req-1712345678901", "tool": "write_file", "args": "{...}" }

   // 客户端响应：
   { "type": "approve", "id": "req-1712345678901", "approved": true }
   ```

5. 最后，结果保存到 ChromaDB 向量记忆中以供将来使用。

### 配置你自己的蜂群

编辑 `cmd/api/main.go` 定义蜂群策略：

```go
queen.RegisterSwarm("my-swarm", swarm.Policy{
    MaxNectar:        50000,  // Token 预算
    Isolate:          true,   // 每组隔离上下文
    RequiresApproval: true,   // 养蜂人模式（HIL）
})
```

### 可用工具

| 工具 | 描述 |
|---|---|
| `list_directory` | 列出目录中的文件和文件夹 |
| `read_file` | 读取文件内容 |
| `write_file` | 创建或覆盖文件 |
| `execute_code` | 在隔离的 Wasm 沙箱中执行代码 |
| `web_search` | 通过 DuckDuckGo 搜索网络（直接答案、定义、摘要） |
| `search_memory` | 在蜂巢向量记忆（ChromaDB）中进行语义搜索 |
| `store_memory` | 将知识保存到向量记忆 |

---

## 🔐 安全性

专家之间每次"接力"都是**用 AES-GCM 加密的**：

1. 在每个工作流开始时生成一个临时会话密钥
2. 累积的上下文**在发送给下一个专家之前被加密**
3. 专家接收加密载荷，解密，处理，并**重新加密**其响应
4. 没有上下文在智能体之间以纯文本传输

这模拟了一个安全的 IPC 通道，即使一个智能体被攻破，它也无法读取管道中其他智能体的历史记录。

---

## 🌐 API 参考

使用 `./jandaira-api --port 8080` 启动 HTTP 服务器，提供以下路由：

### REST 路由

| 方法 | 路由 | 描述 |
|---|---|---|
| `POST` | `/api/dispatch` | 向蜂群提交目标并执行 |
| `GET` | `/api/tools` | 列出所有可用工具及其参数 |
| `GET` | `/api/agents` | 列出已配置工作流中的专家 |
| `GET` | `/ws` | 打开 WebSocket 连接以接收实时事件 |

#### `POST /api/dispatch`

```json
// 请求
{ "goal": "创建一个将两个数字相加的 Go 文件", "group_id": "enxame-alfa" }

// 响应 202
{ "message": "Mission dispatched to the swarm. Follow progress via WebSocket." }
```

---

### WebSocket 事件（`/ws`）

#### 服务器 → 前端

| `type` | 触发时机 | 相关字段 |
|---|---|---|
| `status` | 来自蜂王的进度消息 | `message` |
| `agent_change` | 专家接管流水线 | `agent` |
| `tool_start` | 工具即将执行 | `agent`, `tool`, `args` |
| `approval_request` | AI 要使用受限工具 | `id`, `tool`, `args` |
| `result` | 工作流最终报告 | `message` |
| `error` | 失败或超时 | `message` |

#### 前端 → 服务器

```json
{ "type": "approve", "id": "req-1712345678901", "approved": true }
{ "type": "approve", "id": "req-1712345678901", "approved": false }
```

---

## ⚖️ 许可与商业用途 (双重许可制)

**Jandaira Swarm OS** 采用双重许可模式分发，旨在促进开源社区发展的同时满足企业合规需求。

* **开源用途（AGPLv3）：** 源代码在 [GNU Affero General Public License v3.0](../LICENCE) 许可下免费提供。
* **企业商业用途：** 如果企业希望将 Jandaira 集成到专有商业产品中，我们提供**商业许可**。

---

## 🤝 贡献

欢迎 Pull Request！在开始之前，请先打开一个 issue 描述功能或 bug。

---

*Jandaira：自主、安全与巴西蜂群的力量。* 🐝
