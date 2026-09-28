# Especificaciones de postik

Hay tres capas. En caso de conflicto, cada una manda sobre las siguientes:

1. [`constitution.md`](constitution.md): los principios.
2. [La funcional](postik/alcance-reducido-v1/functional-specs.md): qué hace el producto y para quién. Sus flujos son `F1`–`F18` y sus reglas están en la sección 6.
3. `Sxx-*.md`: una spec técnica por vertical, con casos `Sxx.n` en forma Dado / Cuando / Entonces. Cada caso cita los flujos y reglas que cubre.

Cada spec técnica se escribe y se revisa antes de escribir el código de su vertical.

## Orden de los verticales

| Spec | Vertical | Cubre | En pantalla al terminar | Introduce | Estado |
|---|---|---|---|---|---|
| S01 | Plataforma y superadmin | F2; §6.1: superadmin, su sesión e idiomas | Pantalla de acceso. El superadmin entra con contraseña y TOTP, y ve el panel vacío en español o en inglés | Binario, devcontainer, esquema y migraciones, OpenAPI, SPA embebida, `spec-check`, CI que construye la imagen | Implementada; falta que Rubén la pruebe |
| S02 | Acceso OIDC y organizaciones | F1, F4; §6.1: sesión de los miembros; §6.2: membresía | «Entrar con el proveedor», su propia organización creada y el selector de organización | Cliente OIDC y emisor falso para los tests | Implementada; falta que Rubén la pruebe |
| S03 | Canales, empezando por Telegram | F6, F8; §6.2: clientes; §6.3 | Añadir un canal de Telegram, la barra lateral agrupada por cliente y el menú contextual | Doble de la API de bots de Telegram | Implementada; falta que Rubén la pruebe |
| S04 | Posts, calendario y medios | F9, F10, F15; §6.5–§6.7 | Editor, calendario con vistas de día, semana, mes y lista, etiquetas y biblioteca de medios | Almacenamiento de medios en disco | Implementada; falta que Rubén la pruebe |
| S05 | Publicación y avisos | F11, F13, F14, F18; §6.8 | Un post programado sale a su hora en Telegram. Notificaciones y correos | Cola de trabajos y envío de correo | Implementada. Bloque A probado por Rubén en pelayo (2026-09-28); falta probar el bloque B (avisos) |
| S06 | LinkedIn (perfil y página) y X | F5, F7, F12, aviso de caducidad; §6.3 (X); §6.4 | Conectar, publicar y reconectar cada red | OAuth, tokens cifrados y dobles de LinkedIn y X | Bloque A (OAuth y LinkedIn) implementado; falta que Rubén lo pruebe. Bloques B (X) y C pendientes |
| S07 | Redes de Meta (Facebook, Threads e Instagram) y YouTube | F5, F7, F12; §6.4 | Lo mismo en estas redes | Dobles de la Graph API y de YouTube | Pendiente |
| S08 | MCP | F16; §6.9 | Ajustes > MCP con tokens personales. Un cliente MCP programa un post | SDK de MCP | Pendiente |
| S09 | Equipo y panel del superadmin | F3, F17; §6.2; §8.4 y §8.5 | Invitar y quitar miembros, perfil, borrar la cuenta y el panel del superadmin completo | Correo de invitación | Pendiente |
| S10 | Sustitución de suntzu | Operación | postik en producción y suntzu retirado | Manifiestos en Mileto | Pendiente |

**Primer hito, al cerrar S05:** Rubén entra con su proveedor OIDC, conecta Telegram, programa un post y lo ve salir a su hora. Así se prueba el planificador sin depender de las credenciales de Meta o X ni de que revisen ninguna app.

El equipo va al final porque la funcional ya da a quien entra sin invitación su propia organización (S1 de la funcional). Hasta S09, una sola persona puede usar postik de principio a fin.

## Stack propuesto

Cada spec técnica confirma o cambia la parte que introduce.

