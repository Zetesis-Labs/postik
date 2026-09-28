# postik

Programador de publicaciones en redes sociales: un único binario en Go, con Postgres y un frontend en TanStack embebido.

La interfaz está portada de [Postiz](https://github.com/gitroomhq/postiz-app). El servidor es propio y no contiene código de Postiz. El detalle está en la [constitución](specs/constitution.md) y en el fichero [`NOTICE`](NOTICE). El fork que usamos como referencia es [`Zetesis-Labs/postiz-app`](https://github.com/Zetesis-Labs/postiz-app).

## Especificaciones

Todo parte de [`specs/`](specs/README.md):

- la constitución;
- la especificación funcional;
- una spec técnica por vertical, con casos anclados a tests.

## Desarrollo

Todo se ejecuta dentro del devcontainer, que trae Go, Node, pnpm, Atlas y dos Postgres.

```bash
cp .devcontainer/.env.example .devcontainer/.env   # opcional: credenciales y 2FA del superadmin
docker compose -f .devcontainer/docker-compose.yml up -d
docker compose -f .devcontainer/docker-compose.yml exec dev bash
```

Dentro del contenedor:

| Comando | Qué hace |
|---|---|
| `make web-install` | Instala las dependencias del frontend |
| `make run` | Construye la SPA y el binario, migra y arranca en <http://localhost:8484> |
| `make check` | Código generado, formato, vet, migraciones, tests, tipos y anclaje de specs |
| `make e2e` | Tests de pantalla con Playwright contra el binario |
| `make migration name=…` | Genera una migración con Atlas a partir de `db/schema.sql` |

Para trabajar en el frontend con recarga en caliente, arranca `make run` en una terminal y `cd web && pnpm dev` en otra; Vite queda en <http://localhost:5184>.

## Licencia

[AGPL-3.0](LICENSE), la misma que Postiz.
