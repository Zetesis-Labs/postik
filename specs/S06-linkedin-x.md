# S06 · LinkedIn y X

Cubre F5, F7 y la renovación de F12, el paso 4 de F11, las filas de LinkedIn (perfil y página) y X de §6.4 y la configuración de X de §6.3. Es la primera vertical con OAuth: deja la base que usarán Meta y YouTube en S07.

Se entrega en tres PRs apilados:
- **Bloque A, OAuth y LinkedIn:** conectar, reconectar, tokens cifrados, renovación, aviso de caducidad y publicar en LinkedIn, tanto en el perfil como en la página, con su paso intermedio.
- **Bloque B, X:** conectar, publicar posts e hilos con medios, ajustes del editor, «Verificada» y quitar enlaces.
- **Bloque C, extras:** el carrusel de LinkedIn y los artículos de X.

## 1. Objetivo y límites

Al terminar S06:

- **Añadir canal:** la rejilla ofrece LinkedIn, LinkedIn Page y X cuando hay credenciales. Elegir una lleva a la red a pedir permisos y, al volver, el canal aparece en la barra lateral.
- **LinkedIn Page:** tras autorizar, una rejilla con las páginas que administra la persona; elige una y pulsa «Guardar».
- **Reconectar:** un canal que necesita reconexión lleva el «!» rojo; pulsarlo, o «Reconectar canal» en su menú, repite la autorización sin perder posts ni ajustes.
- **Publicar:** un post programado sale a su hora en LinkedIn (perfil o página) y en X, con medios y con sus comentarios o su hilo, y queda Publicado con el enlace real.
- **Editor:** los ajustes de X (quién puede responder, comunidad, «hecho con IA», «colaboración pagada», artículo) y de LinkedIn (carrusel), y el contador de X con el recuento de X.
- **Tokens:** se guardan cifrados, se renuevan cuando la red lo permite y, si no se puede, el canal pasa a necesitar reconexión y se avisa.

Fuera de S06:

- **Menciones:** en LinkedIn, una `@` escrita sale como texto literal. En X, `@usuario` funciona sin hacer nada, porque X las reconoce en el texto.
- **Meta y YouTube:** son S07.
- **Rotar la clave de cifrado:** si se cambia `POSTIK_ENCRYPTION_KEY`, todos los canales con OAuth tienen que reconectarse.

## 2. Configuración

| Variable | Obligatoria | Qué es |
|---|---|---|
| `POSTIK_ENCRYPTION_KEY` | Sí, si hay alguna red con OAuth configurada | 32 bytes en base64. Cifra los tokens (constitución §8). Sin ella, `postik serve` no arranca si hay credenciales de LinkedIn o X |
| `POSTIK_LINKEDIN_CLIENT_ID` y `POSTIK_LINKEDIN_CLIENT_SECRET` | No | App de LinkedIn. Sin ellas, ni LinkedIn ni LinkedIn Page aparecen en la rejilla. La página necesita que la app tenga aprobado el producto *Community Management API*; la de suntzu lo tiene, porque Postiz pide esos permisos incluso para el perfil |
| `POSTIK_LINKEDIN_VERSION` | No, `202609` | Cabecera `Linkedin-Version`. LinkedIn mantiene cada versión un año como mínimo: hay que subirla al menos una vez al año |
| `POSTIK_LINKEDIN_AUTH_URL` y `POSTIK_LINKEDIN_API_URL` | No, `https://www.linkedin.com` y `https://api.linkedin.com` | Para apuntar al LinkedIn falso |
| `POSTIK_X_API_KEY` y `POSTIK_X_API_SECRET` | No | Claves de consumidor de la app de X (OAuth 1.0a). Sin ellas, X no aparece en la rejilla |
| `POSTIK_X_API_URL` | No, `https://api.x.com` | Para apuntar al X falso |
| `POSTIK_X_STRIP_LINKS` | No, `false` | Quita los enlaces de lo que se publica en X (§6.3: es de la instancia). En X un post con enlace cuesta más de diez veces que uno sin él |

**URL de vuelta** que hay que registrar en cada app: `<POSTIK_PUBLIC_URL>/api/v1/channels/<red>/callback`, con `<red>` igual a `linkedin`, `linkedin-page` o `x`. En pelayo: `https://suntzu.nexolabs.dev/api/v1/channels/linkedin/callback`, y así con cada una.

## 3. Conexión por OAuth (F5, F7)

