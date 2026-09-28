# S05 · Publicación y avisos

Cubre F11, F13, F14 y F18, y la sección 6.8. Es el primer hito del plan: un post programado sale a su hora en Telegram.

Se entrega en dos PRs apilados:
- **Bloque A, publicación:** F11, F13, F14 y los estados del post en el calendario.
- **Bloque B, avisos:** notificaciones (F18), correo y preferencias (§6.8).

## 1. Objetivo y límites

Al terminar S05:

- **Programar:** un post programado se publica a su hora en Telegram. «Publicar ya» sale en el momento. Después salen los comentarios, cada uno tras su retardo.
- **Resultado:** el post queda **Publicado**, con el enlace al mensaje, o en **Error**, con el mensaje visible en el calendario.
- **Nunca dos veces:** un post no se publica dos veces por un reintento, un reinicio o un barrido.
- **Barrido horario:** relanza los programados de las últimas 48 horas que se quedaron sin salir.
- **Vincular publicación:** el enlace de un post publicado sin enlace se puede poner a mano.
- **Campana:** muestra las notificaciones de la organización, con las no leídas de cada miembro.
- **Correos:** los informativos, siempre; los de fallo, al momento; los de éxito, en un resumen horario. Cada miembro elige en Ajustes > General.

Fuera de S05:

- **Renovación de tokens (F12):** Telegram no tiene tokens que renovar. Llega con las redes que sí los tienen, en S06 y S07. Por eso el paso 4 de F11 no hace nada en Telegram.
- **Aviso de caducidad de Facebook e Instagram:** es de S06.

## 2. Configuración

| Variable | Obligatoria | Qué es |
|---|---|---|
| `POSTIK_RESEND_API_KEY` | No | Clave de Resend. Sin ella no se envían correos, y las notificaciones siguen creándose |
| `POSTIK_EMAIL_FROM` | Sí, si hay clave de Resend | Remitente, por ejemplo `postik <postik@mail.zetesis.xyz>` |
| `POSTIK_RESEND_API_URL` | No, `https://api.resend.com` | Para apuntar al Resend falso en desarrollo y en las pruebas |

## 3. Cola de trabajos

- **River**, en el esquema `river` del mismo Postgres:
  - lo migra `postik migrate` con `rivermigrate`, después de las migraciones de Atlas, que solo gestiona `public`;
  - el cliente corre dentro de `postik serve` y se para con el servidor.
- **Trabajos:**

  | Trabajo | Argumentos | Cuándo |
  |---|---|---|
  | `publish_value` | post, fecha de publicación y posición del valor | A la hora del post (posición 0); cada comentario, tras su retardo |
  | `sweep_scheduled` | ninguno | Cada hora (F13) |
  | `success_digest` | ninguno | Cada hora (bloque B) |

- **Encolado:**
  - cada vez que un post queda **Programado** tras crearlo, editarlo o moverlo, se encola `publish_value` en la posición 0, a su fecha;
  - se encola en la misma transacción que guarda el post;
  - los argumentos son únicos: repetir el encolado de lo mismo no duplica el trabajo.
- **Nada se cancela.** Al ejecutarse, el trabajo compara su fecha con la del post. Si el post ya no está Programado (se borró, pasó a borrador o se publicó) o su fecha cambió, no hace nada. Reprogramar deja el trabajo viejo sin efecto y encola uno nuevo.

## 4. Publicar un valor

`publish_value` hace esto, en orden:

1. **Comprueba que sigue siendo su turno.** En la posición 0, el post tiene que estar Programado y con la misma fecha. En un comentario, el post tiene que estar Publicado con esa misma fecha. Si no, no hace nada.
2. **Comprueba el canal.** Si está desactivado o necesita reconexión, el post pasa a Error («Canal desactivado» o «Hay que reconectar el canal»), se avisa (aviso informativo) y no se llama a Telegram.
3. **Mira qué pasó en intentos anteriores.** Cada envío se apunta en `post_deliveries`, con el post, la fecha de publicación y la posición del valor:
   - **si ya consta como enviado**, no se reenvía; se sigue por el paso 6;
   - **si consta como «enviando» sin confirmación**, el proceso se cayó a mitad y no se sabe si Telegram lo recibió. No se reenvía. El post pasa a Error con «No se ha podido confirmar la publicación; revisa el canal» y se avisa del fallo.
