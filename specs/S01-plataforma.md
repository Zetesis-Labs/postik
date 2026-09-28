# S01 · Plataforma y superadmin

Cubre F2 y, de §6.1, el superadmin, su sesión y los idiomas. Además monta todo lo que los demás verticales dan por hecho: el binario, el esquema, la API, la SPA embebida, el anclaje entre specs y tests, el devcontainer y la CI.

## 1. Objetivo y límites

Al terminar S01:

- se abre la pantalla de acceso de postik, portada de Postiz, en español o en inglés;
- el superadmin entra con usuario, contraseña y, si está configurado, un código TOTP o de recuperación;
- llega a su panel, todavía vacío, y puede cerrar sesión.

Fuera de S01: el acceso OIDC y las organizaciones (S02) y el contenido del panel del superadmin (S09).

## 2. Topología

Un solo proceso, `postik serve`, atiende en un puerto:

| Ruta | Qué es |
|---|---|
| `/api/v1/*` | API JSON descrita en `api/openapi.yaml` |
| `/healthz` | El proceso vive |
| `/readyz` | El proceso puede atender: la base de datos responde |
| cualquier otra | La SPA embebida; las rutas que no son ficheros devuelven `index.html` |

`postik migrate` aplica las migraciones y termina. En el despliegue corre antes que `postik serve`.

## 3. Configuración

Todo llega por variables de entorno. Si falta una obligatoria o alguna tiene un valor inválido, el proceso no arranca y el error nombra la variable.

| Variable | Obligatoria | Qué es |
|---|---|---|
| `DATABASE_URL` | Sí | Conexión a Postgres |
| `POSTIK_PUBLIC_URL` | Sí | URL con la que se llega a la aplicación. Si es `https`, la cookie de sesión se marca como segura |
| `POSTIK_LISTEN_ADDR` | No, `:8080` | Dirección de escucha |
| `POSTIK_SUPERADMIN_USERNAME` | Sí | Usuario del superadmin |
| `POSTIK_SUPERADMIN_PASSWORD` | Sí | Contraseña del superadmin, en claro dentro del secreto |
| `POSTIK_SUPERADMIN_TOTP_SECRET` | No | Semilla TOTP en base32. Si está, el 2FA es obligatorio |
| `POSTIK_SUPERADMIN_RECOVERY_CODES` | No | Códigos de recuperación separados por comas. Solo valen si hay semilla TOTP |

## 4. Superadmin sin estado

La funcional (S2) deja al superadmin como pura configuración, sin nada guardado en la base de datos. Consecuencias que se aceptan:

- **Los códigos de recuperación no se gastan.** Un código sirve mientras siga en el secreto. Después de usarlo, hay que rotar el secreto.
- **Un código TOTP puede reutilizarse** dentro de su ventana de validez (±30 segundos).
- **Para quitar el 2FA** se borra la semilla del secreto. No hay otra forma.

## 5. Sesiones

- **Cookie:** `postik_session`, con un identificador aleatorio de 32 bytes. Es `HttpOnly` y `SameSite=Lax`, y lleva `Secure` si la URL pública es `https`.
- **En el servidor:** la tabla `sessions` guarda el hash SHA-256 del identificador, nunca el identificador.
- **Superadmin:** su sesión dura 12 horas desde que entra y no se renueva con el uso (§6.1).
- **Cerrar sesión:** borra la sesión del servidor y la cookie.
- **Peticiones que cambian estado** (`POST`, `PUT`, `PATCH`, `DELETE`): se rechazan con 403 si su cabecera `Origin` no es la de la URL pública.

Las decisiones (validar credenciales, comprobar el TOTP, calcular la caducidad) son funciones puras en `internal/core/access`. El TOTP se implementa con la biblioteca estándar, siguiendo el RFC 6238 con SHA-1, 6 dígitos y pasos de 30 segundos.

## 6. Frontera de la API