1. **Inicio:** `POST /api/v1/channels/{provider}/authorizations`, con `channelId` si es una reconexión, crea una autorización pendiente y devuelve la `url` de la red. La pantalla navega a ella.
   - La autorización guarda la organización activa, la persona, la red, el canal que se reconecta y la hora.
   - **LinkedIn:** el `state` es un valor aleatorio de 32 bytes. Los permisos pedidos son `openid profile w_member_social` para el perfil y `openid profile r_organization_social w_organization_social rw_organization_admin` para la página.
   - **X:** primero pide un *request token* (`POST /oauth/request_token` con `oauth_callback`) y guarda su secreto, cifrado. La autorización se identifica por ese `oauth_token`. La URL es `/oauth/authenticate?oauth_token=…`.
2. **Vuelta:** la red redirige el navegador a `GET /api/v1/channels/{provider}/callback`. Siempre se termina redirigiendo a `/launches`: con `?added=<canal>` si fue bien, con `?continue=<canal>` si falta elegir página, o con `?oauth_error=<código>`.

   | Comprobación | Si falla, `oauth_error` |
   |---|---|
   | La red no devolvió error (la persona canceló) | `denied` |
   | La autorización existe, es de la persona de la sesión y no se ha usado | `invalid_state` |
   | Tiene menos de una hora | `expired` |
   | El intercambio del código (o del *verifier* en X) funciona | `exchange_failed` |
   | En LinkedIn, el `scope` devuelto incluye todos los permisos pedidos | `missing_permissions` |
   | En una reconexión, la cuenta autorizada es la del canal | `wrong_account` |

   La autorización se borra al usarla, vaya bien o mal.
3. **Identidad:**
   - **LinkedIn:** `GET /v2/userinfo`. El `sub` es el ID de la persona, y se usan `name` y `picture`. No hay nombre de usuario.
   - **X:** `GET /2/users/me` con `user.fields=profile_image_url,verified`. Se usan `id`, `name`, `username` y la foto; `verified` da el valor inicial de «Verificada».
   - La foto se descarga y se guarda en `avatars/`, como en Telegram, porque las URLs de las redes caducan.
4. **Canal:**
   - **Perfil de LinkedIn y X:** se crea el canal o, si esa cuenta ya estaba en la organización, se actualiza como en Telegram (§6.3, unicidad): nombre, foto y tokens nuevos, y se conservan el cliente, las franjas y si está desactivado. Al conectar o reconectar, el canal deja de necesitar reconexión.
   - **LinkedIn Page:** se crea un canal en paso intermedio, con `external_id` `pending:<sub>` y los tokens de la persona, que no se puede usar hasta elegir la página (paso 5). Si esa persona ya tenía uno a medias en la organización, se reutiliza.
5. **Elegir la página (F5, paso 5):**
   - `GET /api/v1/channels/{id}/pages` lista las páginas en las que la persona es `ADMINISTRATOR` o `CONTENT_ADMINISTRATOR`, con la aprobación vigente: `GET /rest/organizationAcls?q=roleAssignee&state=APPROVED`, y el nombre y el logo de cada una con `GET /v2/organizations/{id}` y la proyección del logo, como Postiz. Las llamadas a `/v2` llevan solo el token; las cabeceras de versión son de `/rest`.
   - `PUT /api/v1/channels/{id}/page` con `pageId` comprueba que la página está en esa lista (si no, 403 `page_not_administered`) y deja el canal con el ID de la organización, su nombre, su `vanityName` como usuario y el logo guardado en `avatars/`. El canal sale del paso intermedio.
   - Si la organización ya tenía esa página en otro canal, ese canal recibe los tokens nuevos y deja de necesitar reconexión, el canal intermedio se borra y se responde con el canal existente. Es la unicidad de §6.3.
6. **Reconexión (F7):**
   - **Perfil de LinkedIn y X:** la cuenta autorizada tiene que ser la del canal.
   - **LinkedIn Page:** la persona que autoriza tiene que administrar la página del canal, según la lista del paso 5. No hay paso intermedio.
   - Si no se cumple, no se toca nada y se responde `wrong_account`. Postiz lo trataría como una migración, pero manda la funcional (constitución §2).

## 4. Tokens y renovación (F11, paso 4; F12)

- **Guardado:** access token, secreto (X), refresh token y fechas de caducidad, cifrados con AES-256-GCM y la clave de `POSTIK_ENCRYPTION_KEY`. El ID del canal va como dato asociado, para que un token no valga pegado en otro canal.
- **Vigencia:**
  - **LinkedIn:** el access token dura 60 días. Solo hay refresh token si la app tiene aprobados los refresh tokens programáticos; si no, a los 60 días hay que reconectar.
  - **X:** con OAuth 1.0a el token no caduca.
