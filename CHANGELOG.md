# Changelog

Todas las notas relevantes de este proyecto se documentarán en este archivo.

El formato se basa en [Keep a Changelog](https://keepachangelog.com/es-1.0.0/)
y el proyecto sigue [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Añadido

- Subcomando `extract` para extraer `.zip`, `.tar.gz`, `.tgz`, `.rar` y `.7z` de forma concurrente.
- Subcomando `normalize` para renombrar archivos y carpetas a *slugs* (con transliteración Unicode → ASCII vía `gosimple/slug`).
- Subcomando `checksum` que genera `checksums.txt` con SHA-256.
- Subcomando `process` que encadena `Extract -> Normalize -> Checksum -> Move`.
- Subcomando `ingest` que ejecuta `process` e indexa el resultado en la base de datos.
- Subcomando `scan` que indexa un árbol de archivos en SQLite.
- Subcomando `db` para gestionar la base de datos (`init`, `info`, `migrate`, `migrations`, `delete`).
- Subcomando `vault` para gestionar bóvedas (bases de datos con nombre): `list`, `create`, `use`, `current`, `delete`, `path`.
- Flags globales `--verbose`, `--workers`, `--sync` y `--db` (o variable `ASSET_DB`).
- Flag `--dry-run` en `normalize`, `process` e `ingest`.
- Flag `--output` en `checksum`.
- Flag `--min-free` en `extract`, `process` e `ingest`.
- Flag `--remove-source` en `extract`, `process` e `ingest`.
- Flag `--error-dir` (cuarentena) y manifiesto `errores.txt` para archivos corruptos o incompletos.
- Flag `--password` para comprimidos cifrados (RAR y 7z).
- `process` e `ingest` copian también los archivos que no son comprimidos (flag `--include-files` en `extract`).
- Cancelación de la extracción vía `context.Context` (Ctrl+C).
- Aislamiento de código dependiente de SO con build tags (compila en Windows y macOS).
- Migración del driver de SQLite a `modernc.org/sqlite` (pure Go, sin cgo) y búsqueda full-text FTS5.
- Soporte de comprimidos multiparte RAR (`.part1.rar`) y 7z (`.7z.001`).
- Protección contra zip-slip y symlinks, y límite de tamaño por archivo descomprimido.

### Corregido

- Colisión al extraer comprimidos con el mismo nombre en distintos subdirectorios.
- Checksums que se calculaban y se descartaban sin generar salida.
