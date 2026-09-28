# S03 · Canales, empezando por Telegram

Cubre F6 y F8, los clientes de §6.2, §6.3 y la fila de Telegram de §6.4. Es la base de canales sobre la que S06 y S07 añaden las redes con OAuth.

En F8, «Crear post» llega con el editor (S04). «Reconectar» y «Ajustes adicionales» llegan con las redes que los usan (S06 y S07).

## 1. Objetivo y límites

Al terminar S03, en `/launches`:

- **Barra lateral:** muestra los canales de la organización activa agrupados por cliente, con el aviso de «sin canales» cuando no hay ninguno. Se puede plegar.
- **«Añadir canal»:** abre la rejilla con los proveedores que tienen credenciales (S4). En S03 solo existe Telegram.
- **Telegram:** se conecta con el código de 4 caracteres y el comando `/connect` (F6).
- **Menú contextual de cada canal:**
  - copiar el ID;
  - mover a un cliente, también arrastrando el canal sobre un grupo;
  - editar las franjas;
  - activar o desactivar;
  - borrar.

## 2. Configuración

| Variable | Obligatoria | Qué es |
|---|---|---|
| `POSTIK_TELEGRAM_BOT_TOKEN` | No | Token del bot. Sin él, Telegram no aparece en la rejilla |
| `POSTIK_TELEGRAM_API_URL` | No, `https://api.telegram.org` | API de bots. Los tests y `make dev` la apuntan al falso |
| `POSTIK_STORAGE_DIR` | No, `data` | Carpeta de ficheros. Los avatares van en `avatars/` y se sirven públicamente en `/uploads/avatars/<fichero>` |

**Un bot por instancia.** El bot se lee pidiendo a Telegram los mensajes pendientes, y si dos instancias comparten bot se los quitan la una a la otra. suntzu y postik no pueden usar el mismo bot a la vez.

## 3. Conexión de Telegram

1. `POST /api/v1/channels/telegram/connections` crea una conexión pendiente en la organización activa, con un código de 4 caracteres alfanuméricos único entre las pendientes. Devuelve el código y el usuario del bot, que se obtiene de `getMe` y se guarda en caché.
2. La pantalla consulta `GET /api/v1/channels/telegram/connections/{code}` cada 2 segundos. Cada consulta hace una sincronización con Telegram, y como mucho hay una a la vez en el proceso:
   1. pide `getUpdates` desde el último desplazamiento guardado, sin espera, y guarda el nuevo desplazamiento;
   2. reconoce `/connect <código>` y `/connect@<bot> <código>`, tanto en mensajes como en publicaciones de canal;
   3. si el código es de una conexión pendiente y vigente:
      - pide el chat con `getChat`;
      - intenta borrar el mensaje del comando con `deleteMessage` (si el bot no es administrador falla, y se sigue);
      - descarga la foto grande del chat;
      - crea o actualiza el canal en la organización de esa conexión y la marca como conectada;
   4. descarta el resto de mensajes.
3. La respuesta es `pending`, `connected` (con el ID del canal) o `expired`. Un código de otra organización o inexistente da 404.
4. **[Supuesto]** Una conexión pendiente caduca a los 30 minutos. Postiz no pone límite.

## 4. Decisiones del núcleo

En `internal/core/channels`, funciones puras:

- **Reconocer el comando:** en `/connect <código>` o `/connect@<bot> <código>`, el código tiene 4 caracteres alfanuméricos. Cualquier otro texto no es un comando.
- **Estado de una conexión:** conectada, pendiente o caducada, según la hora de creación y la hora actual.
- **Nombre del canal:** el título si es un grupo o un canal; en un chat privado, nombre y apellido.
- **Reconexión del mismo chat** en la misma organización: se actualizan el nombre, el usuario y la foto, y se conservan el cliente, las franjas y si está desactivado (§6.3, unicidad).
- **Franjas:** minutos desde medianoche UTC, entre 0 y 1439. Se guardan ordenadas y sin repetir. Por defecto son 120, 400 y 700, como en Postiz.
- **Nombre de cliente:** se recortan los espacios. Si queda vacío, el canal sale del cliente.

## 5. Modelo de datos