- **Renovación bajo demanda:**
  - antes de llamar a la red, si el access token caduca en menos de 5 minutos y hay refresh token, se renueva;
  - si la red responde 401, se renueva y se repite la llamada una sola vez;
  - la renovación de un canal se hace con un bloqueo de fila, para que dos trabajos no la hagan a la vez.
- **Si no se puede renovar** (no hay refresh token, la red lo rechaza o vuelve a responder 401):
  - el canal pasa a necesitar reconexión;
  - el post que se estaba publicando queda en Error `channel_refresh`;
  - se crea la notificación `refresh_failed`, de tipo `info`: el correo sale a todos los miembros aunque tengan desactivados los de fallo (F12).
- **Si la renovación no llega a conectar** con la red, cuenta como un envío que no empezó (S05 §4): se reintenta.
- **Aviso de caducidad** **[Cambio de la funcional]:** F12 lo prevé para Facebook e Instagram. Aquí se aplica a cualquier canal cuyo token caduca y no tiene refresh token, y en la práctica eso es LinkedIn sin refresh tokens. Un trabajo horario, `token_expiry`:
  - avisa una vez, 7 días antes, con la notificación `channel_expiring` (tipo `info`) y los días que quedan;
  - cuando el token ya ha caducado, marca el canal como pendiente de reconexión y crea `refresh_failed`;
  - al reconectar, la cuenta vuelve a empezar.

## 5. Publicar en una red con OAuth

Vale todo S05 §4: el turno, el canal, los envíos apuntados en `post_deliveries`, los 5 intentos y el «no se sabe si llegó». Cambia esto:

- **Cada red es un publicador** con dos operaciones:
  - publicar un valor, que devuelve el ID y el enlace;
  - decir a qué responde un comentario.

  Telegram pasa a ser uno más.
- **Punto sin retorno:** es la llamada que crea la publicación (`POST /rest/posts`, `POST /rest/socialActions/…/comments` o `POST /2/tweets`).
  - Todo lo anterior (renovar el token o subir los medios) se puede repetir sin publicar dos veces. Un fallo ahí cuenta como un envío que no empezó y se reintenta; un reintento vuelve a subir los medios.
  - Un tiempo agotado o un 5xx en la llamada que crea deja el post como no confirmado (`unconfirmed`).
- **A qué responde cada comentario:**
  - en LinkedIn, siempre al post principal, porque los comentarios no se anidan, como en Postiz;
  - en X, al valor anterior, formando un hilo;
  - en Telegram, al anterior, como hasta ahora.
- **Texto:** LinkedIn y X no admiten HTML. El HTML del editor se convierte a texto plano, como el editor `normal` de Postiz:
  - cada párrafo termina en salto de línea, y al final no queda ninguno;
  - cada elemento de lista empieza por «- »;
  - un enlace se queda en su URL;
  - la negrita pasa a letras negritas de Unicode (`𝗵𝗼𝗹𝗮`) y el subrayado, a letras con subrayado combinado.

## 6. LinkedIn

- **Autor:** `urn:li:person:<sub>` en el perfil y `urn:li:organization:<id>` en la página.
- **Cabeceras de la API REST:** `Linkedin-Version: <POSTIK_LINKEDIN_VERSION>` y `X-Restli-Protocol-Version: 2.0.0`.
- **Texto (`commentary`):** se escapan los caracteres reservados del formato de LinkedIn con `\`: `\ < > # ~ _ | [ ] * ( ) { } @`. Como en Postiz, un `#` escrito sale literal, no como hashtag enlazado.
- **Medios:** se suben desde el disco.

  | Medios | Subida | En el post |
  |---|---|---|
  | Una imagen | `POST /rest/images?action=initializeUpload` y `PUT` del fichero a la `uploadUrl` | `content.media.id` |
  | De 2 a 20 imágenes | Cada una como la anterior | `content.multiImage.images` |
  | Un vídeo | `POST /rest/videos?action=initializeUpload`, `PUT` de cada trozo guardando su `ETag`, y `POST /rest/videos?action=finalizeUpload` **[Supuesto]**; después se espera a que el vídeo esté `AVAILABLE`, como mucho 10 minutos | `content.media.id` |
  | Carrusel (C) | Las imágenes se unen en un PDF, una por página y del tamaño de la mayor, y se sube con `POST /rest/documents?action=initializeUpload` | `content.media.id` y `content.media.title` con el nombre del carrusel |