4. **Apunta «enviando»** y llama a Telegram (§5).
5. **Interpreta la respuesta:**

   | Respuesta | Qué pasa |
   |---|---|
   | Confirmada, con el ID del mensaje | Se apunta como enviado, con el ID y el enlace |
   | La red rechaza el contenido (400) o el bot (401, 403) | Error con la descripción de Telegram. Sin reintento |
   | Límite de ritmo (429) | No empezó: se borra la marca «enviando» y se reintenta |
   | Fallo al conectar (DNS, conexión rechazada) | No empezó: se borra la marca y se reintenta |
   | Tiempo agotado u otro fallo después de enviar la petición, o un 5xx | No se sabe si llegó: Error «No se ha podido confirmar la publicación» (`unconfirmed`) |

   Se intenta como mucho 5 veces en total, con la espera creciente de River entre intentos. Si se agotan, el post pasa a Error con el último motivo: la descripción de Telegram o, si no se pudo conectar, `unreachable`.

   El campo `error` del post guarda la descripción de Telegram tal cual o uno de estos códigos, que la pantalla traduce: `channel_disabled`, `channel_refresh`, `unconfirmed`, `unreachable`.
6. **Si el valor salió:**
   - en la posición 0, el post pasa a **Publicado**, con el enlace de ese mensaje, y se avisa del éxito;
   - si hay un valor siguiente, se encola su `publish_value` para dentro de su retardo, como respuesta al mensaje anterior.
7. **Si falla un comentario,** el post sigue Publicado: el principal ya está fuera. El envío del comentario queda como `failed`, con su motivo; se avisa del fallo y los comentarios siguientes no se envían.

Un valor que Telegram rechaza queda en `post_deliveries` como `failed`, con su motivo. Uno que no llegó a empezar borra su marca, para que el reintento pueda enviarlo.

**Republicar** (reprogramar un post publicado, S04) crea envíos nuevos, porque la fecha forma parte de la clave de `post_deliveries`.

## 5. Telegram

- **Destino:** el `external_id` del canal, que es el ID del chat.
- **Texto:** el HTML del editor pasa a HTML de Telegram, como en Postiz:
  - `<strong>` pasa a `<b>`, y `<u>` se queda;
  - cada `<p>` termina en salto de línea;
  - cada `<li>` se convierte en una línea que empieza por «• »;
  - se quita cualquier otra etiqueta.
- **Medios:** se suben como fichero desde el disco (`multipart/form-data`), nunca por URL. El dominio de postik no tiene por qué ser accesible desde internet.

  | Medios | Llamada |
  |---|---|
  | Ninguno | `sendMessage` |
  | Una imagen | `sendPhoto`, con el texto como pie |
  | Un vídeo | `sendVideo`, con el texto como pie |
  | Varios | `sendMediaGroup`, en grupos de 10; el texto va como pie del primer medio del primer grupo. Telegram exige al menos 2 medios por grupo, así que si el último grupo se queda con uno solo, ese sale por `sendPhoto` o `sendVideo`, sin pie |

  Telegram limita el pie a 1.024 caracteres. Con medios y más texto, Telegram responde 400 y el post queda en Error con su mensaje, igual que en Postiz.
- **Comentarios:** van con `reply_to_message_id` apuntando al mensaje del valor anterior.
- **Enlace:**
  - si el chat tiene nombre público (`username`), `https://t.me/<username>/<id>`;
  - si no, `https://t.me/c/<ID del chat sin el prefijo -100>/<id>`.

## 6. Barrido horario (F13)

Encola `publish_value` en la posición 0 para los posts que cumplen todo esto:
- están Programados;
- su canal no está desactivado, ni en paso intermedio, ni pendiente de reconexión;
- su fecha está entre hace 48 horas y ahora;
- no tienen ya un trabajo pendiente o en curso para esa fecha;
- no tienen envíos apuntados para esa fecha.

Los que llevan más de 48 horas vencidos siguen como Programados y no se tocan.

