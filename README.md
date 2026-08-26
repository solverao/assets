# asset

CLI para la gestión masiva de archivos: extraer comprimidos, normalizar nombres y calcular checksums.

## Requisitos

- Go 1.25 o superior.

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

Formatos soportados: `.zip`, `.tar.gz`, `.tgz`, `.rar`, `.7z`.

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

- Todo pasa a minúsculas.
- Los caracteres que no son `a-z0-9` se convierten en `-`.
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

### `pipeline`

Encadena todo el flujo: `Extract -> Normalize -> Checksum -> Move`.

```bash
asset pipeline --src <origen> --dest <destino>
asset pipeline -s <origen> -d <destino>
```

- `--src` / `-s`: directorio origen (obligatorio).
- `--dest` / `-d`: directorio destino final (obligatorio).
- `--dry-run`: muestra los movimientos sin aplicarlos.

El trabajo se realiza en un directorio temporal y, al final, los archivos (incluido `checksums.txt`) se mueven al destino. El traslado usa `rename` y, si el origen y el destino están en distintos dispositivos, copia y elimina.

## Flags globales

Disponibles en todos los subcomandos:

- `--verbose` / `-v`: muestra información de depuración.
- `--workers` / `-w`: número de workers concurrentes (por defecto, el número de CPUs).
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
asset pipeline -s ./entrada -d ./salida
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

La lógica de cada subcomando vive en funciones exportadas dentro de `cmd/`
(`RunExtractionLogic`, `RunNormalizationLogic`, `RunChecksumLogic`), de modo que
`pipeline` las reutiliza. Para añadir un subcomando, sigue ese patrón: un wrapper
delgado de `cobra` (`RunE`) que delega en una función `Run...Logic` exportada.

La versión se inyecta en el binario con:

```bash
go build -ldflags "-X asset/cmd.version=<versión>" .
```

## Licencia

MIT. Consulta [LICENSE](LICENSE).