- **Post:** `POST /rest/posts` con `visibility: PUBLIC`, `distribution.feedDistribution: MAIN_FEED` y `lifecycleState: PUBLISHED`. El ID llega en la cabecera `x-restli-id`.
- **Enlace:** `https://www.linkedin.com/feed/update/<urn>/`. Si LinkedIn responde 201 sin `x-restli-id`, el post queda Publicado sin enlace (F14) y sus comentarios no se envían, porque no hay a qué responder.
- **Comentarios:** `POST /rest/socialActions/<urn del principal>/comments`, con `actor` (el autor) y `message.text`. Solo texto (§6.4).
- **Errores:**

  | Respuesta | Qué es |
  |---|---|
  | 401 | Token caducado o revocado: renovar (§4) |
  | 429 | No empezó: se reintenta |
  | 400, 403 o 422 | Rechazo: Error con el `message` de LinkedIn |
  | 5xx o tiempo agotado en la llamada que crea | No confirmado |

## 7. X

- **Firma:** todas las llamadas llevan OAuth 1.0a con HMAC-SHA1, con las claves de consumidor y el token y secreto del canal. La API v2 acepta este modo en `POST /2/tweets` y en la subida de medios.
- **Texto:** el texto plano de §5. Con `POSTIK_X_STRIP_LINKS` se quitan las URLs.
- **Medios:** hasta 4 imágenes, o un vídeo, o un GIF.
  - Se suben por trozos: `POST /2/media/upload/initialize` con la categoría `tweet_image`, `tweet_gif` o `tweet_video`, un `append` por trozo y `finalize`.
  - Si X responde con `processing_info`, se consulta el estado hasta `succeeded`, como mucho 10 minutos.
  - Si el procesado da `failed`, es un rechazo.
- **Post:** `POST /2/tweets` con:
  - `text`;
  - `media.media_ids`;
  - `reply.in_reply_to_tweet_id`, en los elementos del hilo;
  - `reply_settings`, salvo si es `everyone`, que es el valor por defecto de X;
  - `community_id` y `share_with_followers: true`, si hay comunidad;
  - `made_with_ai` y `paid_partnership`, si están marcadas.
- **Enlace:** `https://x.com/<usuario>/status/<id>`.
- **«Verificada»:** es un ajuste del canal (F8, «Ajustes adicionales»). Sube el límite de 280 a 4.000 caracteres.
- **Errores:**

  | Respuesta | Qué es |
  |---|---|
  | 401 | Token revocado. OAuth 1.0a no se renueva: el canal pasa a necesitar reconexión (§4) |
  | 429 | No empezó: se reintenta |
  | 402, o la cuenta sin crédito | Rechazo, con el `detail` de X |
  | 400 o 403 (contenido duplicado, cuenta suspendida…) | Rechazo, con el `detail` de X |
  | 5xx o tiempo agotado en `POST /2/tweets` | No confirmado |

- **Artículos (C):**
  - `post_type: article` crea un borrador con `POST /2/articles/draft` (título, cuerpo convertido del HTML del editor y portada opcional);
  - si el estado es `published`, a continuación `POST /2/articles/{id}/publish`;
  - el enlace es el del artículo.

## 8. Ajustes y validación por red

Los ajustes van en `settings` de cada post, que ya existe desde S04. El núcleo los valida con funciones puras, al guardar y en el servidor, igual que el resto de §6.5.

| Red | Regla | Código |
|---|---|---|
| LinkedIn | Como mucho 3.000 caracteres | `too_long` |
| LinkedIn | Un vídeo va solo, sin otros medios | `video_alone` |
| LinkedIn | Como mucho 20 imágenes | `too_many_media` |
| LinkedIn | Los comentarios no llevan medios | `comment_media` |
| LinkedIn (C) | El carrusel necesita 2 imágenes o más y ningún vídeo | `carousel_media` |
| X | 280 caracteres, o 4.000 si el canal es «Verificada», con el recuento de X | `too_long` |
| X | Como mucho 4 imágenes, o un vídeo o GIF solo | `too_many_media` o `video_alone` |
| X | `who_can_reply_post` es obligatorio salvo en artículos: `everyone`, `following`, `mentionedUsers`, `subscribers` o `verified` | `invalid_settings` |
| X | La comunidad, si hay, es `https://x.com/i/communities/<número>` | `invalid_settings` |
| X (C) | Un artículo lleva título y estado (`draft` o `published`), solo imágenes, y en borrador no lleva comentarios | `invalid_settings` |

**Recuento de X:** las reglas de `twitter-text`. Cada URL cuenta 23; los emojis, y los caracteres fuera de los rangos latinos y de puntuación, cuentan 2; el resto, 1. La pantalla usa la librería `twitter-text`, como Postiz, y el servidor una implementación en Go que pasa los mismos ejemplos.

## 9. Modelo de datos