| Método y ruta | Qué hace |
|---|---|
| `GET /api/v1/instance` | Configuración pública: si el superadmin tiene 2FA y qué idiomas hay |
| `POST /api/v1/auth/superadmin` | Entrada del superadmin con `username`, `password` y `code` (opcional) |
| `POST /api/v1/auth/logout` | Cierra la sesión |
| `GET /api/v1/me` | Quién es la sesión actual: `{ "kind": "superadmin" }`. Sin sesión, 401 |

Los errores tienen siempre la forma `{ "code": "...", "message": "..." }`.

## 7. Interfaz

La interfaz se porta de Postiz (constitución, §2):

- **Pantalla de acceso:** el diseño de `app/(app)/auth/layout.tsx`, con el panel oscuro a la izquierda y el logotipo de postik. El texto promocional y los testimonios de Postiz no se traen.
- **Contenido de la pantalla de acceso:** título «Iniciar sesión», el enlace «Acceso de administrador» y el selector de idioma de `components/layout/language.component.tsx`, limitado a español e inglés. El botón OIDC llega en S02.
- **Formulario del superadmin:** los mismos `Input` y `Button` de Postiz. El campo del código solo aparece si `GET /api/v1/instance` dice que hay 2FA.
- **Panel del superadmin:** la cabecera y el marco de `components/new-layout/layout.component.tsx`, con un aviso de que el panel todavía está vacío y el menú con «Cerrar sesión».
- **Idioma:** el del navegador si es español o inglés, y si no, inglés. El selector lo cambia y la elección se recuerda en el navegador. Las traducciones parten de las de Postiz; las claves nuevas se añaden en los dos idiomas.
- **Versiones fijadas:** Tailwind 3.4.17, igual que Postiz, para que las clases portadas se vean igual.

## 8. Anclaje

- `make spec-check` recorre los encabezados `## Sxx.n` de `specs/S*.md`.
- En Go, busca `// Sxx.n` en la primera línea del comentario de cada `func Test…`.
- En Playwright, busca títulos de test que empiezan por `Sxx.n`.
- Falla si un caso no tiene test o si un test cita un caso que no existe.

## 9. Casos

## S01.1 El proceso informa de su salud y de la de la base de datos

Cubre: constitución §10.

- **Dado** `postik serve` en marcha con la base de datos accesible.
- **Cuando** se pide `/healthz` y `/readyz`.
- **Entonces** los dos responden 200. Si la base de datos deja de responder, `/readyz` pasa a 503 y `/healthz` sigue en 200.

## S01.2 Las migraciones dejan el esquema al día y se pueden repetir

Cubre: constitución §7.

- **Dado** una base de datos vacía.
- **Cuando** se ejecuta `postik migrate` dos veces seguidas.
- **Entonces** la primera crea el esquema, la segunda no cambia nada, y las dos terminan sin error.

## S01.3 Una configuración incompleta impide arrancar y nombra la variable

Cubre: §3 de esta spec.

- **Dado** un entorno sin `POSTIK_SUPERADMIN_PASSWORD`, o con `POSTIK_SUPERADMIN_TOTP_SECRET` que no es base32 válido, o con códigos de recuperación sin semilla TOTP.
- **Cuando** se carga la configuración.
- **Entonces** se devuelve un error que nombra la variable afectada y el proceso no arranca.

## S01.4 El superadmin sin 2FA entra con usuario y contraseña

Cubre: F2.

- **Dado** una instancia sin semilla TOTP.
- **Cuando** se envía `POST /api/v1/auth/superadmin` con el usuario y la contraseña correctos.
- **Entonces** la respuesta es 200 y trae la cookie `postik_session`. `GET /api/v1/me` con esa cookie devuelve `kind: superadmin`.

## S01.5 Con 2FA hace falta un código TOTP vigente

Cubre: F2; §6.1.

