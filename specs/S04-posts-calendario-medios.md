# S04 · Posts, calendario y medios

Cubre F9, F10 y F15, §6.5–§6.7 y la fila de Telegram de §6.4. Completa además la acción «Crear post» del menú de un canal, que quedó pendiente en S03.

La publicación de verdad llega en S05: aquí «Programado» solo guarda el estado y la fecha. Cuando un caso habla de lanzar o anular la publicación pendiente, se refiere a lo que S05 conectará sobre este estado.

Se entrega en dos PRs apilados: primero los medios (bloque A) y después posts y calendario (bloque B).

## 1. Objetivo y límites

Al terminar S04:

- **Biblioteca de medios (`/media`):** subir, buscar, paginar y borrar.
- **Calendario de `/launches`:**
  - vistas de día, semana, mes y lista;
  - navegación y filtro por cliente;
  - tarjetas de los posts con su estado y etiqueta.
- **Editor de posts:**
  - canales, contenido global y personalizado por canal;
  - valores encadenados con retardo e «Insertar medio»;
  - vista previa con el exceso de caracteres en rojo;
  - etiquetas, fecha y las cuatro acciones de F9.
- **Sobre un post existente:**
  - abrirlo y «Actualizar» o «Programar»;
  - duplicar, borrar y arrastrar a otro hueco.

Fuera de S04, además de lo que ya excluye la funcional (§9):

- los ajustes propios de cada red, que llegan con sus redes en S06 y S07; Telegram no tiene;
- las menciones, que ninguna red de la v1 usa antes de S07.

## 2. Configuración

Ninguna variable nueva. Los medios se guardan en `POSTIK_STORAGE_DIR/media/AAAA/MM/` y se sirven en `/uploads/media/…`.

## 3. Medios

- **Subida:** `POST /api/v1/media` recibe un fichero por petición, en `multipart/form-data`. La pantalla sube los de una tanda uno detrás de otro y enseña el progreso de cada uno.
- **Tipo:** se detecta por las firmas del contenido, nunca por la extensión ni por el `Content-Type` que declara el cliente.

  | Tipo | Firma |
  |---|---|
  | JPEG | `FF D8 FF` |
  | PNG | `89 50 4E 47 0D 0A 1A 0A` |
  | GIF | `GIF87a` o `GIF89a` |
  | WebP | `RIFF`, 4 bytes y `WEBP` |
  | BMP | `BM` |
  | TIFF | `II*\0` o `MM\0*` |
  | AVIF | caja `ftyp` con marca `avif` o `avis` |
  | MP4 | caja `ftyp` con cualquier otra marca |

- **Tamaño:** como mucho 10 MB por imagen y 1 GB por vídeo. Se mide contando lo que se lee, no por la cabecera `Content-Length`. Si el límite se supera a mitad de lectura, se corta y se borra lo escrito.
- **Borrado:** es lógico. El medio sale de la biblioteca, pero su fichero sigue sirviéndose para no romper los posts que ya lo usan.
- **Metadatos:** el texto alternativo y, en los vídeos, una miniatura (imagen subida por el mismo camino) con el segundo del vídeo del que sale.

## 4. Posts

- **Grupo:** cada envío del editor crea un grupo en la organización, con una entrada (post) por canal. El grupo guarda las etiquetas y el origen (`web` ahora; `mcp` en S08).
- **Valores:** cada post guarda sus valores en orden:
  - el principal y los encadenados (hilo o comentarios);
  - de cada uno, el HTML del editor, el retardo en minutos y los medios (ID, URL, texto alternativo y miniatura).
- **Estados:** `draft`, `scheduled`, `published` y `error`. En S04 solo se llega a `draft` y `scheduled`; los otros dos los pone S05.
- **Validación** en `internal/core/posts`, con funciones puras:
  - al programar o publicar ya:
    - cada post tiene texto o medio en su valor principal y ningún valor vacío;
    - la longitud del texto sin etiquetas cabe en el límite de la red; en Telegram son 4.096, contados como el navegador (unidades UTF-16);
    - la fecha no ha pasado, con un margen de 1 minuto;
  - un borrador solo exige que el valor principal no esté vacío;
  - los canales tienen que ser de la organización, no estar desactivados ni en paso intermedio;
  - las etiquetas y los medios, de la organización.