| Tabla | Campos |
|---|---|
| `oauth_authorizations` | `state` (clave), `organization_id`, `user_id`, `provider`, `channel_id` (reconexión), `secret` (cifrado; el del *request token* de X), `created_at` |
| `channel_credentials` | `channel_id` (clave; se borra con el canal), `access_token`, `access_secret`, `refresh_token` (los tres cifrados), `expires_at`, `refresh_expires_at`, `expiry_warned_for`, `updated_at` |
| `channels` | nuevo `settings` (JSON; `{"verified": true}` en X) |

Las autorizaciones de más de una hora se borran al crear una nueva.

## 10. Frontera de la API

| Método y ruta | Qué hace |
|---|---|
| `GET /api/v1/channels/providers` | Añade `linkedin`, `linkedin-page` y `x`, si tienen credenciales |
| `POST /api/v1/channels/{provider}/authorizations` | `{ "channelId"?: "…" }` → `{ "url": "…" }` |
| `GET /api/v1/channels/{provider}/callback` | Vuelta de la red; redirige a `/launches` (§3) |
| `GET /api/v1/channels/{id}/pages` | Páginas de LinkedIn que administra la persona: `id`, `name` y `picture` |
| `PUT /api/v1/channels/{id}/page` | `{ "pageId": "…" }` → el canal (§3, paso 5) |
| `PUT /api/v1/channels/{id}/settings` | `{ "verified": true \| false }`; solo en X |
| `GET /api/v1/channels` | Añade `settings` |

## 11. Interfaz

Portada de Postiz (constitución §2):

- **Rejilla de «Añadir canal»:** LinkedIn, LinkedIn Page y X, con los iconos de Postiz (`/icons/platforms/<red>.png`). Pulsar lleva a la red.
- **Vuelta a `/launches`:**
  - `added`: el aviso «Canal añadido»;
  - `oauth_error`: el aviso con el error traducido;
  - `continue`: abre la rejilla de páginas (`continue-provider/with-continue-provider.tsx` y `linkedin/linkedin.continue.tsx`), de selección única y con «Guardar».
- **Barra lateral:**
  - el canal en paso intermedio se ve semitransparente y abre la rejilla de páginas;
  - el que necesita reconexión lleva el «!» rojo y el aviso «Canal desconectado, pulsa para reconectar».
- **Menú del canal:**
  - «Reconectar canal», resaltada, solo si hace falta;
  - «Ajustes adicionales», solo en X, abre el modal con el interruptor «Verificada» (`launches/settings.modal.tsx`).
- **Editor:**
  - los ajustes de X (`new-launch/providers/x/x.provider.tsx`: tipo, quién puede responder, comunidad, IA, colaboración pagada; título, estado y portada del artículo en C);
  - los de LinkedIn (`linkedin/linkedin.provider.tsx`: carrusel y su nombre, en C);
  - el contador de X usa su recuento y su límite.

## 12. Pruebas

- **LinkedIn falso** (`internal/testsupport/fakelinkedin`):
  - la página de autorización redirige al momento a la URL de vuelta con un código;
  - se le puede ordenar que la persona cancele, que conceda menos permisos o que el refresh token no valga;
  - imita `userinfo`, `organizationAcls`, `organizations`, las subidas de imágenes, vídeos y documentos, `posts` y `comments`;
  - apunta lo que recibe, incluidas las cabeceras de versión;
  - se le puede ordenar que falle la próxima llamada, con un código o con un corte.
- **X falso** (`internal/testsupport/fakex`):
  - imita el baile OAuth 1.0a y **comprueba la firma** de cada llamada con las claves de consumidor y el secreto del token;
  - imita `users/me`, la subida por trozos con su procesado, `tweets` y los artículos;
  - apunta lo que recibe y falla a demanda, como el de LinkedIn.
- **`postik-fakes`** los sirve en `/linkedin/` y `/x/`, para `make dev` y los tests de pantalla.
- **Recuento de X:** una tabla de ejemplos en `testdata` que pasan el núcleo en Go y la pantalla.

## 13. Casos

### A · OAuth y LinkedIn

## S06.1 La rejilla ofrece solo las redes con credenciales

Cubre: F5, paso 1; S4 de la funcional.

- **Dado** una instancia con credenciales de LinkedIn y sin las de X.
- **Cuando** se piden los proveedores.
- **Entonces**:
  - salen LinkedIn y LinkedIn Page, además de Telegram si tiene bot;
  - X no sale.

## S06.2 Conectar LinkedIn crea el canal con su identidad y los tokens cifrados

Cubre: F5, pasos 2–6; §3; §4, guardado.