## 7. Vincular publicación (F14)

- **Cuándo:** un post Publicado sin enlace. Telegram siempre devuelve ID, así que en la v1 es solo una red de seguridad.
- **API:** `PUT /api/v1/posts/{id}/release` con `url`.
  - La URL tiene que ser `https://`.
  - Si el post no está Publicado o ya tiene enlace, responde 409 `release_not_missing`.
- **Pantalla:** icono «Vincular publicación» en la tarjeta. Abre el modal de Postiz, portado con un campo de URL.

## 8. Avisos (bloque B)

- **Notificaciones:**
  - son de la organización;
  - guardan su tipo (`info`, `failure` o `success`), la plantilla, sus datos (canal, red, enlace, motivo) y la fecha;
  - el texto se compone al mostrarlas, en el idioma de cada pantalla o del correo.

  | Plantilla | Tipo | Texto |
  |---|---|---|
  | `published` | `success` | «Tu post se ha publicado en Telegram: <enlace>» |
  | `failed` | `failure` | «Error al publicar en Telegram en <canal>: <motivo>» |
  | `comment_failed` | `failure` | «Error al publicar los comentarios en Telegram en <canal>: <motivo>» |
  | `unconfirmed` | `failure` | «No se ha podido confirmar tu post en Telegram. Revisa <canal>» |
  | `channel_disabled` | `info` | «No se ha podido publicar en <canal> porque está desactivado. Actívalo y vuelve a intentarlo» |
  | `channel_refresh` | `info` | «No se ha podido publicar en <canal> porque hay que volver a conectarlo» |

- **No leídas:** cada usuario guarda cuándo abrió el panel por última vez. Son no leídas las creadas después.
- **API:**
  - `GET /api/v1/notifications` devuelve las 10 últimas y cuántas hay sin leer;
  - `POST /api/v1/notifications/read` marca que se ha abierto el panel.
- **Correo** (si Resend está configurado), por su API HTTP, a los miembros de la organización:
  - **informativo:** a todos, en el momento, sin mirar preferencias;
  - **fallo:** en el momento, a quienes tienen activados los correos de fallo;
  - **éxito:** en el resumen horario, a quienes tienen activados los correos de éxito. Como mucho un correo por hora y organización, con las publicaciones que aún no habían salido en un resumen. Cada notificación se marca al incluirla, para no repetirla.
- **Idioma de los correos:** español en la v1. postik no guarda el idioma de cada persona.
- **Pie de los correos:** enlace a `<POSTIK_PUBLIC_URL>/settings`.
- **Preferencias:**
  - en el usuario, activadas por defecto, como en Postiz;
  - en Ajustes > General, con dos casillas: correos de éxito y correos de fallo;
  - `GET /api/v1/me` las devuelve y `PUT /api/v1/me/preferences` las cambia.

## 9. Modelo de datos

| Tabla | Campos |
|---|---|
| `post_deliveries` | `post_id` (se borra con el post), `publish_at`, `value_index`, `state` (`sending`, `sent` o `failed`), `external_id`, `url`, `error`, `created_at`, `updated_at`; clave `(post_id, publish_at, value_index)` |
| `notifications` (B) | `id`, `organization_id`, `kind`, `template`, `params` (JSON), `created_at`, `digested_at` |
| `users` (B) | nuevos `notifications_read_at`, `email_success` y `email_failure` |

Las tablas de River viven en el esquema `river` y las crea `rivermigrate`.

## 10. Interfaz

Portada de Postiz (constitución, §2):

- **Tarjeta del calendario:**
  - Publicado: la vista previa lleva a la publicación;
  - Error: borde rojo y el motivo en el aviso;
  - Publicado sin enlace: icono «Vincular publicación» (`launches/missing-release.modal.tsx`).
- **Campana** (B): `notifications/notification.component.tsx`, con los datos de nuestra API.
- **Ajustes** (B): ruta `/settings` con la pestaña General y las dos casillas de Postiz.

## 11. Pruebas

