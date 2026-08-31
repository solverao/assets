# asset

CLI para la gestión masiva de archivos: extraer comprimidos, normalizar nombres y calcular checksums.

## Requisitos

- Go 1.25 o superior.

El proyecto no usa cgo: el driver de SQLite es `modernc.org/sqlite` (pure Go) y la
búsqueda full-text usa FTS5, por lo que el binario compila de forma cruzada
(`GOOS=windows go build ./...`, `GOOS=darwin go build ./...`) sin toolchain de C.

## Instalación

```bash
go build ./...
```

Esto genera el binario `asset`. También puedes ejecutarlo directamente:

```bash
go run . <subcomando>
```

## Comandos

### `extract`

Extrae de forma concurrente todos los archivos comprimidos de un directorio.

```bash
asset extract --src <origen> --dest <destino>
asset extract -s <origen> -d <destino>
```

- `--src` / `-s`: directorio origen (obligatorio).
- `--dest` / `-d`: directorio destino (obligatorio).
- `--remove-source`: borra cada comprimido del origen tras extraerlo con éxito.
- `--error-dir`: directorio de cuarentena para los que fallan (por defecto `.errores` junto a `dest`); se escribe un `errores.txt` con el motivo.
- `--password`: contraseña para archivos cifrados (RAR y 7z).

Formatos soportados: `.zip`, `.tar.gz`, `.tgz`, `.rar`, `.7z` (incluidos los multiparte `.part1.rar` y `.7z.001`).

Comportamiento:

- Conserva la estructura de subdirectorios del origen en el destino.
- Si un comprimido contiene una única subcarpeta, la "aplana" un nivel para evitar carpetas dobles.
- La extracción se ejecuta en paralelo usando todas las CPUs disponibles.

### `normalize`

Renombra recursivamente archivos y carpetas a *slugs* en minúsculas.

```bash
asset normalize --dir <directorio>
asset normalize -d <directorio>
```

- `--dir` / `-d`: directorio a normalizar (obligatorio).
- `--dry-run`: muestra los cambios sin aplicarlos.

Reglas:

- Los acentos y alfabetos no latinos se transliteran a ASCII (p. ej. `café` → `cafe`, `Москва` → `moskva`, `影師` → `ying-shi`).
- Todo pasa a minúsculas.
- Los caracteres que no son `a-z0-9` se convierten en `-` (incluido `_`).
- La extensión se conserva en minúsculas.
- Si el nombre queda vacío, se usa `item`.
- En caso de colisión se añade un sufijo numérico (`-1`, `-2`, ...).

### `checksum`

Calcula el SHA-256 de todos los archivos de un directorio y escribe el resultado en `checksums.txt`.

```bash
asset checksum --dir <directorio>
asset checksum -d <directorio>
```

- `--dir` / `-d`: directorio a procesar (obligatorio).
- `--output` / `-o`: nombre del fichero de checksums (por defecto `checksums.txt`).

El archivo se crea dentro del directorio procesado, con una entrada por línea:

```text
<sha256>  <ruta/relativa>
```

### `process`

Procesa el flujo completo de archivos: `Extract -> Normalize -> Checksum -> Move`.

```bash
asset process --src <origen> --dest <destino>
asset process -s <origen> -d <destino>
```

- `--src` / `-s`: directorio origen (obligatorio).
- `--dest` / `-d`: directorio destino final (obligatorio).
- `--dry-run`: muestra los movimientos sin aplicarlos.
- `--min-free`: espacio libre mínimo en destino (bytes, por defecto 1 GiB).
- `--remove-source`: borra cada comprimido del origen tras extraerlo con éxito.
- `--error-dir`: directorio de cuarentena para los que fallan (por defecto `.errores` junto a `dest`).
- `--password`: contraseña para archivos cifrados (RAR y 7z).

El trabajo se realiza en un directorio temporal y, al final, los archivos (incluido `checksums.txt`) se mueven al destino. El traslado usa `rename` y, si el origen y el destino están en distintos dispositivos, copia y elimina.

### `ingest`

Ejecuta `process` y después indexa el resultado en la base de datos SQLite.

```bash
asset ingest --src <origen> --dest <destino> --db <base-de-datos>
```

- `--src` / `-s`, `--dest` / `-d`, `--dry-run`, `--min-free`, `--remove-source`, `--error-dir`, `--password`: igual que en `process`.
- `--db`: ruta de la base de datos (o variable de entorno `ASSET_DB`).

### `vault`

Gestiona bóvedas (bases de datos con nombre).
El registro de bóvedas vive en el directorio de configuración del sistema
(`~/.config/asset/vaults.json` en Linux).

```bash
asset vault create fotos          # crea y registra una bóveda
asset vault use fotos             # la fija como bóveda actual
asset vault list                  # lista bóvedas (la actual con *)
asset vault current               # muestra la bóveda actual
asset vault path fotos            # imprime su ruta
asset vault delete fotos --files  # desregistra y borra sus archivos
```

- `vault create <nombre> [--path <dir>]`: crea la BD y la registra. Sin `--path`,
  usa `<config>/asset/vaults/<nombre>/assets.db`.
- `vault delete <nombre> [--yes] [--files]`: desregistra; con `--files` borra
  también la base de datos y sus archivos auxiliares.

Cuando no se pasa `--db`, `scan` e `ingest` usan la **bóveda actual** (o `ASSET_DB` si está definida).

### `scan` y `db`

```bash
asset scan -d <directorio>        # indexa un árbol en la base de datos
asset db info                     # estado de la base de datos
asset db migrations               # lista las migraciones aplicadas
```

## Flags globales

Disponibles en todos los subcomandos:

- `--verbose` / `-v`: muestra información de depuración.
- `--workers` / `-w`: número de workers concurrentes (0 = auto).
- `--sync`: sincroniza las escrituras a disco con fsync (por defecto activado).
- `--db`: ruta de la base de datos SQLite (o variable de entorno `ASSET_DB`).
- `--version`: muestra la versión.

## Completions de shell

Cobra genera scripts de autocompletado para los shells más comunes:

```bash
asset completion bash > /etc/bash_completion.d/asset
asset completion zsh > "${fpath[1]}/_asset"
asset completion fish > ~/.config/fish/completions/asset.fish
```

## Ejemplo

```bash
# Extraer todos los comprimidos de ./entrada a ./extraido
asset extract -s ./entrada -d ./extraido

# Normalizar los nombres de ./extraido
asset normalize -d ./extraido

# Generar checksums de ./extraido
asset checksum -d ./extraido

# O todo de una vez
asset process -s ./entrada -d ./salida

# Procesar e indexar en la base de datos
asset ingest -s ./entrada -d ./salida --db ./assets.db
```

## Verificación

```bash
go build ./...
go vet ./...
go test ./...
```

O con el `Makefile`:

```bash
make build
make vet
make lint
make test
make cover
```

## Desarrollo

La lógica de cada subcomando vive en servicios exportados dentro de `internal/`
(`extract.ExtractorService`, `normalize.NormalizerService`, `checksum.ChecksumService`),
de modo que `process` e `ingest` los reutilizan. Para añadir un subcomando, sigue ese
patrón: un wrapper delgado de `cobra` (`RunE`) que delega en el servicio correspondiente.

La versión se inyecta en el binario con:

```bash
go build -ldflags "-X asset/cmd.version=<versión>" .
```

## Licencia

MIT. Consulta [LICENSE](LICENSE).