| Pieza | Propuesta | Notas |
|---|---|---|
| Servidor | Go 1.26 | Como Sebastián. |
| API | Se escribe primero la OpenAPI (`api/openapi.yaml`). El servidor se genera con `oapi-codegen` en modo strict y el frontend saca sus tipos con `openapi-typescript` | Como Sebastián. |
| Base de datos | Postgres con `pgx` y `bun`. Esquema declarado en `db/schema.sql` y migraciones con Atlas | Como Sebastián. |
| Frontend | La interfaz de Postiz portada (constitución §2) a una SPA con Vite, React, TanStack Router y TanStack Query, embebida en el binario con `embed.FS`. Tailwind 3.4, la misma versión que Postiz, para que las clases se vean igual | **Distinto de Sebastián**, que usa TanStack Start con Nitro y por eso ejecuta Node dentro del pod. Aquí la SPA es estática, como pide la constitución (§7). |
| Cola de trabajos (S05) | River, sobre el mismo Postgres y en su propio esquema `river`. Lo migra `rivermigrate` dentro de `postik migrate`; Atlas solo gestiona `public` | Si River estorba, la alternativa es una tabla propia con `FOR UPDATE SKIP LOCKED`. |
| OIDC (S02) | `coreos/go-oidc` y `golang.org/x/oauth2` | |
| MCP (S08) | SDK oficial `modelcontextprotocol/go-sdk`: HTTP streamable, `auth.RequireBearerToken` y metadatos del recurso protegido | Mismo diseño que la rama `feat/mcp-keycloak-oauth` de ZetesisPortal (F16). |
| Tests | `go test` contra el Postgres del devcontainer; Playwright para las pantallas | Constitución, §5. |

## Esqueleto del repositorio

Lo crea S01:

```
postik/
├── cmd/postik/             binario único: `postik serve`, `postik migrate`
├── internal/
│   ├── core/               núcleo funcional, sin I/O; un paquete por dominio
│   ├── httpapi/            servidor generado y su adaptación al núcleo
│   ├── postgres/           acceso a datos
│   ├── auth/               superadmin, OIDC y sesiones
│   ├── providers/          una implementación por red (S03, S06, S07)
│   ├── jobs/               trabajos programados (S05)
│   ├── mcp/                servidor MCP (S08)
│   └── webui/              embed de la SPA compilada
├── api/openapi.yaml
├── db/schema.sql           esquema declarado; Atlas genera las migraciones
├── web/                    SPA; sus tests de Playwright en web/e2e
├── tools/speccheck/        comprueba el anclaje entre specs y tests
├── .devcontainer/          Go, Node y dos Postgres: el de la app y el de Atlas
├── Dockerfile              multietapa: SPA → binario Go → imagen mínima
└── Makefile                generate, fmt, test, e2e, spec-check, migration, check
```

## Preguntas abiertas

Cada una se cierra antes de empezar el vertical que la necesita:

| Id | Pregunta | Hace falta en |
|---|---|---|
| P1 | ¿Migramos canales y posts de suntzu o empezamos de cero y se reconectan los canales? Hoy suntzu no tiene posts programados a futuro. | S10 |
| P2 | Dominio: ¿reutilizamos `suntzu.nexolabs.dev` o ponemos uno nuevo? | S10 |
| P3 | **Decidida el 2026-09-28: Resend**, como suntzu, por su API HTTP y sin SDK. La clave está en Infisical `pelayo-cluster` `/postik` (`RESEND_API_KEY`). | S05 |
| P4 | OIDC en desarrollo: ¿el realm de desarrollo de Zetesis-Auth o un emisor local en el devcontainer? | S02 |
| P5 | **Decidida el 2026-09-28: sí.** postik se despliega en pelayo junto a suntzu (namespace `postik`, `postik.nexolabs.dev` detrás de colon). Ese dominio resuelve a la LAN (`10.0.0.7`): vale para S01–S05 y para los retornos OAuth de S06/S07, que pasan por el navegador, pero no para que Instagram y Threads descarguen medios; eso se resuelve en S07, con Instagram al final por decisión de Rubén (2026-09-28). Comparte el bot de Telegram con suntzu, que está apagada. | S02 |
| P6 | **Decidida el 2026-09-28: ghcr.io** (`ghcr.io/zetesis-labs/postik`, etiquetas `sha-<commit>` y `main`), publicada por la CI al mergear a `main`. | Primer despliegue (P5) |