- **Dado** una instancia con semilla TOTP y el reloj fijado.
- **Cuando** se entra con la contraseña correcta y el código del paso actual, del anterior, del siguiente o de hace dos pasos.
- **Entonces** los tres primeros entran; el de hace dos pasos, y la entrada sin código, dan el error genérico.

## S01.6 Un código de recuperación sustituye al TOTP y no se gasta

Cubre: F2; §6.1; §4 de esta spec.

- **Dado** una instancia con semilla TOTP y los códigos de recuperación `alfa-1234` y `beta-5678`.
- **Cuando** se entra dos veces con la contraseña correcta y `alfa-1234` como código.
- **Entonces** las dos entradas funcionan. Un código que no está en la lista da el error genérico.

## S01.7 Cualquier dato incorrecto da el mismo error, sin decir cuál

Cubre: F2.

- **Dado** una instancia con 2FA.
- **Cuando** se entra con el usuario mal, con la contraseña mal o con el código mal.
- **Entonces** las tres respuestas son idénticas: 401 con `code: invalid_credentials`, y ninguna trae cookie.

## S01.8 La sesión del superadmin caduca a las 12 horas aunque se use

Cubre: §6.1.

- **Dado** una sesión de superadmin abierta a las 10:00, que se usa cada hora.
- **Cuando** se pide `GET /api/v1/me` a las 21:59 y a las 22:00.
- **Entonces** a las 21:59 responde 200 y a las 22:00 responde 401.

## S01.9 Cerrar sesión la invalida en el servidor

Cubre: F2.

- **Dado** una sesión de superadmin abierta.
- **Cuando** se envía `POST /api/v1/auth/logout` y después se reutiliza la cookie antigua.
- **Entonces** el cierre borra la cookie y la petición con la cookie antigua responde 401.

## S01.10 Sin sesión, la API responde 401

Cubre: §6.1.

- **Dado** una petición sin cookie o con una cookie desconocida.
- **Cuando** se pide `GET /api/v1/me`.
- **Entonces** la respuesta es 401 con `code: unauthenticated`.

## S01.11 Una petición que cambia estado desde otro origen se rechaza

Cubre: §5 de esta spec.

- **Dado** una sesión de superadmin abierta.
- **Cuando** se envía `POST /api/v1/auth/logout` con `Origin: https://otro.example`.
- **Entonces** la respuesta es 403 y la sesión sigue viva.

## S01.12 El binario sirve la SPA y la API por separado

Cubre: §2 de esta spec.

- **Dado** el binario con la SPA embebida.
- **Cuando** se piden `/`, `/launches` y `/api/v1/no-existe`.
- **Entonces** `/` y `/launches` devuelven el `index.html` de la SPA, y `/api/v1/no-existe` devuelve 404 en JSON.

## S01.13 La pantalla de acceso sale en el idioma del navegador y el selector lo cambia

Cubre: §6.1; §8.6.

- **Dado** un navegador en español y otro en inglés.
- **Cuando** abren la pantalla de acceso, y después el segundo elige español en el selector y recarga.
- **Entonces** el primero ve «Iniciar sesión», el segundo ve «Sign In», y tras elegir español y recargar ve «Iniciar sesión».

## S01.14 El superadmin entra con TOTP, llega a su panel y sale

Cubre: F2; §8.5; §8.6.

- **Dado** una instancia con 2FA.
- **Cuando** en la pantalla de acceso se pulsa «Acceso de administrador», se rellenan usuario, contraseña y código, se entra, y después se elige «Cerrar sesión».
- **Entonces** tras entrar se ve el panel del superadmin, y tras cerrar sesión se vuelve a la pantalla de acceso.

## S01.15 El anclaje detecta casos sin test y tests sin caso

Cubre: constitución §3.

- **Dado** una spec con los casos `S99.1` y `S99.2`, y tests que citan `S99.1` y `S99.3`.
- **Cuando** se ejecuta la comprobación del anclaje.
- **Entonces** falla, y el informe dice que `S99.2` no tiene test y que `S99.3` no existe.