- **«Publicar ya»:** la fecha la pone el servidor al minuto actual. La que envía el cliente se ignora.
- **Siguiente hueco libre:**
  - franjas: las del canal si se indica uno; si no, la unión de las de todos los canales activos de la organización;
  - desde hoy (UTC), día a día, el primer minuto futuro de las franjas en el que la organización no tenga ya un post, en cualquier canal (como Postiz);
  - se buscan como mucho 365 días.
- **Editar un post:** se envían sus valores, la fecha y las etiquetas del grupo, con uno de dos modos:
  - `update` cambia los detalles sin tocar el estado;
  - `schedule` lo pasa a `scheduled` con la fecha nueva.

  Si el post está publicado, `schedule` exige `republish: true`; si no, responde 409 `republish_required`.
- **Mover (arrastrar):** cambia solo la fecha, con los mismos dos modos y la misma regla de republicar. Una fecha pasada responde 400. Un borrador sigue siendo borrador.
- **Duplicar:** no toca el servidor. La pantalla abre el editor con el texto y los medios del post, sin ajustes ni etiquetas y con la fecha del siguiente hueco libre. Al guardar, se crea un grupo nuevo.
- **Borrar:** borra el grupo entero. Borrar un canal borra sus posts (F8).
- **Etiquetas:** nombre (único en la organización) y color. Borrar una etiqueta la quita de los grupos que la tenían.

## 5. Modelo de datos

| Tabla | Campos |
|---|---|
| `media` | `id`, `organization_id`, `name`, `path`, `kind` (`image` o `video`), `mime`, `size`, `alt`, `thumbnail_path`, `thumbnail_seconds`, `created_at`, `deleted_at` |
| `tags` | `id`, `organization_id`, `name` (único en la organización), `color`, `created_at` |
| `post_groups` | `id`, `organization_id`, `origin`, `created_by`, `created_at` |
| `post_group_tags` | `group_id`, `tag_id` |
| `posts` | `id`, `organization_id`, `group_id`, `channel_id` (se borra con el canal), `status`, `publish_at`, `post_values` (JSON), `settings` (JSON), `release_url`, `error`, `created_at`, `updated_at` |

## 6. Frontera de la API

Todo actúa sobre la organización activa. Lo que es de otra organización responde 404.

| Método y ruta | Qué hace |
|---|---|
| `GET /api/v1/media?page=&search=` | Biblioteca: 18 por página, lo más reciente primero, buscando por nombre original |
| `POST /api/v1/media` | Sube un fichero (§3). Errores: `unsupported_type` (415), `too_large` (413), `unreadable` (400) |
| `PUT /api/v1/media/{id}` | Texto alternativo y miniatura (`thumbnailMediaId`, `thumbnailSeconds`) |
| `DELETE /api/v1/media/{id}` | Borrado lógico |
| `GET /api/v1/tags` · `POST /api/v1/tags` · `PUT /api/v1/tags/{id}` · `DELETE /api/v1/tags/{id}` | Etiquetas |
| `GET /api/v1/posts?from=&to=&customer=` | Posts del rango para el calendario, con canal, estado, etiquetas y extracto |
| `GET /api/v1/posts/list?page=&status=` | Vista de lista: 100 por página; `all`, `scheduled`, `draft` o `published` |
| `GET /api/v1/posts/next-slot?channelId=` | Siguiente hueco libre |
| `POST /api/v1/posts` | Crea un grupo: `type` (`schedule`, `draft` o `now`), `publishAt`, `tags` y, por canal, `values` y `settings` |
| `GET /api/v1/posts/{id}` | Un post con su canal y las etiquetas del grupo, para editarlo |
| `PUT /api/v1/posts/{id}` | Edita (§4): `mode`, `republish`, `publishAt`, `values`, `settings` y `tags` |
| `PUT /api/v1/posts/{id}/date` | Mueve (§4): `publishAt`, `mode` y `republish` |
| `DELETE /api/v1/posts/{id}/group` | Borra el grupo del post |

Los errores de validación responden 400 con `code: invalid_post` y la lista de problemas, cada uno con su canal, la posición del valor y un código:

| Código | Mensaje en pantalla |
|---|---|
| `empty` | «El post debe tener al menos un carácter o una imagen» |
| `too_long` | «El post es demasiado largo» |
| `past_date` | «La fecha ya ha pasado» |
| `channel_unavailable` | El canal ya no se puede usar |

## 7. Interfaz

Portada de Postiz (constitución, §2), con estos cambios:

- **Calendario:** `launches/calendar.tsx`, `calendar.context.tsx` y `filters.tsx`. Los datos vienen de nuestra API con TanStack Query. El arrastre usa `react-dnd`, como en Postiz.
- **Editor:** `new-launch/*`, con TipTap como en Postiz. Se quitan:
  - el copiloto de IA, generar imagen o vídeo y «Diseñar medio»;
  - las firmas, los sets y «Elegir un set»;
  - «Repetir», la vista previa pública, los comentarios internos y las menciones.
- **Estado del editor:** el `store.ts` de Postiz, que usa zustand, se porta sobre una primitiva propia de almacén (`web/src/lib/store.ts`, con `useSyncExternalStore`). Es estado de pantalla, no acceso a datos (constitución §2).
- **Etiquetas:** `tags.component.tsx`.
- **Medios:** `media/media.component.tsx`. Uppy se sustituye por la misma zona de arrastre sobre un `<input type="file">` nativo, con subida por `XMLHttpRequest` para ver el progreso.
- **«Crear post»:** vuelve al menú contextual del canal, con el canal preseleccionado.

## 8. Pruebas

- **Reglas de mover:** la regla de hueco pasado y la de actualizar o reprogramar se prueban por la API (`PUT /posts/{id}/date`).
- **Gesto de arrastrar:** no se automatiza, porque Playwright no mueve bien `react-dnd` con su backend de HTML5. Queda para la prueba manual de Rubén.
- **Tests de pantalla:** tres casos, S04.21–S04.23.

## 9. Casos

### A · Medios

## S04.1 Subir una imagen la deja en la biblioteca con una URL pública

Cubre: F15 pasos 1–3; §6.7.

- **Dado** una persona con sesión.
- **Cuando** sube `foto.png`, una imagen PNG.
- **Entonces**:
  - responde con el medio: nombre `foto.png`, tipo `image` y `image/png`;
  - su URL empieza por `/uploads/media/` y sirve los mismos bytes;
  - aparece el primero en la biblioteca.

## S04.2 El tipo sale del contenido, no del nombre

Cubre: F15 paso 2; §6.7.

- **Dado** ficheros con las firmas de JPEG, PNG, GIF, WebP, BMP, TIFF, AVIF y MP4, un PNG llamado `video.mp4` y un texto llamado `foto.jpg`.
- **Cuando** se suben.
- **Entonces**:
  - los ocho primeros se aceptan con su tipo real;
  - `video.mp4` queda como `image/png`;
  - `foto.jpg` responde 415 `unsupported_type`.

## S04.3 El límite de tamaño se cumple aunque el cliente mienta

Cubre: §6.7.

- **Dado** una imagen de 10 MB y 1 byte, enviada sin `Content-Length`.
- **Cuando** se sube.
- **Entonces** responde 413 `too_large`, no queda ningún fichero en disco y la biblioteca sigue vacía.

## S04.4 La biblioteca pagina de 18 en 18 y busca por nombre original

Cubre: F15, otras acciones.

- **Dado** 20 imágenes subidas, de las cuales dos se llaman `playa-1.jpg` y `playa-2.jpg`.
- **Cuando** se piden las páginas 1 y 2, y se busca `playa`.
- **Entonces** la primera trae 18, las más recientes primero; la segunda, 2; y la búsqueda, solo las dos de playa.

## S04.5 Borrar un medio lo saca de la biblioteca pero su URL sigue sirviendo

Cubre: F15; §6.7, borrado.

- **Dado** un medio subido.
- **Cuando** se borra.
- **Entonces** no aparece en la biblioteca y su URL sigue devolviendo el fichero.

## S04.6 Texto alternativo y miniatura de un vídeo

Cubre: F15, en el editor; §6.7, metadatos.

- **Dado** un vídeo y una imagen subidos.
- **Cuando** se guarda el texto alternativo «Demo» y la imagen como miniatura del segundo 3.
- **Entonces** la biblioteca muestra el vídeo con ese texto, la URL de la miniatura y el segundo 3.

## S04.7 Los medios de otra organización no se ven ni se tocan

Cubre: constitución §8.

- **Dado** un medio de Bruno.
- **Cuando** Ana lista la biblioteca, lo edita y lo borra.
- **Entonces** no lo ve, y editar y borrar responden 404.

### B · Posts y calendario

Los casos S04.8–S04.23 entran en el PR de posts y calendario, apilado sobre este.
