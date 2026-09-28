# Constitución de postik

Principios que toda especificación, plan y cambio de código de este repositorio deben respetar. Si una spec o un cambio los contradice, se cambia la spec o el cambio, no la constitución. Cambiar la constitución va en un commit propio que lo diga.

postik programa y publica posts en redes sociales para equipos organizados en organizaciones. Se inspira en el comportamiento de Postiz y viene a sustituir a suntzu, la instancia de Postiz que corre en pelayo.

## 1. Idioma

- **El código es en inglés:** identificadores, ficheros, rutas de la API, códigos de error y mensajes de commit.
- **La documentación es en español:** specs, README, esta constitución y los comentarios, cuando los haya.
- **La interfaz está en español e inglés** (§6.1 de la funcional).

## 2. Nada heredado de Postiz

- Postiz se lee, no se copia. El fork [`Zetesis-Labs/postiz-app`](https://github.com/Zetesis-Labs/postiz-app) sirve para consultar cómo se comporta. De allí no pasa aquí ningún fichero, fragmento, componente, traducción ni icono, aunque la licencia lo permita. Por eso son dos repositorios.
- Si Postiz y la funcional discrepan, manda la funcional.

## 3. Especificación primero, con anclaje

- La funcional, en `specs/postik/alcance-reducido-v1/functional-specs.md`, describe el qué: flujos `F1`–`F18` y reglas en la sección 6.
- Cada vertical técnico tiene su spec en `specs/Sxx-*.md`, con casos `Sxx.n` en forma Dado / Cuando / Entonces. Cada caso cita los flujos (`F9`) o las reglas (`§6.5`) que cubre.
- **La suite se escribe antes que la implementación.** Un caso entra primero como test que falla y después como código que lo pone en verde.
- Cada test cita su caso. En Go, en la primera línea del comentario del test (`// S03.2 …`). En Playwright, al principio del título.
- `make spec-check` falla si un caso no tiene test o si un test cita un caso que no existe.
- Las specs se cambian antes que el código. Si el código demuestra que una spec estaba mal, se corrige la spec y el commit explica por qué.

## 4. Núcleo funcional, cáscara imperativa

- Las decisiones viven en funciones puras, sin red, base de datos, reloj ni aleatoriedad. Por ejemplo: validar un post contra las reglas de cada red, las transiciones de estado de posts y canales, el siguiente hueco libre, los permisos por rol y la caducidad de las sesiones.
- La cáscara recoge las entradas, llama al núcleo y aplica los efectos. Las tres cosas no se mezclan en el mismo bucle.
- El reloj y los generadores de identificadores se inyectan, para que los tests controlen el tiempo.
- Lo que cambia de una red a otra se resuelve con implementaciones pequeñas de una interfaz y funciones inyectadas. No hay un «proveedor base» embebido que haga de todo.

## 5. Tests verticales

- Un test vertical entra por una frontera del sistema: la API HTTP, el MCP o un trabajo programado. Comprueba lo que percibe el usuario: la respuesta, el estado que queda guardado y lo que se envía a la red.
- Los tests usan un Postgres real, que pone el devcontainer. Si no hay base de datos, el test falla; no se salta.
- Cada red social tiene un doble con `httptest` que imita su API. En CI nunca se llama a una red real. Con el proveedor OIDC pasa lo mismo: en los tests hay un emisor falso dentro del propio proceso.
- Lo que se ve en pantalla se comprueba con Playwright contra el binario construido.
- Las pruebas contra las redes reales son manuales, con cuentas de prueba, al cerrar cada vertical de redes.

## 6. Hecho quiere decir hasta la interfaz

Un vertical está hecho cuando se cumplen tres condiciones:

- todos sus casos están en verde;
- sus pantallas están cableadas de verdad, sin datos simulados;
- Rubén lo ha probado.

Si el motor y la API están en verde pero la pantalla está a medias, el vertical sigue en curso.

## 7. Un proceso, un binario, Postgres

- **Un solo binario:** sirve la API y el frontend, que va embebido, y ejecuta los trabajos programados.
- **Postgres es el único sitio donde se guarda estado:** datos, sesiones y cola de trabajos. No hay Redis, Temporal, Elasticsearch ni colas externas.
- **Frontend estático:** es una SPA. No hay Node en ejecución.
- **Medios en disco:** se guardan en un volumen y el binario los sirve en una URL pública estable.
- **Siempre encendido:** no se escala a cero, porque los posts tienen que salir a su hora.
- **Migraciones:** van versionadas y se aplican con `postik migrate` antes de arrancar. El esquema nunca se sincroniza en caliente ni con pérdida de datos.
- **Migraciones generadas:** se generan con Atlas a partir del esquema declarado. No se escriben ni se editan a mano. La única excepción es el esquema de la cola de trabajos: lo migra su propia herramienta, dentro del mismo `postik migrate`.

## 8. Seguridad

- **Secretos:** las credenciales del superadmin, la semilla TOTP, las credenciales de las redes y la clave de cifrado llegan por variables de entorno desde secretos de Kubernetes. Nunca están en el repositorio.
- **Tokens de las redes:** se guardan cifrados con una clave del despliegue.
- **Tokens personales de MCP:** se guardan como hash.
- **Aislamiento entre organizaciones:** toda lectura de datos de una organización filtra por ella en la capa de datos. Cada vertical incluye un caso que demuestra que un miembro no ve lo de otra organización.

## 9. Dependencias

- Primero la biblioteca estándar.
- Cada dependencia nueva se justifica en la spec del vertical que la introduce.
- Las versiones van fijadas.

## 10. Operación

- **Logs:** estructurados, en JSON.
- **Estado del proceso:** `/healthz` y `/readyz`.
- **Consumo:** se mide al cerrar cada vertical. La referencia a batir es suntzu, que en reposo ocupaba unos 2,5 GiB (medido el 2026-09-28).
