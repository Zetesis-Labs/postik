# S02 · Acceso OIDC y organizaciones

Cubre F1 y F4, la sesión de los miembros de §6.1 y la membresía de §6.2. Las invitaciones (F3) y el equipo llegan en S09. En S02 la entrada por invitación todavía no existe, así que con «exigir invitación» activado solo entra quien ya tenía cuenta.

## 1. Objetivo y límites

Al terminar S02:

- **Acceso:** la pantalla de acceso ofrece «Iniciar sesión con <proveedor>».
- **Primera vez:** una persona entra por OIDC, recibe su propia organización como Propietario y llega a `/launches`.
- **Siguientes veces:** entra en su organización, sin duplicar nada.
- **Selector de organización:** quien pertenece a más de una organización lo ve en la cabecera y cambia de una a otra.
- **Cerrar sesión:** se hace desde el menú lateral, igual que el superadmin, hasta que S09 traiga los Ajustes.
- **`/launches`:** muestra el marco de la aplicación. El calendario llega en S04 y los canales en S03.

## 2. Configuración

| Variable | Obligatoria | Qué es |
|---|---|---|
| `POSTIK_OIDC_ISSUER` | No* | Emisor OIDC. Se descubre la primera vez que alguien pulsa «Iniciar sesión», así que la aplicación arranca aunque el proveedor esté caído |
| `POSTIK_OIDC_CLIENT_ID` | No* | Cliente registrado en el proveedor |
| `POSTIK_OIDC_CLIENT_SECRET` | No* | Secreto del cliente |
| `POSTIK_OIDC_DISPLAY_NAME` | No, `OIDC` | Nombre del proveedor en el botón |
| `POSTIK_REQUIRE_INVITATION` | No, `false` | Opción de instancia «exigir invitación» (§6.1) |

\* Van las tres o ninguna. Sin ellas solo entra el superadmin. Si falta alguna de las tres, la aplicación no arranca y el error nombra la que falta.

La URL de retorno que hay que registrar en el proveedor es `<POSTIK_PUBLIC_URL>/api/v1/auth/oidc/callback`.

## 3. Flujo técnico de F1

1. `GET /api/v1/auth/oidc/login` genera `state`, `nonce` y un verificador PKCE (S256). Guarda el hash del `state`, el `nonce` y el verificador en `oidc_logins`, deja el `state` en la cookie `postik_oidc_state` (10 minutos, `HttpOnly`, `SameSite=Lax`) y redirige al proveedor pidiendo `openid profile email`.
2. `GET /api/v1/auth/oidc/callback`:
   1. exige que el `state` de la URL sea el de la cookie y que el intento exista y tenga menos de 10 minutos; lo borra para que no pueda reutilizarse;
   2. si el proveedor devolvió `error`, vuelve al acceso;
   3. canjea el código con el verificador, valida el `id_token` (emisor, audiencia, caducidad y `nonce`) y lee `sub`, `email`, `name` y `preferred_username`;
   4. decide en el núcleo qué hacer (§4) y aplica la decisión en una transacción;
   5. abre una sesión de miembro y redirige a `/launches`.
3. Los errores vuelven a `/auth/login?error=<código>`, y la pantalla de acceso los muestra traducidos:

| Código | Cuándo |
|---|---|
| `oidc_denied` | El proveedor rechazó o se canceló la autorización |
| `oidc_state` | El `state` no coincide, no existe o ha caducado |
| `oidc_unavailable` | No se pudo hablar con el proveedor |
| `email_required` | Primera entrada sin email en las claims |
| `invitation_required` | La instancia exige invitación y la persona no tiene cuenta |

## 4. Decisiones del núcleo

En `internal/core/identity`, funciones puras:

- **Qué hacer al entrar:** a partir de si la persona ya existe, si trae email, si la instancia exige invitación y si trae invitación (siempre «no» hasta S09), el resultado es uno de estos:
  - entra;
  - se crea con su organización;
  - se deniega por falta de email;
  - se deniega por falta de invitación.

  Quien ya existe entra siempre.
