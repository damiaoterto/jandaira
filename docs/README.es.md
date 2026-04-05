# 🐝 Jandaira Swarm OS

<p align="center">
  <img src="../jandaira.png" alt="Jandaira Logo"/>
</p>

Un marco de **múltiples agentes autónomos** escrito en Go, inspirado en la inteligencia colectiva de la abeja nativa sudamericana _Melipona subnitida_ — la **Jandaíra**.

---

> 🌐 [English](README.en.md) · [Português](../README.md) · [中文](README.zh.md) · [Русский](README.ru.md) · **Español**

> 📦 [**Descargar binarios precompilados**](https://github.com/damiaoterto/jandaira/releases) — Linux, Windows, macOS y Raspberry Pi

---

## 📖 ¿Por qué "Jandaira"?

La **Jandaíra** es una abeja sin aguijón. Pequeña, resiliente y extraordinariamente cooperativa — no necesita de un líder centralizado para construir una colmena funcional. Cada obrera conoce su papel, ejecuta su tarea con autonomía y devuelve el resultado a la comunidad.

Este es exactamente el modelo de arquitectura que el proyecto implementa:

- La **Reina (`Queen`)** no ejecuta tareas — ella orquesta, valida políticas y garantiza la seguridad.
- Las **Especialistas (`Specialists`)** son agentes ligeros con herramientas restringidas, operando en silos aislados.
- El **Néctar** es la metáfora para el presupuesto de tokens.
- El **Panal (`Honeycomb`)** es la memoria vectorial compartida.
- El **Apicultor** es el humano en el bucle: aprueba o bloquea acciones vitales.

---

## 🚀 Tutorial de Uso

### Requisitos Previos

```bash
# Go 1.22 o superior
go version

# Opcional: Establece mediante variables de entorno (Pipeline/CI)
export OPENAI_API_KEY="sk-..."
# NOTA: El Asistente Interactivo (Wizard) también puede pedirte esta clave
# en el acceso inicial y guardarla ocultamente en tu Cloud Vault nativo (`~/.config/jandaira/.secrets`).
```

### Ejecutar la colmena

```bash
# Entorno Dev
go run ./cmd/cli/main.go
```

Cuando lo inicies, el **Bubble Tea Wizard** te guiará para configurar la clave de OpenAI, el presupuesto de tokens, etc. ¡No es necesario pre-configurar manualmente!

## 🔐 Seguridad y Vault

Cada "pase de testigo" entre Especialistas está **encriptado de extremo a extremo con AES-GCM**.
Además, los créditos, llaves y accesos son gestionados estáticamente localmente usando nuestro propio paquete `internal/security/vault.go` protegiendo tu clave API de accesos por otros procesos o usuarios no autorizados por el OS.

## 🤝 Contribuyendo

¡Pull Requests son bienvenidos! Abre un caso (issue) describiendo la funcionalidad antes de iniciar grandes cambios.

---
_Jandaira: Autonomía, Seguridad y la Fuerza del Enjambre._ 🐝