| Tabla | Campos |
|---|---|
| `customers` | `id`, `organization_id`, `name` (único en la organización), `created_at` |
| `channels` | `id`, `organization_id`, `provider`, `external_id` (único con la organización y el proveedor), `name`, `username`, `picture`, `disabled`, `refresh_needed`, `in_between_steps`, `customer_id`, `posting_times`, `created_at`, `updated_at` |
| `telegram_connections` | `code`, `organization_id`, `created_at`, `channel_id` |
| `telegram_state` | `next_offset` |

Borrar un canal lo borra de verdad. S04 hará que se lleve sus posts en cascada (F8).

## 6. Frontera de la API

Todo actúa sobre la organización activa. Un canal o un cliente de otra organización responde 404.

| Método y ruta | Qué hace |
|---|---|
| `GET /api/v1/channels/providers` | Proveedores con credenciales: `identifier` y `name` |
| `GET /api/v1/channels` | Canales con su estado, cliente y franjas |
| `DELETE /api/v1/channels/{id}` | Borra el canal |
| `PUT /api/v1/channels/{id}/disabled` | `{ "disabled": true \| false }` |
| `PUT /api/v1/channels/{id}/customer` | `{ "name": "…" }` mueve por nombre (lo crea si no existe; vacío lo quita), o `{ "customerId": "…" \| null }` mueve a un cliente existente o lo quita |
| `PUT /api/v1/channels/{id}/posting-times` | `{ "times": [minutos] }` |
| `GET /api/v1/customers` | Clientes de la organización |
| `POST /api/v1/channels/telegram/connections` | Paso 1 de §3 |
| `GET /api/v1/channels/telegram/connections/{code}` | Paso 2 de §3 |

## 7. Pruebas

- **Telegram falso:** `internal/testsupport/faketelegram` imita la API de bots: `getMe`, `getUpdates`, `getChat`, `getFile`, `deleteMessage` y la descarga de ficheros.
  - Los tests de Go le meten mensajes directamente.
  - `postik-fakes` lo sirve bajo `/telegram`, con `POST /telegram/_control/message` para Playwright y un formulario en `GET /telegram/_control` para probar a mano.
- **Bot real:** en `make dev`, cambiando `POSTIK_TELEGRAM_BOT_TOKEN` y `POSTIK_TELEGRAM_API_URL` en `.devcontainer/.env`.

## 8. Interfaz

Portada de Postiz (constitución, §2):

- **Barra de canales:** `launches.component.tsx`, con los grupos por cliente, el plegado y el aviso de «sin canales». El arrastre usa `react-dnd`, como en Postiz.
- **«Añadir canal» y la rejilla:** `add.provider.component.tsx`. Se quita el botón de enlace de invitación, que pertenece a la API pública, fuera de la v1.
- **Ventana de Telegram:** `web3/providers/telegram.provider.tsx`.
- **Menú contextual:** `menu/menu.tsx`, con las acciones de §1.
- **Mover a cliente:** `customer.modal.tsx`.
- **Franjas:** `time.table.tsx`. Se corrige un fallo de Postiz: borraba la franja por su posición en la lista ordenada y no por su valor.
- **Avisos y tooltips:** el toaster de Postiz y `react-tooltip`.

## 9. Casos

## S03.1 Solo se ofrecen los proveedores con credenciales

Cubre: F5 paso 1; S4.

- **Dado** una instancia sin token de Telegram y otra con él.
- **Cuando** se piden los proveedores y se intenta abrir una conexión de Telegram.
- **Entonces** la primera no ofrece ninguno y la conexión responde 404. La segunda ofrece Telegram.

## S03.2 Conectar Telegram crea el canal con nombre, usuario y foto

Cubre: F6.

- **Dado** el bot `@postik_test_bot` y el grupo «Grupo Zetesis» (`@grupozetesis`), con foto, donde el bot es administrador.
- **Cuando** se abre una conexión, alguien escribe `/connect <código>` en el grupo y la pantalla consulta el estado.
- **Entonces**:
  - la conexión devuelve un código de 4 caracteres y el bot `postik_test_bot`;
  - el estado pasa a `connected`;
  - el canal aparece activo, con 3 franjas por defecto, «Grupo Zetesis», `grupozetesis` y una foto servida en `/uploads/avatars/`;
  - el mensaje del comando se ha borrado.

## S03.3 Sin mensaje la conexión sigue pendiente y a los 30 minutos caduca