- **Frontera del trabajo:** los tests de Go ejecutan `publish_value` y el barrido directamente, con reloj inyectado y el Telegram falso. No esperan al planificador de River. Que el trabajo se encole con la fecha correcta se comprueba sobre las tablas de River.
- **Telegram falso:** apunta lo que recibe (método, chat, texto, pie, respuesta y ficheros) y se le puede ordenar que falle la próxima llamada con un código o un corte de conexión.
- **Resend falso** (B): en `postik-fakes`, apunta los correos recibidos.
- **Tests de pantalla:** usan «Publicar ya», porque el planificador real no se puede adelantar. Que un post salga a su hora lo cubre Go.

## 12. Casos

### A · Publicación

## S05.1 Programar encola la publicación a su hora, sin duplicarla

Cubre: F9; F11, inicio; §3, encolado.

- **Dado** un canal de Telegram.
- **Cuando**:
  - se programa un post para mañana a las 10:00;
  - se guarda otro con «Publicar ya»;
  - se guarda un borrador.
- **Entonces**:
  - hay un `publish_value` en la posición 0 para mañana a las 10:00 y otro para el minuto actual;
  - el borrador no encola nada;
  - volver a guardar el primero con `update` no crea otro trabajo.

## S05.2 Llegada la hora, sale a Telegram y queda Publicado con enlace

Cubre: F11, pasos 3 y 5; §5, texto y enlace.

- **Dado** un post programado cuyo HTML es `<p>Hola <strong>mundo</strong></p><ul><li><p>uno</p></li></ul>`, en un canal público `@postik_demo` y en otro privado `-1001234567890`.
- **Cuando** se ejecuta su `publish_value`.
- **Entonces**:
  - Telegram recibe `sendMessage` con `parse_mode` HTML y el texto `Hola <b>mundo</b>\n• uno`;
  - los posts quedan Publicados;
  - los enlaces son `https://t.me/postik_demo/<id>` y `https://t.me/c/1234567890/<id>`.

## S05.3 Los medios se suben como fichero, y el tipo decide la llamada

Cubre: F11, paso 3; §5, medios.

- **Dado** un post con una imagen, otro con un vídeo y otro con 12 imágenes.
- **Cuando** se publican.
- **Entonces**:
  - el primero va por `sendPhoto`, con el texto como pie;
  - el segundo, por `sendVideo`;
  - el tercero, por dos `sendMediaGroup` de 10 y 2, con el pie solo en el primer medio;
  - todos los ficheros llegan en el cuerpo `multipart`, con el contenido del disco;
  - el enlace es el del primer mensaje.

## S05.4 Los comentarios salen en orden, tras su retardo, como respuesta

Cubre: F11, paso 3; §4, paso 6; §5, comentarios.

- **Dado** un post con el principal y dos comentarios con retardos de 5 y 0 minutos.
- **Cuando** se publica el principal.
- **Entonces**:
  - se encola el primer comentario para 5 minutos después;
  - al ejecutarlo, sale como respuesta al principal y se encola el segundo para ese mismo minuto;
  - el segundo sale como respuesta al primero.

## S05.5 Si el post cambió, el trabajo no hace nada

Cubre: F11, paso 1; §3, nada se cancela.

- **Dado** un post programado para las 10:00, con su trabajo encolado.
- **Cuando** antes de ejecutarse el trabajo el post:
  - se reprograma a las 11:00;
  - o pasa a borrador;
  - o se borra.
- **Entonces**:
  - el trabajo de las 10:00 no llama a Telegram ni cambia el post;
  - en el primer caso hay un trabajo nuevo para las 11:00.

## S05.6 Con el canal desactivado o pendiente de reconexión no se llama a Telegram

Cubre: F11, paso 2.

- **Dado** un post programado.
- **Cuando** su canal se desactiva, o se marca pendiente de reconexión, y se ejecuta el trabajo.
- **Entonces**:
  - el post queda en Error con `channel_disabled` o `channel_refresh`;
  - Telegram no recibe nada.

## S05.7 Si Telegram rechaza el post, queda en Error sin reintento

Cubre: F11, reintentos; §4, paso 5.

- **Dado** un post programado.
- **Cuando** Telegram responde 400 con «Bad Request: message is too long».
- **Entonces**:
  - el post queda en Error con esa descripción;
  - el trabajo no se reintenta;
  - no queda ningún envío apuntado como `sending`.

