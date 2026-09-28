# postik — Especificación funcional: alcance reducido v1

- **Referencia:** comportamiento de Postiz v2.24.0 (tag `v2.24.0` del fork de referencia [`Zetesis-Labs/postiz-app`](https://github.com/Zetesis-Labs/postiz-app)). postik no comparte código ni historia con Postiz.
- **Fecha:** 2026-09-27 (actualizada el 2026-09-28).
- **Convenciones:**
  - Lo que no lleva etiqueta es comportamiento de Postiz v2.24.0 que se conserva tal cual.
  - **[Cambio v1]** marca una desviación acordada respecto a Postiz.
  - «No evidenciado» indica que el código de Postiz no permite afirmarlo.

## 1. Resumen

postik replica Postiz, el programador de publicaciones en redes sociales, en un alcance reducido. Un equipo organizado en organizaciones conecta sus canales (páginas y cuentas de redes sociales), escribe publicaciones para varios canales a la vez, las programa en un calendario y el sistema las publica a su hora: con hilos o comentarios encadenados, reintentos seguros y avisos cuando algo falla. Un agente externo puede hacer lo mismo a través de MCP.

La v1 cubre:

- organizaciones, equipos y roles;
- inicio de sesión solo por OIDC contra cualquier proveedor, más un superadmin local de emergencia;
- ocho proveedores de canal: Facebook, Instagram (vía Facebook), Threads, LinkedIn (perfil), LinkedIn (página), X, YouTube y Telegram;
- editor, calendario, programación y publicación;
- biblioteca de medios;
- notificaciones;
- servidor MCP.

Queda fuera todo lo demás que ofrece Postiz; el detalle está en la sección 9. El sistema tiene que publicar a su hora aunque nadie tenga la aplicación abierta.

## 2. Actores y roles

### Personas

| Actor | Quién es | Cómo entra |
|---|---|---|
| **Superadmin de plataforma** | Operador de la instancia. Cuenta única y local. | Usuario y contraseña definidos en los secretos del despliegue, con 2FA TOTP opcional. **[Cambio v1]** |
| **Miembro** | Cualquier persona con identidad en el proveedor OIDC configurado. | Inicio de sesión OIDC. **[Cambio v1]** |

### Roles dentro de una organización

Un miembro tiene un rol en cada organización a la que pertenece:

| Rol | Nombre en Postiz | Qué añade sobre el anterior |
|---|---|---|
| **Usuario** | `USER` | Conectar, configurar y borrar canales; crear, editar y borrar posts; gestionar medios, etiquetas y clientes. |
| **Admin** | `ADMIN` | Invitar miembros; quitar miembros de rol Usuario. |
| **Propietario** | `SUPERADMIN` | Quitar también a los Admin. Es quien crea la organización; no se puede invitar a nadie con este rol. |

En Postiz, el rol solo restringe la gestión del equipo y la clave de API de la organización (que en la v1 desaparece). Canales, posts y medios están abiertos a cualquier miembro: sus endpoints solo comprueban límites de plan, y la v1 no tiene planes.

El «Propietario» se llama `SUPERADMIN` en Postiz, pero no tiene nada que ver con el superadmin de plataforma. La spec usa «Propietario» para no confundirlos.

### Poderes del superadmin de plataforma

- **Añadir sin invitación:** meter a un usuario que ya existe en cualquier organización, con rol Usuario o Admin.
- **Crear organizaciones:** hace falta cuando la instancia exige invitación (sección 6.1), porque entonces nadie puede crear la suya.

### Sistemas externos

- **Proveedor OIDC:** uno por instancia, cualquier proveedor conforme a OIDC. **[Cambio v1]**
- **Redes sociales:** las de los ocho proveedores de canal.
- **Proveedor de correo:** Resend o SMTP. Sin proveedor configurado no se envían correos.
- **Clientes MCP:** Claude, ChatGPT, Cursor y similares.

## 3. Objetivos y trabajos de cada actor

1. **Publicar sin estar presente.** El miembro deja programado el contenido de la semana para varias redes y cada pieza sale a su hora, con su hilo o sus comentarios. Si algo falla, se entera.
2. **Adaptar un mismo mensaje a cada red.** Escribe una vez y personaliza por canal: texto, medios y ajustes propios de cada red (tipo de post, título de YouTube, quién puede responder en X…).
3. **Trabajar en equipo y con clientes.** Una agencia agrupa los canales por cliente, filtra el calendario por cliente e invita a su equipo con distintos permisos.
4. **Mantener sanos los canales.** Ver qué canal hay que reconectar y hacerlo en un clic.
5. **Delegar en un agente.** Desde un cliente MCP, listar canales, consultar las reglas de cada red, subir medios y programar posts.
6. **Operar la instancia (superadmin).** Entrar aunque el proveedor OIDC esté caído y dar de alta equipos.

## 4. Puntos de entrada

### Pantallas

- Inicio de sesión.
- Calendario (`/launches`), con la barra lateral de canales.
- Biblioteca de medios (`/media`).
- Ajustes (`/settings`).
- Retorno de la conexión de un canal (`/integrations/social/<proveedor>`).
- Panel del superadmin. **[Cambio v1]**

### Integraciones

- **Retorno del inicio de sesión OIDC.**
- **Retorno de la autorización de cada red social**, válido una hora desde que se inicia.
- **Telegram:** el sistema escucha los comandos `/connect <código>` que recibe su bot.
- **Servidor MCP**, con descubrimiento de autorización que apunta al proveedor OIDC. **[Cambio v1]**

### Procesos automáticos

| Proceso | Cuándo |
|---|---|
| Publicación de cada post programado | A su hora |
| Comentarios o hilo de un post | Tras el post principal, con su retardo |
| Barrido de posts que se quedaron sin publicar | Cada hora |
| Renovación anticipada del token | Antes de que caduque, solo en Threads |
| Aviso de caducidad del token de Facebook e Instagram **[Cambio v1]** | Una comprobación al día; avisa 7 días antes |
| Correo de resumen de publicaciones con éxito | Como mucho una vez por hora y organización |

### Salidas

- Correos: fallo de publicación, canal desconectado, token a punto de caducar **[Cambio v1]**, invitación y resumen de éxitos.
- Notificaciones dentro de la aplicación.
- Publicaciones en las redes.

## 5. Flujos

### F1. Primer acceso de un miembro por OIDC **[Cambio v1]**

- **Inicio:** la persona pulsa «Entrar con <proveedor>» en la pantalla de acceso.
- **Pasos:**
  1. La aplicación la redirige al proveedor OIDC pidiendo `openid profile email`.
  2. Al volver, identifica a la persona por emisor + `sub` y toma el email y el nombre de las claims.
  3. Si no existía, la crea.
  4. Si llegó con una invitación vigente (F3), entra directamente en esa organización.
  5. Si no, se le crea su propia organización, de la que es Propietario, como en Postiz.
- **Fin:** sesión abierta en su organización activa (F4).
- **Errores:**
  - El proveedor rechaza o cancela la autorización: se vuelve a la pantalla de acceso con el error.
  - Si la instancia exige invitación y la persona llega sin ella, se le deniega el acceso con un mensaje.
  - El proveedor no entrega email: se deniega el acceso con un mensaje, porque el email hace falta para invitaciones y avisos. **[Cambio v1]**

### F2. Acceso del superadmin **[Cambio v1]**

- **Inicio:** en la pantalla de acceso, el enlace discreto «Acceso de administrador».
- **Pasos:**
  1. Introduce usuario y contraseña.
  2. Si hay 2FA configurado, introduce el código TOTP o un código de recuperación.
- **Fin:** sesión de superadmin en el panel del superadmin.
- **Errores:** credenciales o código incorrectos dan un error genérico, sin decir qué parte falló.
- **Contraseña:**
  - no se cambia desde la aplicación: se cambia rotando el secreto del despliegue;
  - no hay recuperación de contraseña;
  - no existe ninguna otra cuenta local.

### F3. Invitar a un miembro y aceptar la invitación

- **Inicio:** un Admin o el Propietario, en Ajustes > Equipo, pulsa «Añadir miembro».
- **Pasos:**
  1. Indica el email y el rol (solo Usuario o Admin).
  2. Marca «enviar por correo» o no. Si no lo marca, el enlace se copia al portapapeles.
  3. El invitado abre el enlace.
  4. Si tiene sesión, entra en la organización con el rol de la invitación.
  5. Si no tiene sesión, la invitación se guarda 15 minutos mientras inicia sesión (F1) y se aplica al volver.
- **Fin:** el invitado aparece en la lista del equipo.
- **Reglas:**
  - El enlace caduca a los 2 días y solo se puede usar una vez.
  - En Postiz la invitación no está atada al email indicado: la acepta quien abra el enlace. No evidenciado que se compruebe el email.
- **Errores:** invitación caducada o ya usada: se rechaza con un mensaje.

### F4. Cambiar de organización activa

- **Inicio:** el selector de organización de la cabecera, visible si el miembro pertenece a más de una.
- **Pasos:** elige una organización y la aplicación recarga con ella como activa. La elección se recuerda en el navegador.
- **Fin:** calendario, canales y medios pasan a ser los de la organización elegida.
- **Alternativa:** si la organización recordada ya no es suya, se usa la primera de su lista.

### F5. Conectar un canal por OAuth

- **Inicio:** «Añadir canal» en la barra lateral, o el «+» de un hueco del calendario cuando todavía no hay canales.
- **Pasos:**
  1. Una rejilla muestra solo los proveedores con credenciales configuradas en el despliegue. **[Cambio v1]**
  2. Al elegir uno, la aplicación redirige a la red a pedir permisos.
  3. Al volver, comprueba que se concedieron todos los permisos pedidos.
  4. Crea el canal o, si esa cuenta ya estaba conectada en la organización, lo actualiza.
  5. **Facebook, Instagram, LinkedIn (página) y YouTube** tienen un paso intermedio: una rejilla con las páginas, cuentas o canales disponibles para elegir uno solo y pulsar «Guardar». Hasta ese momento el canal queda «en paso intermedio» y no se puede usar.
  6. Al canal se le asignan tres franjas de publicación por defecto: 9:20, 14:10 y 19:00 en la hora local de quien conecta.
- **Fin:** el canal aparece en la barra lateral, activo.
- **Alternativas:** para conectar varias páginas de la misma red se repite el flujo entero, una vez por página.
- **Errores:**
  - Faltan permisos: error de permisos insuficientes y el canal no se crea.
  - Autorización caducada (más de una hora): error y hay que reiniciar el flujo.

### F6. Conectar Telegram

- **Inicio:** elegir Telegram en «Añadir canal».
- **Pasos:**
  1. La aplicación genera un código de 4 caracteres.
  2. Pide añadir el bot `@<nombre del bot>` al grupo o canal y enviar `/connect <código>`.
  3. Mientras tanto consulta cada 2 segundos hasta detectar ese mensaje.
  4. Si el bot es administrador del chat, borra el mensaje del comando.
  5. Obtiene el nombre, la foto y el usuario del chat.
- **Fin:** el canal de Telegram queda conectado.
- **Errores:** si el mensaje no llega, la espera continúa hasta que el usuario cierre la ventana. No evidenciado que haya un tiempo máximo.

### F7. Reconectar un canal

- **Inicio:** el canal muestra un «!» rojo sobre su avatar con el aviso «Canal desconectado, pulsa para reconectar», o el usuario elige «Reconectar canal» en su menú.
- **Pasos:** repite la autorización de F5.
  - En Facebook, Instagram, LinkedIn (página) y YouTube se comprueba que la página o cuenta autorizada es la misma que la del canal.
- **Fin:** el canal vuelve a estar activo, con sus posts y ajustes intactos.
- **Errores:** si la cuenta autorizada no es la del canal, se muestra un error pidiendo reconectar la cuenta correcta.

### F8. Gestionar un canal (menú contextual)

| Acción | Efecto |
|---|---|
| **Crear post** | Abre el editor con el canal preseleccionado. No aparece si el canal necesita reconexión. |
| **Copiar ID del canal** | Copia el identificador al portapapeles. |
| **Reconectar** | F7. Solo aparece si el canal necesita reconexión, resaltada. |
| **Ajustes adicionales** | Solo en X: la casilla «Verificada» (cuenta premium), que sube el límite de caracteres. |
| **Mover a cliente** | Asigna el canal a un cliente por nombre (lo crea si no existe). También se puede arrastrar el canal sobre un grupo de la barra lateral. |
| **Editar franjas** | Edita las horas de publicación preferidas del canal (sirven para «siguiente hueco libre», F9). |
| **Activar / Desactivar** | Un canal desactivado se ve semitransparente, no aparece al crear posts y sus posts ya programados fallarán (F11). |
| **Borrar** | Borra primero todos los posts del canal y después el canal. |

### F9. Crear un post

- **Inicio:** cualquiera de estas tres entradas:
  - el botón «Crear post»: la fecha se rellena con el siguiente hueco libre;
  - pulsar un hueco futuro del calendario: se usa la fecha de ese hueco, o ahora + 10 minutos si el hueco es la hora actual;
  - «Crear post» en el menú de un canal.
- **Pasos:**
  1. Elige uno o varios canales. No aparecen los desactivados ni los que están en paso intermedio.
  2. Escribe el contenido global: texto con formato y medios de la biblioteca (F15).
  3. Si quiere, añade valores adicionales: los siguientes elementos del hilo o comentarios. Cada uno puede llevar un retardo en minutos.
  4. Si quiere, personaliza un canal: se copia el contenido global como punto de partida y a partir de ahí ese canal tiene su propio texto y medios.
  5. Rellena los ajustes propios de cada red (sección 6.4).
  6. Revisa la vista previa por red: el texto que excede el límite de caracteres de la red aparece resaltado en rojo.
  7. Añade etiquetas si quiere.
  8. Fija fecha y hora (en la hora local del navegador).
  9. Pulsa una de las cuatro acciones:
     - **«Añadir al calendario»:** programa el post.
     - **«Guardar como borrador».**
     - **«Publicar ya»:** aparece al pasar el ratón; fija la fecha a ahora.
     - **Siguiente hueco libre:** calcula la fecha con el primer minuto futuro libre entre las franjas del canal, día a día.
- **Fin:** el post aparece en el calendario en estado Programado, o Borrador.
- **Errores:** la validación se hace al guardar y la repite el servidor (sección 6.5). Los mensajes posibles son:
  - «El post debe tener al menos un carácter o una imagen».
  - «Revisa los ajustes», o el error concreto de la red.
  - «El post es demasiado largo».

### F10. Editar, duplicar, borrar y reprogramar

- **Editar:** abrir un post del calendario. Solo se ve su canal y no se pueden añadir más. Hay dos formas de guardar:
  - **«Actualizar»:** cambia los detalles sin tocar el estado ni volver a programar.
  - **«Programar»:** vuelve a programar. Anula la publicación pendiente anterior y lanza una nueva.
  - Guardar un post ya publicado exige confirmar que se quiere republicar.
- **Duplicar:** copia solo el texto y los medios (ni ajustes por canal ni etiquetas), crea un grupo nuevo y le asigna el siguiente hueco libre.
- **Borrar:** tras confirmar, borra el grupo entero (el post en todos sus canales) y anula su publicación pendiente.
- **Arrastrar en el calendario:**
  - No se puede soltar un post en un hueco pasado.
  - Si el post ya está publicado, o está programado con la fecha vencida, pregunta «¿Actualizar solo detalles o reprogramar?». Reprogramar implica republicar.
  - En el resto de casos, arrastrar reprograma directamente.

### F11. Publicación programada

- **Inicio:** llega la hora de un post programado. Si la hora ya pasó al programarlo, se publica de inmediato.
- **Pasos:**
  1. Si el post ya no está Programado (se publicó o se pasó a borrador), no hace nada.
  2. Si el canal está desactivado o necesita reconexión, marca el post como Error («Canal desactivado» o «Hay que reconectar el canal»), avisa y no intenta publicar.
  3. Publica el post principal. Después publica cada comentario o elemento del hilo en orden, esperando el retardo de cada uno.
  4. Si la red devuelve un error de autorización, intenta renovar el token y reintenta.
  5. Si la publicación se confirma, marca el post como Publicado y guarda el enlace a la publicación real.
- **Fin:** Publicado, con enlace; o Error, con el mensaje visible en el calendario.
- **Reintentos, pensados para no publicar nunca dos veces:**
  - Solo se reintenta, hasta 5 veces, cuando se renovó el token o cuando la acción ni siquiera llegó a empezar.
  - Si la red rechaza el contenido, no se reintenta: se marca Error.
  - Si no se sabe si la red recibió la publicación (se agotó el tiempo a mitad), tampoco se reintenta: queda como no confirmada.
- **Otros casos:**
  - Si la renovación del token falla, el canal pasa a necesitar reconexión y se avisa (F12).
  - Si la red publicó pero no devolvió identificador, el post queda Publicado con el enlace pendiente (F14).
- **Avisos:** notificación en la aplicación siempre. El fallo se manda además por correo en el momento; el éxito entra en el resumen horario (sección 6.8).

### F12. Renovación del token de un canal

- **Anticipada:** solo en Threads, justo antes de que caduque el token. No se hace si el canal está borrado, en paso intermedio o pendiente de reconexión.
- **Bajo demanda:** en las demás redes, cuando una llamada a la red falla por autorización (publicar o usar una herramienta desde MCP).
- **Si la renovación falla:** el canal pasa a «necesita reconexión», se crea una notificación y se envía un correo aunque el usuario tenga los correos desactivados. El texto es «No se ha podido renovar tu canal <red>… vuelve a conectarlo» con enlace al calendario.
- **Vigencia por red:** está en la sección 6.4. En Facebook e Instagram el token no se puede renovar: dura unos 59 días y después obliga a reconectar.
- **Aviso de caducidad** **[Cambio v1]:** en Facebook e Instagram, una comprobación diaria avisa 7 días antes de que caduque el token, con notificación y correo, para reconectar a tiempo. Se avisa una sola vez por canal y caducidad; al reconectar, la cuenta vuelve a empezar.

### F13. Recuperar posts que se quedaron sin publicar

- **Inicio:** barrido automático cada hora.
- **Pasos:** busca posts que cumplan todo esto y los relanza:
  - están Programados;
  - su canal está sano;
  - su fecha está entre hace 2 días y ahora;
  - no tienen publicación en curso.
- **Fin:** esos posts se publican (F11).
- **Límite:** los que llevan más de 2 días vencidos no se recuperan y siguen como Programados.

### F14. Vincular una publicación sin enlace

- **Inicio:** en el calendario, el post publicado muestra el icono «Vincular publicación» porque la red no devolvió identificador.
- **Pasos:** el usuario indica cuál es la publicación real.
- **Fin:** el post queda con su enlace.
- **Detalle:** no evidenciado cómo se elige exactamente la publicación en la interfaz.
- **Nota:** en Postiz v2.24.0 solo TikTok deja posts sin enlace. Ninguna de las ocho redes de la v1 lo hace hoy; se conserva como red de seguridad.

### F15. Subir y gestionar medios

- **Inicio:** la biblioteca de medios (`/media`), o «Insertar medio» dentro del editor.
- **Pasos:**
  1. Sube uno o varios archivos. El navegador bloquea las tandas de más de 1 GB.
  2. El servidor detecta el tipo real por el contenido del archivo, no por la extensión.
  3. El archivo queda disponible en una URL pública estable.
- **Otras acciones:**
  - Buscar por nombre original.
  - Paginar, 18 por página.
  - Borrar, con confirmación.
  - En el editor: editar el texto alternativo y, en los vídeos, elegir un fotograma como miniatura.
- **Errores:** «Tipo de archivo no admitido», «El archivo es demasiado grande» o «No se ha podido leer el archivo».

### F16. Operar desde un cliente MCP **[Cambio v1]**

Se replica el diseño de la rama `feat/mcp-keycloak-oauth` de ZetesisPortal.

- **Inicio:** en Ajustes > MCP, el miembro elige su cliente (Claude, Claude Code, ChatGPT, Cursor, VS Code, Gemini CLI…). La pantalla le ofrece dos vías:
  - **automática**, en un clic, para los clientes que saben autenticarse solos;
  - **manual**, con un token personal.
- **Pasos por la vía automática:**
  1. El cliente llama a la URL del MCP sin credenciales. La respuesta «no autorizado» le indica dónde están los metadatos del recurso protegido.
  2. Los metadatos apuntan al proveedor OIDC. El cliente se da de alta en él por registro dinámico, abre el inicio de sesión y obtiene un token con el permiso `mcp:tools` y dirigido a la URL del MCP.
  3. El servidor comprueba emisor, audiencia y permiso, y localiza al miembro por su `sub`.
- **Pasos por la vía manual:** el miembro crea un token personal en Ajustes > MCP y lo pega en la configuración de su cliente, en cabecera o en la URL.
- **Elección de organización:**
  - Si la URL lleva la organización (`/mcp/<organización>`), se actúa sobre ella; el miembro tiene que pertenecer a ella.
  - Si no la lleva y el miembro pertenece a una sola, se usa esa.
  - Si pertenece a varias, se devuelve un error con la lista de organizaciones posibles.
- **Fin:** el agente usa las herramientas de la sección 6.9. Los posts creados así aparecen en el calendario marcados con origen MCP.
- **Errores:**

  | Situación | Respuesta |
  |---|---|
  | Sin credenciales | «No autorizado», con el enlace a los metadatos |
  | Token caducado, de otro emisor o dirigido a otro servicio | «Token inválido» |
  | Token sin el permiso necesario | «Permiso insuficiente», indicando cuál falta |
  | La cuenta del proveedor aún no ha entrado nunca en la aplicación web | «Entra una vez en postik y vuelve a intentarlo» |
  | La organización de la URL no existe o el miembro no pertenece a ella | «Acceso denegado» |

### F17. Borrar la propia cuenta

- **Inicio:** Ajustes > General > «Borrar cuenta», con confirmación.
- **Casos:**
  - Si es Propietario de una organización con más miembros, se bloquea: antes tiene que quitarlos.
  - Si es Propietario único, su organización se borra.
  - En las demás organizaciones, solo deja de ser miembro.
- **Fin:** sus datos personales se anonimizan y se cierra la sesión.
- **Después:** si vuelve a entrar por OIDC, se le trata como persona nueva.
- **Salir de una organización:** no hay acción independiente; en Postiz solo ocurre al borrar la cuenta.

### F18. Notificaciones en la aplicación

- **Inicio:** llega una notificación a la organización.
- **Pasos:**
  1. La campana muestra cuántas hay sin leer para ese miembro.
  2. Al abrir el panel el contador se pone a cero, se muestran las 10 últimas con las no leídas resaltadas y los enlaces del texto son clicables.
- **Fin:** abrir el panel marca todas como leídas.

## 6. Reglas funcionales y restricciones

### 6.1 Identidad y sesión

- **Métodos de acceso** **[Cambio v1]:**
  - Solo OIDC contra un único proveedor por instancia, más el superadmin local.
  - Desaparecen el registro con email y contraseña, la activación, la recuperación de contraseña y los accesos específicos de GitHub, Google, Apple, Farcaster y wallet. Google u otros se pueden usar si son el proveedor OIDC configurado.
- **Control de acceso:** lo decide el proveedor OIDC. La app deja entrar a quien el proveedor autentica; por ejemplo, en Keycloak se restringe con los roles del cliente.
  - La opción de instancia «exigir invitación», desactivada por defecto, sirve para proveedores abiertos como Google.
  - Quien entra por invitación no recibe una organización propia extra.
- **Identificación del miembro:** emisor + `sub`. El email y el nombre vienen de las claims del primer acceso.
  - El email no se edita en la aplicación.
  - El nombre, la bio y el avatar sí se pueden editar.
- **Superadmin** **[Cambio v1]:**
  - Usuario, contraseña y, si se usa 2FA, la semilla TOTP vienen de los secretos del despliegue.
  - La aplicación no guarda ni cambia su contraseña.
  - Los códigos de recuperación también vienen del secreto.
- **Sesión** **[Cambio v1]:** Postiz la mantiene un año, en cookie. En la v1:
  - la de un miembro caduca tras 7 días sin uso y, como mucho, a los 30 días. Al caducar se le manda al proveedor OIDC, que le deja entrar sin pedir nada si su sesión allí sigue viva;
  - la del superadmin dura 12 horas y no se renueva;
  - quitar a alguien de una organización le corta el acceso a ella en el acto;
  - darle de baja en el proveedor OIDC no cierra la sesión abierta: la pierde cuando caduca.
- **Idioma y zona horaria:** son preferencias del navegador, no del perfil. La zona horaria se detecta automáticamente y todas las fechas se guardan en UTC.
- **Idiomas** **[Cambio v1]:** solo español e inglés; Postiz trae muchos más.

### 6.2 Organizaciones y equipo

- **Membresía:** un miembro puede pertenecer a varias organizaciones y tiene un rol en cada una.
- **Invitaciones:**
  - Solo las crean Admin y Propietario.
  - Solo con rol Usuario o Admin.
  - Caducan en 2 días y son de un solo uso.
- **Quitar miembros:** solo se puede quitar a alguien de rol estrictamente inferior (Usuario < Admin < Propietario). El botón «Quitar» solo aparece cuando está permitido.
- **Clientes:**
  - Agrupan canales dentro de una organización.
  - El nombre es único en la organización.
  - Un canal pertenece a un cliente o a ninguno.
- **Sin planes** **[Cambio v1]:** no hay límites de canales, de posts al mes ni de miembros. Es lo que hace Postiz cuando no tiene la facturación configurada.

### 6.3 Canales

- **Estados:**

  | Estado | Cuándo | Efecto |
  |---|---|---|
  | Activo | Por defecto | Se puede usar. |
  | En paso intermedio | Falta elegir página o cuenta (F5) | No se puede usar. |
  | Desactivado | Lo desactiva un miembro | No se ofrece al crear posts; sus posts fallan. |
  | Necesita reconexión | Falló la renovación o la red lo desconectó | Aviso rojo; sus posts fallan. |
  | Borrado | Lo borra un miembro | Se borran también sus posts. |

- **Unicidad:** un canal es único por organización y cuenta de la red. Conectar la misma cuenta otra vez actualiza el canal existente.
- **Franjas de publicación:**
  - Son una lista de horas del día.
  - Por defecto hay tres, fijadas al conectar.
  - Solo se usan para calcular el siguiente hueco libre; no restringen cuándo se puede programar.
- **Configuración en X:**
  - La opción «quitar enlaces de los posts de X» es de la instancia, no del usuario.
  - La casilla «Verificada» del canal sube el límite de 280 a 4.000 caracteres.

### 6.4 Reglas por red

| Red | Conexión | Qué se publica | Límite de caracteres | Hilo o comentarios | Token |
|---|---|---|---|---|---|
| **Facebook** (páginas) | OAuth + elegir página | Texto o enlace, fotos, un vídeo mp4 (sale como Reel), story | 63.206 | Sí | Unos 59 días, no renovable: hay que reconectar |
| **Instagram** (vía Facebook) | OAuth + elegir cuenta business | Post (1 medio o carrusel de hasta 10), story, reel, reel de prueba | 2.200 | Sí | Unos 59 días, no renovable: hay que reconectar |
| **Threads** | OAuth | Texto, una imagen o vídeo, carrusel (2 o más) | 500 | Sí (hilo) | Renovación anticipada |
| **LinkedIn** (perfil) | OAuth | Texto, imágenes, un vídeo, carrusel de documento | 3.000 | Sí, solo texto | Renovable |
| **LinkedIn** (página) | OAuth + elegir página | Igual que el perfil | 3.000 | Sí, solo texto | Renovable |
| **X** | OAuth | Post, artículo | 280 (4.000 si es «Verificada»; 100.000 los artículos) | Sí (hilo) | No caduca |
| **YouTube** | OAuth + elegir canal | Exactamente un vídeo mp4 | 5.000 (descripción) | No | Renovación bajo demanda |
| **Telegram** | Bot + código (F6) | Texto; un medio con pie; grupo de hasta 10 medios | 4.096, con formato básico (negrita, subrayado, párrafos) | Sí (como respuestas) | No caduca |

**Ajustes y validaciones propios de cada red**, que se rellenan por canal en el editor:

- **Facebook:**
  - tipo `post` o `story`;
  - URL opcional;
  - fondo de color para posts de solo texto, de un catálogo de unos 70 fondos y con un máximo de 130 caracteres de texto.
  - Una story necesita al menos un medio, y cada medio sale como una story independiente.
- **Instagram:**
  - tipo `post` o `story`, obligatorio;
  - reel de prueba, con estrategia de graduación `MANUAL` o `SS_PERFORMANCE`;
  - hasta 3 colaboradores;
  - audio para reels: pista y volúmenes de audio y vídeo de 0 a 100.
  - Validaciones: al menos un medio; carrusel de 10 como máximo; el reel de prueba lleva exactamente un vídeo; el audio solo va en un reel de un vídeo, nunca en una story.
  - La búsqueda de audio es una herramienta que se puede lanzar desde MCP.
- **Threads:** sin ajustes propios.
- **LinkedIn:**
  - «publicar las imágenes como carrusel», que necesita al menos 2 imágenes y ningún vídeo; las imágenes se unen en un documento PDF;
  - nombre del carrusel.
  - Solo se admite un vídeo.
- **X:**
  - quién puede responder, obligatorio salvo en los artículos (5 opciones);
  - comunidad, como URL de una comunidad de X;
  - tipo `post` o `article`;
  - en los artículos, título obligatorio, estado `draft` o `published` obligatorio y portada opcional;
  - marcas «hecho con IA» y «colaboración pagada».
  - Los artículos solo admiten imágenes, y un artículo en borrador no puede llevar comentarios.
- **YouTube:**
  - título obligatorio, de 2 a 100 caracteres;
  - visibilidad `public`, `private` o `unlisted`, obligatoria;
  - «hecho para niños»;
  - miniatura;
  - etiquetas, con un total de 500 caracteres como máximo.
- **Telegram:** sin ajustes propios. Los documentos solo se agrupan si todos los medios del grupo son documentos.

### 6.5 Posts

- **Estados y transiciones:**

  ```mermaid
  stateDiagram-v2
    [*] --> Borrador: Guardar como borrador
    [*] --> Programado: Añadir al calendario / Publicar ya
    Borrador --> Programado: Programar
    Programado --> Publicado: publicación confirmada
    Programado --> Error: rechazo, canal desactivado o sin reconectar
    Error --> Programado: Reprogramar
    Publicado --> Programado: Reprogramar (republicar)
    Borrador --> [*]: Borrar
    Programado --> [*]: Borrar
    Publicado --> [*]: Borrar
    Error --> [*]: Borrar
  ```

- **Grupos:** un post del editor crea un grupo con una entrada por canal. Cada entrada tiene su valor principal y sus valores encadenados (hilo o comentarios). Borrar afecta al grupo entero.
- **Validación al guardar** (programado o publicar ya), repetida en el servidor:
  - al menos un carácter o un medio;
  - ajustes de cada red válidos (sección 6.4);
  - longitud dentro del límite de cada red.
- **Borradores:** solo exigen que el contenido no esté vacío y no se programan.
- **Fechas pasadas:** en Postiz solo se bloquean en el calendario (sin «+» en huecos pasados y sin arrastrar hacia el pasado). No evidenciado que el servidor lo impida desde el selector de fecha. **[Cambio v1]** El servidor también lo rechaza, con el mensaje «La fecha ya ha pasado».
- **Etiquetas:** tienen nombre y color, y se asignan al grupo. En el calendario se ven como una franja del color de la etiqueta.
- **Origen:** cada post guarda desde dónde se creó (web o MCP).

### 6.6 Calendario

- **Vistas:** día, semana, mes y lista. La vista elegida se recuerda.
  - La lista pagina de 100 en 100 y filtra por estado: todos, programados, borradores o publicados.
- **Navegación:** anterior, siguiente y «Hoy».
- **Filtro por cliente:** afecta a los canales y posts que se muestran.
- **Huecos pasados:** se ven atenuados, sin «+» y sin aceptar que se suelte un post.
- **Tarjeta de cada post:**
  - avatar del canal con el icono de la red;
  - extracto del texto;
  - hora local;
  - prefijo «Borrador:» si lo es;
  - borde rojo y aviso con el mensaje si está en Error;
  - franja de color si tiene etiqueta;
  - icono «Vincular publicación» si falta el enlace (F14).
- **Acciones al pasar el ratón:** duplicar, vincular publicación (si aplica) y borrar.

### 6.7 Medios

- **Tipos admitidos:** JPEG, PNG, GIF, WebP, AVIF, BMP, TIFF y vídeo MP4.
- **Tamaño máximo:** 10 MB por imagen y 1 GB por vídeo. Se hace cumplir aunque el cliente declare un tamaño falso.
- **URL pública:** los archivos se sirven en una URL estable. Instagram, por ejemplo, descarga el medio desde esa URL, así que tiene que ser accesible desde internet.
- **Borrado:** es lógico: el medio deja de aparecer en la biblioteca. No evidenciado qué pasa con los posts que ya lo usan.
- **Metadatos:** texto alternativo y, en los vídeos, miniatura (URL y segundo del vídeo).

### 6.8 Notificaciones y correo

- **Alcance:** las notificaciones son de la organización. El «no leído» se calcula para cada miembro según cuándo abrió el panel por última vez.
- **Tipos de aviso:**

  | Tipo | Ejemplo | Correo |
  |---|---|---|
  | Informativo | Canal desconectado; token a punto de caducar **[Cambio v1]** | Siempre, sin mirar preferencias |
  | Fallo | Error al publicar | En el momento, si el miembro tiene activados los correos de fallo |
  | Éxito | Post publicado | Agrupado en un resumen de como mucho uno por hora y organización, si el miembro tiene activados los correos de éxito |

- **Preferencias:** Ajustes > General, con dos casillas: correos de éxito y correos de fallo.
- **Pie de los correos:** enlace a los ajustes de notificaciones.
- **Invitaciones:** solo se envían por correo si hay proveedor configurado y quien invita lo marca.

### 6.9 MCP

- **Autenticación** **[Cambio v1]:** acepta dos tipos de credencial.
  - **Tokens del proveedor OIDC:** del emisor configurado, dirigidos a la URL del MCP y con el permiso `mcp:tools`. Solo en cabecera; nunca en la URL, porque acabarían en logs e historiales.
  - **Tokens personales:** cada miembro crea, nombra y revoca los suyos. Se aceptan en cabecera o en la URL, para clientes que no admiten cabeceras.
  - Desaparecen el servidor OAuth propio de Postiz, su pantalla de consentimiento, las «apps aprobadas» y la clave de API de la organización.
- **Organización:** cada conexión actúa sobre una sola organización, elegida como se describe en F16. El superadmin puede usar cualquiera.
- **Herramientas:** se conservan con los mismos nombres que en Postiz, para que los agentes y prompts existentes sigan valiendo.

  | Herramienta | Qué hace |
  |---|---|
  | `integrationList` | Lista los canales, filtrables por cliente. |
  | `groupList` | Lista los clientes. |
  | `integrationSchema` | Para una red: reglas, límite de caracteres, ajustes admitidos y herramientas propias. |
  | `triggerTool` | Ejecuta una herramienta propia de una red (en la v1, solo la búsqueda de audio de Instagram). Si el token caducó intenta renovarlo; si no puede, el canal pasa a necesitar reconexión y devuelve error. |
  | `schedulePostTool` | Crea posts como borrador, programados o para publicar ya, con valores encadenados, ajustes y adjuntos por URL. Aplica las mismas validaciones que el editor y devuelve los identificadores creados, o una lista de errores que el agente puede corregir y reenviar. |
  | `postsListTool` | Lista los posts de un intervalo de fechas en UTC, en cualquier estado, filtrables por cliente. No borra. |
  | `postSettingsTool` | Cambia solo los ajustes de red de un post no publicado. No toca el texto ni la fecha. |
  | `uploadFromUrlTool` | Trae a la biblioteca un archivo desde una URL pública. Rechaza direcciones internas y, si falla, explica la causa real. |

- **Pantalla:** Ajustes > MCP tiene un selector de cliente que da las dos vías de conexión (automática y manual) con su configuración lista para copiar, y la gestión de tokens personales como pestaña propia del selector.
- **Requisitos del proveedor para la vía automática:** registro dinámico de clientes con los hosts de retorno de los clientes admitidos, el permiso `mcp:tools` y una audiencia igual a la URL del MCP. En Keycloak: registro dinámico anónimo con «Trusted Hosts», PKCE obligatorio para clientes públicos y un audience mapper, como en ZetesisPortal. Con proveedores que no lo admiten (Google, por ejemplo) solo funciona la vía manual.

## 7. Conceptos de datos

| Concepto | Qué es | Campos que ve el usuario |
|---|---|---|
| **Miembro** | Persona con acceso, identificada por su proveedor OIDC | Nombre, email, bio, avatar, preferencias de correo |
| **Superadmin** | Cuenta local de operación, fuera de las organizaciones | Usuario, 2FA activo o no |
| **Organización** | Espacio de trabajo | Nombre, miembros |
| **Token personal** | Credencial de un miembro para clientes MCP | Nombre, fecha de creación, último uso; revocable |
| **Membresía** | Relación miembro–organización | Rol: Usuario, Admin o Propietario |
| **Invitación** | Enlace para unirse | Email indicado, rol, caducidad (2 días), usada o no |
| **Cliente** | Agrupación de canales de una organización | Nombre |
| **Canal** | Cuenta, página o chat conectado de una red | Red, nombre, avatar, estado (sección 6.3), cliente, franjas de publicación, ajustes adicionales |
| **Grupo de publicación** | Lo que se crea de una vez en el editor | Canales, etiquetas |
| **Post** | La entrada de un grupo en un canal | Fecha, estado, valores (principal + hilo o comentarios con retardo), ajustes de la red, enlace publicado, mensaje de error, origen |
| **Etiqueta** | Marca de color para posts | Nombre, color |
| **Medio** | Archivo de la biblioteca | Nombre original, URL pública, tipo, texto alternativo, miniatura |
| **Notificación** | Aviso de la organización | Texto, enlace, fecha |

## 8. Representación gráfica

### 8.1 Estructura general

```
┌────────────────────────────────────────────────────────────────────┐
│ [logo]  Calendario  Medios  Ajustes      [Org ▾]  [🔔 3]  [avatar ▾] │
├───────────────┬────────────────────────────────────────────────────┤
│ CANALES       │ [Día|Semana|Mes|Lista]  ◀ Hoy ▶  [Cliente ▾]        │
│               │                                    [Crear post]    │
│ ▾ Cliente A   │ ┌────────┬────────┬────────┬────────┬────────┐     │
│   (f)  Página │ │  lun   │  mar   │  mié   │  jue   │  vie   │     │
│   (in) Perfil │ ├────────┼────────┼────────┼────────┼────────┤     │
│ ▾ Sin cliente │ │░░░░░░░░│   +    │ (X)9:20│   +    │   +    │     │
│   (X)  @cuenta│ │░pasado░│        │«Lanza…»│        │        │     │
│   (tg) Canal !│ └────────┴────────┴────────┴────────┴────────┘     │
│               │                                                    │
│ [+ Añadir     │  ! = necesita reconexión   ░ = hueco pasado        │
│    canal]     │                                                    │
└───────────────┴────────────────────────────────────────────────────┘
```

- **Barra lateral:** canales agrupados por cliente. Menú contextual por canal (F8). Aviso rojo si necesita reconexión. Semitransparente si está desactivado.
- **Cabecera:** el selector de organización solo aparece con más de una. La campana muestra el contador de no leídas (F18). El menú del avatar lleva a Ajustes y a Cerrar sesión.

### 8.2 Editor de post (modal)

```
┌───────────────────────────────── Crear post ─────────────────────────────┐
│ Canales: (f)✓ (in)✓ (X)✓ (tg)                                            │
│ [Global] [f] [in] [X]            ← pestaña por canal = personalizar       │
│ ┌──────────────────────────────┐  ┌─────────── Vista previa ───────────┐ │
│ │ Texto…                        │  │ (X) texto…[exceso en rojo]         │ │
│ │ [Insertar medio] [img][img]   │  └────────────────────────────────────┘ │
│ │ + Añadir comentario/hilo (retardo: _ min)                              │ │
│ └──────────────────────────────┘                                         │
│ Ajustes de X: quién responde [▾]  tipo [post▾]  comunidad [____]         │
│ Etiquetas [▾]   Fecha [27/09 19:00]                                       │
│ [Borrar]        [Guardar como borrador]  [Añadir al calendario ▾ Publicar ya]│
└──────────────────────────────────────────────────────────────────────────┘
```

**Quitado respecto a Postiz** (sección 9): copiloto de IA, generar imagen o vídeo, «Diseñar medio», firmas, sets, el selector «Elegir un set» y «Repetir».

### 8.3 Añadir canal

- **Paso 1:** una rejilla con las redes disponibles (icono y nombre).
- **Paso 2:** según la red, una de estas tres cosas:
  - redirección a la red;
  - la ventana de Telegram, con el código y las instrucciones;
  - la rejilla de páginas o cuentas, de selección única, con el botón «Guardar».

### 8.4 Ajustes (pestañas)

| Pestaña | Visible para | Contenido |
|---|---|---|
| **General** | Todos | Perfil (nombre, bio, avatar), preferencias de correo, borrar cuenta |
| **Equipo** | Todos (acciones según rol) | Lista de miembros con su rol, «Añadir miembro», «Quitar» cuando está permitido |
| **MCP** | Todos | Selector de cliente con las vías automática y manual; tokens personales (crear, revocar) |

### 8.5 Panel del superadmin **[Cambio v1]**

- Buscador de organizaciones y miembros.
- «Añadir usuario existente a organización», con rol.
- «Crear organización», útil cuando la instancia exige invitación.

### 8.6 Acceso

- Botón principal «Entrar con <proveedor>».
- Enlace discreto «Acceso de administrador», que abre usuario, contraseña y, si hay 2FA, el código.
- Selector de idioma.

## 9. Restricciones y decisiones de alcance

**Fuera de la v1:**

- **IA:**
  - el copiloto de redacción y la generación de posts;
  - el agente o chat (`/agents`);
  - la generación de imagen y vídeo;
  - el editor de diseño («Diseñar medio»), que Postiz solo ofrece con IA;
  - HeyGen y ReelFarm;
  - las herramientas MCP de generación y de recortes de vídeo.
- **Negocio:**
  - facturación, planes, límites por plan y enterprise;
  - anuncios, afiliados y UGC.
- **Plataforma para terceros:**
  - la API pública v1 y su CLI;
  - el SDK y la extensión del navegador;
  - el puente para las apps móviles;
  - el servidor OAuth propio, las «OAuth apps», las «apps aprobadas» y la clave de API de la organización.
- **Automatizaciones:**
  - webhooks, autopost (RSS), sets y firmas;
  - *plugs* y *plugs internos* (repost o comentario automático tras X «me gusta», añadir comentario, reposts de otros usuarios);
  - acortador de enlaces;
  - correos de rachas.
- **Canales:**
  - las demás redes de Postiz, TikTok incluido;
  - Instagram independiente (sin Facebook): las cuentas que se usan están vinculadas a páginas de Facebook;
  - migración entre proveedores;
  - cambiar el nombre o avatar del bot (ninguna de las ocho lo admite).
- **Medios:**
  - almacenamiento en la nube y procesado o conversión de medios;
  - importar medios de terceros;
  - el widget de subida de MCP.
- **Otros:**
  - analíticas (segunda ola);
  - comentarios internos de equipo (en Postiz v2.24.0 no están enlazados en la interfaz);
  - posts periódicos (la opción «Repetir» del editor);
  - vista previa pública (el enlace `/p/<id>` sin sesión);
  - impersonar a un miembro desde el panel del superadmin;
  - onboarding tras el primer acceso;
  - páginas de administración `/admin/errors` y `/admin/stats`;
  - «intercambiar credenciales» entre cuentas;
  - el transporte SSE del MCP.

**Consecuencias heredadas de Postiz que se aceptan:**

- **Facebook e Instagram:** obligan a reconectar más o menos cada 59 días, porque su token no se renueva. La v1 avisa 7 días antes (F12).
- **Posts atascados:** los que llevan más de 2 días sin publicar no se recuperan solos.
- **Zona horaria:** cada miembro ve las horas en la de su navegador.
- **Borrado:** borrar un post borra el grupo en todos sus canales.
- **Canales:** cualquier miembro, sea cual sea su rol, puede conectar, desactivar y borrar canales.

## 10. Preguntas resueltas

| Id | Pregunta | Propuesta |
|---|---|---|
| **S1** | Política de entrada. | **Cerrada (2026-09-27):** decide el proveedor OIDC; quien llega sin invitación recibe su propia organización. Opción «exigir invitación», desactivada por defecto. Quien entra por invitación no recibe organización extra. |
| **S2** | ¿Semilla TOTP y códigos de recuperación del superadmin en el secreto, o alta del 2FA desde la aplicación? | **Cerrada (2026-09-28):** en el secreto; el superadmin es pura configuración, sin estado en la base de datos. |
| **S3** | Clientes MCP cuyo proveedor OIDC no admite registro dinámico. | **Cerrada (2026-09-27):** tokens personales por miembro, como en ZetesisPortal; la clave de organización de Postiz desaparece. |
| **S4** | ¿Mostrar redes sin credenciales configuradas? Postiz las muestra y falla al conectar. | **Cerrada (2026-09-28):** solo las que tienen credenciales. |
| **S5** | Instagram independiente (Instagram Login, sin página de Facebook). Necesita `INSTAGRAM_APP_ID`/`INSTAGRAM_APP_SECRET`, que suntzu no tiene; a cambio, su token sí se renueva. La vía Facebook exige cuenta profesional vinculada a una página. | **Cerrada (2026-09-28):** fuera de la v1; todas las cuentas están vinculadas a una página de Facebook. |
| **S6** | ¿Comentarios en la vista previa pública? | **Sin efecto (2026-09-28):** la vista previa pública sale entera de la v1. |
| **S7** | ¿Impedir en el servidor programar en el pasado? | **Cerrada (2026-09-28):** sí, con el mensaje «La fecha ya ha pasado». |
| **S8** | Duración de la sesión. | **Cerrada (2026-09-28):** miembros, 7 días sin uso y 30 como máximo, con reentrada transparente por OIDC; superadmin, 12 horas. Detalle en 6.1. |
| **S9** | Idiomas de la interfaz. | **Cerrada (2026-09-28):** español e inglés. |
| **S10** | Organización sobre la que actúa MCP. | **Cerrada (2026-09-27):** como en ZetesisPortal: organización opcional en la URL; sin ella, la única del miembro, o error con la lista si tiene varias. |
| **S11** | Proveedor OIDC que no entrega email. | **Cerrada (2026-09-28):** se rechaza el acceso con un mensaje. |
| **S12** | Aviso antes de que caduque el token de Facebook o Instagram. Sin aviso, el canal caduca y lo primero que se entera el equipo es que un post programado falla. | **Cerrada (2026-09-28):** dentro de la v1; una comprobación diaria avisa 7 días antes, con notificación y correo (F12). |

## Referencias en Postiz v2.24.0

Rutas relativas al tag `v2.24.0` de [`Zetesis-Labs/postiz-app`](https://github.com/Zetesis-Labs/postiz-app).

| Área | Dónde mirar |
|---|---|
| Identidad | `apps/backend/src/services/auth/`, `apps/backend/src/services/auth/providers/oauth.provider.ts` (login OIDC genérico ya existente) |
| Organizaciones | `libraries/nestjs-libraries/src/database/prisma/organizations/`, `apps/frontend/src/components/settings/teams.component.tsx` |
| Canales | `apps/backend/src/api/routes/integrations.controller.ts`, `no.auth.integrations.controller.ts`, `libraries/nestjs-libraries/src/integrations/social/*.provider.ts`, `dtos/posts/providers-settings/` |
| Posts | `libraries/nestjs-libraries/src/database/prisma/posts/`, `apps/frontend/src/components/new-launch/`, `apps/frontend/src/components/launches/calendar.tsx` |
| Publicación | `apps/orchestrator/src/workflows/post-workflows/post.workflow.v1.1.2.ts`, `missing.post.workflow.ts`, `refresh.token.workflow.ts` |
| Medios | `libraries/nestjs-libraries/src/upload/`, `apps/frontend/src/components/media/media.component.tsx` |
| Notificaciones | `libraries/nestjs-libraries/src/database/prisma/notifications/`, `digest.email.workflow.ts` |
| MCP | `libraries/nestjs-libraries/src/chat/start.mcp.ts`, `libraries/nestjs-libraries/src/chat/tools/` |