Cubre: F6, errores; §3 de esta spec.

- **Dado** una conexión abierta.
- **Cuando** se consulta sin que llegue el mensaje, y después pasados 30 minutos llega `/connect <código>`.
- **Entonces** primero está `pending` y después `expired`, sin crear ningún canal.

## S03.4 Volver a conectar el mismo chat actualiza el canal y conserva sus ajustes

Cubre: §6.3, unicidad.

- **Dado** el grupo conectado, en el cliente «Acme», con franjas propias y desactivado.
- **Cuando** el grupo pasa a llamarse «Zetesis Labs» y se vuelve a conectar en la misma organización.
- **Entonces** sigue habiendo un solo canal, se llama «Zetesis Labs» y conserva el cliente, las franjas y el estado desactivado.

## S03.5 Cada código conecta en su organización y el de otra no se ve

Cubre: F6; constitución §8.

- **Dado** Ana y Bruno, cada uno en su organización, con una conexión abierta cada uno.
- **Cuando** llegan los dos comandos y Ana consulta su código y el de Bruno.
- **Entonces** el suyo queda conectado en su organización y el de Bruno da 404. El canal de Bruno solo aparece en la organización de Bruno.

## S03.6 Si el bot no puede borrar el mensaje, la conexión sigue adelante

Cubre: F6 paso 4.

- **Dado** un grupo donde el bot no es administrador.
- **Cuando** llega `/connect <código>`.
- **Entonces** el canal se conecta aunque el mensaje no se haya borrado.

## S03.7 Mover a un cliente por nombre lo crea una vez y se puede deshacer

Cubre: F8, mover a cliente; §6.2, clientes.

- **Dado** dos canales sin cliente.
- **Cuando**:
  - se mueve el primero a « Acme » y el segundo a «Acme»;
  - después se mueve el segundo por ID al cliente de otra organización;
  - y por último se saca el primero del cliente.
- **Entonces**:
  - solo existe un cliente «Acme», con los dos canales;
  - mover al cliente ajeno responde 404;
  - el primer canal queda sin cliente.

## S03.8 Las franjas se guardan ordenadas, sin repetir y dentro del día

Cubre: F8, editar franjas; §6.3.

- **Dado** un canal con las franjas por defecto.
- **Cuando** se guardan `[700, 60, 700, 1439]` y después `[1440]`.
- **Entonces** quedan `[60, 700, 1439]`, y el segundo intento responde 400 sin cambiar nada.

## S03.9 Desactivar y activar un canal

Cubre: F8; §6.3.

- **Dado** un canal activo.
- **Cuando** se desactiva y después se activa.
- **Entonces** la lista lo muestra desactivado y luego activo.

## S03.10 Borrar un canal lo quita de la lista

Cubre: F8.

- **Dado** dos canales.
- **Cuando** se borra uno.
- **Entonces** la lista solo trae el otro.

## S03.11 Los canales de otra organización no se ven ni se tocan

Cubre: constitución §8.

- **Dado** un canal en la organización de Bruno.
- **Cuando** Ana pide la lista y trata de desactivarlo, moverlo, cambiar sus franjas y borrarlo.
- **Entonces** la lista de Ana no lo trae, las cuatro acciones responden 404 y el canal sigue intacto.

## S03.12 Añadir un canal de Telegram desde la rejilla hasta verlo en la barra lateral

Cubre: F6; §8.1; §8.3.

- **Dado** una persona recién entrada, sin canales.
- **Cuando** ve el aviso de «sin canales», pulsa «Añadir canal», elige Telegram, pulsa «Conectar Telegram» y el comando llega al bot.
- **Entonces** la ventana se cierra y el canal aparece en la barra lateral con su nombre.

## S03.13 El menú contextual mueve a cliente, edita franjas, desactiva y borra

Cubre: F8; §8.1.

- **Dado** un canal de Telegram conectado.
- **Cuando** se usan en su menú:
  - «Mover / añadir a grupo» con el nombre «Acme»;
  - «Editar franjas», añadiendo una;
  - «Desactivar canal»;
  - «Borrar».
- **Entonces**:
  - aparece el grupo «Acme» con el canal;
  - las franjas pasan a ser cuatro;
  - el canal se ve semitransparente;
  - al borrarlo desaparece de la barra.