## S05.8 Si la llamada no llegó a empezar, se reintenta hasta 5 veces

Cubre: F11, reintentos; §4, paso 5.

- **Dado** un post programado.
- **Cuando**:
  - en el primer intento Telegram responde 429 y en el segundo responde bien;
  - en otro post, Telegram no acepta conexiones en ninguno de los cinco intentos.
- **Entonces**:
  - el primero queda Publicado, con un solo mensaje en Telegram;
  - el segundo sigue Programado tras los cuatro primeros intentos y queda en Error `unreachable` tras el quinto.

## S05.9 Si no se sabe si llegó, no se reenvía nunca

Cubre: F11, reintentos; §4, pasos 3 y 5.

- **Dado** un post programado.
- **Cuando**:
  - Telegram corta la conexión después de recibir la petición;
  - en otro post, hay un envío en `sending` de un intento anterior (el proceso se cayó a mitad) y el trabajo se vuelve a ejecutar.
- **Entonces**:
  - los dos quedan en Error con «No se ha podido confirmar la publicación»;
  - en el segundo, Telegram no recibe nada nuevo.

## S05.10 Un comentario que falla no deshace el principal

Cubre: F11; §4, paso 7.

- **Dado** un post con el principal ya publicado y dos comentarios pendientes.
- **Cuando** Telegram rechaza el primer comentario.
- **Entonces**:
  - el post sigue Publicado, con su enlace;
  - se guarda el error del comentario;
  - el segundo comentario no se encola.

## S05.11 Un trabajo repetido no publica dos veces

Cubre: F11, reintentos; §4, paso 3.

- **Dado** un post cuyo principal ya salió (envío `sent`).
- **Cuando** su `publish_value` de la posición 0 se ejecuta otra vez.
- **Entonces**:
  - Telegram no recibe nada;
  - el post sigue Publicado con el mismo enlace.

## S05.12 El barrido relanza los vencidos de las últimas 48 horas, y solo esos

Cubre: F13.

- **Dado**:
  - un post programado hace 3 horas, sin trabajo pendiente;
  - otro, hace 3 días;
  - otro, hace 1 hora en un canal desactivado;
  - otro, hace 1 hora con su trabajo aún pendiente.
- **Cuando** se ejecuta el barrido.
- **Entonces**:
  - solo el primero recibe un `publish_value` nuevo;
  - el de hace 3 días sigue Programado.

## S05.13 Republicar envía de nuevo como publicación nueva

Cubre: F10, republicar; §4, republicar.

- **Dado** un post publicado.
- **Cuando** se reprograma con `republish: true` para dentro de un minuto y se ejecuta su trabajo.
- **Entonces**:
  - Telegram recibe un mensaje nuevo;
  - el post vuelve a estar Publicado, con el enlace nuevo.

## S05.14 Vincular una publicación sin enlace

Cubre: F14.

- **Dado** un post Publicado sin enlace y otro con enlace.
- **Cuando** se envía `https://t.me/postik_demo/7` al primero, `http://…` al primero y una URL al segundo.
- **Entonces**:
  - el primero queda con su enlace;
  - la URL `http` responde 400;
  - el segundo responde 409 `release_not_missing`.

## S05.15 «Publicar ya» sale en Telegram y la tarjeta enlaza a la publicación

Cubre: F9, «Publicar ya»; F11; §10.

- **Dado** una persona con un canal de Telegram, en la vista de semana.
- **Cuando** crea un post y pulsa «Publicar ya».
- **Entonces**:
  - el Telegram falso recibe el mensaje;
  - la tarjeta del post pasa a publicada y su vista previa lleva al enlace del mensaje.

## S05.16 Un post en Error se ve en rojo con su motivo

Cubre: F11, fin; §6.6, tarjeta; §10.

- **Dado** el Telegram falso preparado para rechazar el próximo envío con «Bad Request: chat not found».
- **Cuando** se publica un post con «Publicar ya».
- **Entonces**:
  - la tarjeta tiene el borde rojo;
  - al pasar el ratón, se ve el motivo.

### B · Avisos

Los casos del bloque B (S05.17 en adelante) entran con su PR.