- **Nombre visible:** `name`; si no viene, `preferred_username`; si tampoco, la parte local del email.
- **Nombre de la organización nueva:** el nombre visible de la persona. **[Supuesto]** Postiz se lo pregunta al registrarse; aquí no hay formulario de registro.
- **Organización activa:** la recordada, si la persona sigue siendo miembro; si no, la primera a la que se unió.

## 5. Modelo de datos

| Tabla | Campos |
|---|---|
| `users` | `id`, `issuer`, `subject` (únicos juntos), `email`, `name`, `created_at` |
| `organizations` | `id`, `name`, `created_at` |
| `memberships` | `organization_id`, `user_id`, `role` (`USER`, `ADMIN`, `OWNER`), `created_at` |
| `oidc_logins` | `state_hash`, `nonce`, `code_verifier`, `created_at` |

`sessions.user_id` pasa a referenciar `users` y se borra en cascada.

## 6. Frontera de la API

| Método y ruta | Qué hace |
|---|---|
| `GET /api/v1/instance` | Añade `oidc: { name }` si hay proveedor configurado |
| `GET /api/v1/me` | Si es un miembro: `user` (`id`, `name`, `email`), `organizations` (`id`, `name`, `role`) y `activeOrganizationId` |
| `POST /api/v1/me/active-organization` | Cambia la organización activa. Solo acepta una organización del miembro; si no, 404. La elección queda en la cookie `postik_org` |
| `GET /api/v1/auth/oidc/login` | Paso 1 de §3 |
| `GET /api/v1/auth/oidc/callback` | Paso 2 de §3 |

## 7. Pruebas

- **Proveedor OIDC falso:** `internal/testsupport/fakeoidc` lo implementa, con descubrimiento, JWKS, `/authorize` y `/token` con PKCE, y firma RS256.
  - Los tests de Go le dicen quién entra antes de cada acceso.
  - En pantalla, su `/authorize` muestra un formulario para escribir `sub`, email y nombre.
- **`cmd/postik-fakes`:** lo sirve en el puerto 5556 para Playwright y para probar a mano en el devcontainer (`make dev`).
- **Proveedor real:** Zetesis-Auth u otro se usa cambiando las variables de §2 (P4 del índice).

## 8. Interfaz

Portada de Postiz (constitución, §2):

- **Botón de acceso:** el de `components/auth/providers/oauth.provider.tsx`, blanco, con «Iniciar sesión con <proveedor>».
- **Selector de organización:** el de `components/layout/organization.selector.tsx`. Solo aparece con más de una organización y la lista se abre al pasar el ratón. Al elegir una, la página se recarga.
- **Modo claro u oscuro:** el interruptor de `components/layout/mode.component.tsx`. La elección se recuerda en el navegador.
- **Marco de `/launches`:** el de S01, con «Calendario» en el menú lateral y el título de la sección.

## 9. Casos

## S02.1 La pantalla de acceso ofrece el proveedor configurado

Cubre: F1; §8.6.

- **Dado** una instancia con `POSTIK_OIDC_DISPLAY_NAME=Zetesis`.
- **Cuando** se pide `GET /api/v1/instance`.
- **Entonces** trae `oidc.name = "Zetesis"`. Sin proveedor configurado, `oidc` no viene.

## S02.2 La primera entrada crea a la persona y su organización

Cubre: F1 pasos 1–3 y 5; §6.2.

- **Dado** una instancia con proveedor y sin nadie dado de alta.
- **Cuando** Ana (`sub` `ana-1`, email `ana@example.com`, nombre «Ana Pérez») completa el acceso.
- **Entonces** acaba en `/launches` con sesión de miembro, y `GET /api/v1/me` devuelve a Ana con una sola organización, «Ana Pérez», con rol `OWNER` y activa.

## S02.3 Volver a entrar no duplica nada ni cambia los datos