- **Dado** una persona en su organización y el LinkedIn falso con la cuenta «Ana García».
- **Cuando** inicia la autorización y el navegador vuelve con el código.
- **Entonces**:
  - la URL de la red lleva el `state`, la URL de vuelta y los permisos del perfil;
  - la vuelta redirige a `/launches?added=<canal>`;
  - el canal es de LinkedIn, se llama «Ana García», tiene la foto guardada en `avatars/` y las tres franjas por defecto;
  - en la base de datos, el access token no aparece en claro;
  - la autorización ya no existe.

## S06.3 Conectar la misma cuenta otra vez actualiza el canal

Cubre: F5, paso 4; §6.3, unicidad.

- **Dado** un canal de LinkedIn con cliente, franjas propias y desactivado.
- **Cuando** se vuelve a conectar la misma cuenta, que ahora se llama «Ana G.».
- **Entonces**:
  - sigue habiendo un solo canal, llamado «Ana G.», con tokens nuevos;
  - conserva el cliente, las franjas y que está desactivado.

## S06.4 Una vuelta inválida no crea canal y explica por qué

Cubre: F5, errores; §3, comprobaciones.

- **Dado** una persona que inicia la autorización de LinkedIn.
- **Cuando** vuelve:
  - habiendo cancelado en la red;
  - con un `state` que no existe o que inició otra persona;
  - pasada más de una hora;
  - con menos permisos de los pedidos.
- **Entonces**:
  - redirige a `/launches` con `oauth_error` `denied`, `invalid_state`, `expired` y `missing_permissions`;
  - no se crea ningún canal.

## S06.5 Reconectar el perfil deja el canal activo, y solo con la misma cuenta

Cubre: F7; §3, reconexión.

- **Dado** un canal de LinkedIn que necesita reconexión, con un post programado.
- **Cuando**:
  - se reconecta con la misma cuenta;
  - en otro canal igual, se reconecta con otra cuenta.
- **Entonces**:
  - el primero deja de necesitar reconexión, con tokens nuevos y el post intacto;
  - el segundo redirige con `wrong_account` y no cambia nada: ni tokens, ni nombre, ni estado.

## S06.6 Conectar LinkedIn Page deja el canal en paso intermedio y lista las páginas

Cubre: F5, paso 5; §3, pasos 4 y 5.

- **Dado** el LinkedIn falso con una persona que administra «Zetesis» y «Nexo Labs», y que es solo analista de una tercera página.
- **Cuando** conecta LinkedIn Page y pide las páginas.
- **Entonces**:
  - la URL de la red lleva los permisos de la página;
  - la vuelta redirige a `/launches?continue=<canal>`;
  - el canal está en paso intermedio y no se ofrece al crear posts;
  - la lista trae «Zetesis» y «Nexo Labs», con su logo, y no la tercera.

## S06.7 Elegir la página deja el canal listo, y solo si la persona la administra

Cubre: F5, paso 5; §3, paso 5.

- **Dado** el canal intermedio de S06.6.
- **Cuando**:
  - se elige una página que la persona no administra;
  - después se elige «Zetesis».
- **Entonces**:
  - lo primero responde 403 `page_not_administered` y el canal sigue en paso intermedio;
  - lo segundo deja el canal activo, llamado «Zetesis», con el ID de la organización, su `vanityName` y el logo en `avatars/`.

## S06.8 Elegir una página que ya estaba conectada actualiza ese canal

Cubre: F5, paso 4; §3, paso 5; §6.3, unicidad.

- **Dado** un canal de la página «Zetesis» que necesita reconexión, con un post programado.
- **Cuando** alguien de la organización conecta LinkedIn Page y elige «Zetesis».
- **Entonces**:
  - la respuesta es el canal que ya existía, que deja de necesitar reconexión y tiene los tokens nuevos;
  - el canal intermedio ya no existe;
  - el post sigue en su canal.

## S06.9 Reconectar la página pide seguir administrándola

Cubre: F7; §3, reconexión.

- **Dado** dos canales de la página «Zetesis» que necesitan reconexión.
- **Cuando**:
  - en el primero, reconecta alguien que la administra;
  - en el segundo, alguien que ya no la administra.
- **Entonces**:
  - el primero vuelve a estar activo, sin pasar por la rejilla de páginas;
  - el segundo redirige con `wrong_account` y no cambia nada.

## S06.10 Un post sale en LinkedIn como texto plano y queda Publicado con enlace

Cubre: F11, pasos 3 y 5; §5, texto; §6.

- **Dado** un post programado cuyo HTML es `<p>Hola <strong>mundo</strong> (#1)</p><ul><li><p>uno</p></li></ul>`, en un canal de perfil y en otro de página.
- **Cuando** se ejecuta su `publish_value`.
- **Entonces**:
  - LinkedIn recibe `POST /rest/posts` con las cabeceras de versión y protocolo y el texto `Hola 𝗺𝘂𝗻𝗱𝗼 \(\#1\)\n- uno`;
  - el autor es `urn:li:person:<sub>` en el perfil y `urn:li:organization:<id>` en la página;
  - los dos quedan Publicados, con el enlace `https://www.linkedin.com/feed/update/<urn>/`.

## S06.11 Los medios se suben desde el disco y el tipo decide el post

Cubre: F11, paso 3; §6, medios.

- **Dado** un post con una imagen, otro con tres y otro con un vídeo de 9 MB.
- **Cuando** se publican.
- **Entonces**:
  - el primero sube la imagen y la usa en `content.media`;
  - el segundo sube las tres y las usa en `content.multiImage`;
  - el tercero sube el vídeo en trozos, lo finaliza con sus `ETag` y lo publica cuando está `AVAILABLE`;
  - LinkedIn recibe el contenido de los ficheros del disco.

## S06.12 Los comentarios de LinkedIn responden al post principal

Cubre: F11, paso 3; §5, a qué responde cada comentario.

- **Dado** un post con el principal y dos comentarios, en un canal de página.
- **Cuando** se publican.
- **Entonces**:
  - los dos comentarios van a `socialActions/<urn del principal>/comments`, en orden;
  - llevan como `actor` la organización.

## S06.13 Un fallo antes del punto sin retorno se reintenta y no publica dos veces

Cubre: F11, reintentos; §5, punto sin retorno.

- **Dado** un post con una imagen.
- **Cuando**:
  - la subida de la imagen responde 503 en el primer intento, y todo va bien en el segundo;
  - en otro post, `POST /rest/posts` corta la conexión después de recibir la petición.
- **Entonces**:
  - el primero queda Publicado, con un solo post en LinkedIn;
  - el segundo queda en Error `unconfirmed` y no se reintenta.

## S06.14 Un token caducado se renueva y el post sale una sola vez

Cubre: F11, paso 4; F12, bajo demanda; §4.

- **Dado** un canal de LinkedIn con refresh token:
  - uno con el access token caducado;
  - otro con el access token vigente, pero con LinkedIn preparado para responder 401 una vez;
  - un tercero igual que el segundo, y con la renovación preparada para responder 503 una vez.
- **Cuando** se publica un post en cada uno.
- **Entonces**:
  - el primero renueva antes de publicar;
  - el segundo renueva tras el 401 y repite la llamada;
  - el tercero no publica en el primer intento, queda Programado y sin envío apuntado, y sale en el segundo;
  - los tres quedan Publicados, con un solo post cada uno, y los dos primeros tienen guardados los tokens nuevos.

## S06.15 Si no se puede renovar, el canal pide reconexión y se avisa a todos

Cubre: F11, otros casos; F12, si la renovación falla; §4.

- **Dado** una organización con Ana, que tiene los correos de fallo desactivados, y dos canales de LinkedIn:
  - uno sin refresh token;
  - otro cuyo refresh token LinkedIn rechaza.
- **Cuando** LinkedIn responde 401 al publicar en cada uno.
- **Entonces**:
  - los dos canales necesitan reconexión;
  - los dos posts quedan en Error `channel_refresh`;
  - hay dos notificaciones `refresh_failed`, y Ana recibe los dos correos.

## S06.16 El aviso de caducidad sale una vez, y al caducar el canal pide reconexión

Cubre: F12, aviso de caducidad; §4.

- **Dado** un canal de LinkedIn sin refresh token cuyo access token caduca dentro de 6 días.
- **Cuando**:
  - se ejecuta `token_expiry`, y otra vez una hora después;
  - pasan 7 días y se ejecuta de nuevo.
- **Entonces**:
  - hay una sola notificación `channel_expiring`, con 6 días, y su correo;
  - después, el canal necesita reconexión y hay una notificación `refresh_failed`.

## S06.17 El servidor rechaza lo que LinkedIn no admite

Cubre: F9, errores; §8.

- **Dado** un canal de LinkedIn.
- **Cuando** se guarda un post:
  - de 3.001 caracteres;
  - con un vídeo y una imagen;
  - con 21 imágenes;
  - con un comentario que lleva una imagen.
- **Entonces** responde 400 con `too_long`, `video_alone`, `too_many_media` y `comment_media`, en ese orden, y no guarda nada.

## S06.18 Conectar LinkedIn desde la rejilla

Cubre: F5; §11.

- **Dado** una persona en `/launches`, con el LinkedIn falso.
- **Cuando** abre «Añadir canal» y elige LinkedIn.
- **Entonces**:
  - vuelve a `/launches` con el aviso «Canal añadido»;
  - el canal aparece en la barra lateral con su nombre y su foto.