Cubre: F1 paso 2; §6.1.

- **Dado** Ana ya dada de alta.
- **Cuando** vuelve a entrar y ahora el proveedor dice que se llama «Ana P.» y su email es `ana@otra.example`.
- **Entonces** sigue siendo una sola persona con una sola organización, y conserva el nombre y el email del primer acceso.

## S02.4 Sin email, la primera entrada se deniega

Cubre: F1, errores.

- **Dado** un proveedor que no entrega email.
- **Cuando** una persona nueva completa el acceso.
- **Entonces** vuelve a `/auth/login?error=email_required`, sin sesión, y no se crea ni la persona ni la organización.

## S02.5 Con «exigir invitación», quien no tiene cuenta no entra

Cubre: F1, errores; §6.1.

- **Dado** `POSTIK_REQUIRE_INVITATION=true` y Ana ya dada de alta desde antes.
- **Cuando** entran una persona nueva y Ana.
- **Entonces** la nueva vuelve a `/auth/login?error=invitation_required` sin que se cree nada, y Ana entra.

## S02.6 Si el proveedor rechaza la autorización, se vuelve al acceso

Cubre: F1, errores.

- **Dado** un proveedor que responde `error=access_denied`.
- **Cuando** vuelve al callback.
- **Entonces** acaba en `/auth/login?error=oidc_denied`, sin sesión.

## S02.7 Un retorno que no corresponde al navegador o ha caducado se rechaza

Cubre: §3 de esta spec.

- **Dado** un acceso iniciado.
- **Cuando** el callback llega sin la cookie de `state`, con un `state` distinto, pasados 10 minutos, o por segunda vez con el mismo `state`.
- **Entonces** los cuatro acaban en `/auth/login?error=oidc_state` y ninguno abre sesión.

## S02.8 La sesión de un miembro caduca tras 7 días sin uso y como mucho a los 30

Cubre: §6.1.

- **Dado** Ana con sesión abierta el día 1.
- **Cuando** usa la aplicación el día 7 y el día 13, deja de usarla, y vuelve el día 21; y, en otra sesión, la usa todos los días desde el día 1.
- **Entonces** la primera sesión sigue viva el día 13 y ha caducado el día 21; la segunda caduca el día 31, aunque se use a diario.

## S02.9 La organización activa es la recordada si sigue siendo suya y, si no, la primera

Cubre: F4, alternativa; constitución §8.

- **Dado** Ana en «Ana Pérez», y después también en «Agencia Norte».
- **Cuando** pide `GET /api/v1/me` sin cookie de organización, con la de «Agencia Norte», y con la de una organización ajena.
- **Entonces** la activa es «Ana Pérez», luego «Agencia Norte», y con la ajena vuelve a ser «Ana Pérez». La ajena nunca aparece en la lista.

## S02.10 Solo se puede cambiar a una organización propia

Cubre: F4; constitución §8.

- **Dado** Ana en «Ana Pérez» y «Agencia Norte», y una organización ajena.
- **Cuando** pide cambiar a «Agencia Norte» y después a la ajena.
- **Entonces** el primer cambio responde 204 y deja «Agencia Norte» como activa. El segundo responde 404 y la activa no cambia.

## S02.11 Una persona entra con el proveedor, llega al calendario y sale

Cubre: F1; §8.1; §8.6.

- **Dado** la pantalla de acceso en español.
- **Cuando** se pulsa «Iniciar sesión con Fake», se escribe en el proveedor quién es, se vuelve y se cierra sesión.
- **Entonces** ve «Calendario» y ningún selector de organización. Al cerrar sesión vuelve a «Iniciar sesión».

## S02.12 Con dos organizaciones aparece el selector y cambia la activa

Cubre: F4; §8.1.

- **Dado** una persona que entra y que además pertenece a una segunda organización.
- **Cuando** recarga, pasa el ratón por el selector y elige la segunda.
- **Entonces** la página se recarga con la segunda organización en la cabecera.