## S06.19 Conectar LinkedIn Page elige la página en la rejilla

Cubre: F5, paso 5; §11; funcional §8.3.

- **Dado** una persona en `/launches`, con el LinkedIn falso y dos páginas que administra.
- **Cuando** elige LinkedIn Page, marca «Zetesis» en la rejilla y pulsa «Guardar».
- **Entonces**:
  - al volver de la red se abre la rejilla con las dos páginas;
  - tras guardar, el canal «Zetesis» aparece activo en la barra lateral.

## S06.20 Un canal desconectado se ve y se reconecta

Cubre: F7; F8, «Reconectar»; §11.

- **Dado** un canal de LinkedIn que necesita reconexión.
- **Cuando** la persona pulsa su avatar.
- **Entonces**:
  - antes de pulsar, el avatar lleva el «!» rojo y el aviso;
  - tras la vuelta, el «!» desaparece.

## S06.21 «Publicar ya» sale en LinkedIn y la tarjeta enlaza a la publicación

Cubre: F9, «Publicar ya»; F11; §11.

- **Dado** una persona con un canal de LinkedIn.
- **Cuando** crea un post y pulsa «Publicar ya».
- **Entonces**:
  - el LinkedIn falso recibe el post;
  - la vista previa de la tarjeta lleva a `https://www.linkedin.com/feed/update/…`.

## S06.22 Si LinkedIn publica sin devolver el ID, el post queda Publicado sin enlace

Cubre: F11, otros casos; F14.

- **Dado** un post con el principal y un comentario, y LinkedIn preparado para aceptar el principal sin devolver su ID.
- **Cuando** se publican el principal y el comentario.
- **Entonces**:
  - el post queda Publicado, sin enlace, a la espera de vincularlo a mano (F14);
  - el comentario no se envía y queda como `failed`, con el motivo;
  - se avisa del fallo del comentario.

### B · X (casos que se detallan en su PR)

- **S06.23** Conectar X por OAuth 1.0a: *request token*, vuelta con *verifier*, identidad, «Verificada» inicial y firma válida en cada llamada.
- **S06.24** Un post sale en X como texto plano, con enlace `x.com/<usuario>/status/<id>`, y su hilo encadena cada elemento al anterior.
- **S06.25** Medios de X: hasta 4 imágenes, o un vídeo esperando su procesado; `media_ids` en el post.
- **S06.26** Ajustes de X en el post: `reply_settings` (no se envía con `everyone`), comunidad, `made_with_ai` y `paid_partnership`.
- **S06.27** Con `POSTIK_X_STRIP_LINKS`, las URLs no llegan a X.
- **S06.28** Recuento de X: URL = 23 y emoji = 2; 280, o 4.000 con «Verificada»; el servidor rechaza `too_long`.
- **S06.29** Errores de X: el contenido duplicado queda en Error sin reintento, un 429 se reintenta y un 401 deja el canal pendiente de reconexión.
- **S06.30** (pantalla) Conectar X y publicar un hilo con «Publicar ya».
- **S06.31** (pantalla) «Ajustes adicionales» > «Verificada» sube el contador del editor a 4.000.
- **S06.32** (pantalla) El editor pide «quién puede responder» antes de guardar en X.

### C · Carrusel de LinkedIn y artículos de X (casos que se detallan en su PR)

- **S06.33** Carrusel de LinkedIn: las imágenes salen en un PDF subido como documento, con su nombre; menos de 2 imágenes o un vídeo dan `carousel_media`.
- **S06.34** Artículos de X: borrador y publicación, con título y estado obligatorios; solo imágenes; sin comentarios en borrador.

## 14. Preguntas abiertas

| Id | Pregunta | Hace falta en |
|---|---|---|
| Q1 | **Cerrada (2026-09-28):** Rubén publica en su perfil y en la página desde suntzu, así que la app tiene *Community Management API*. Queda por ver en la primera prueba real si LinkedIn entrega refresh token; si no, se reconecta cada 60 días, con el aviso de §4. | A |
| Q2 | ¿La app de X tiene crédito en el pago por uso? Hoy X cobra cada post: unos 0,015 $ sin enlace y 0,20 $ con enlace. De eso depende activar `POSTIK_X_STRIP_LINKS`. | B |
| Q3 | Artículos de X: exigen X Premium en la cuenta. Si no se van a usar, salen de C. | C |
| Q4 | **[Por comprobar]** Si `w_member_social` basta para comentar como persona. La API de comentarios nombra `w_member_social_feed`. Se confirma en la primera prueba real; si falta, se añade a los permisos pedidos. | A |
