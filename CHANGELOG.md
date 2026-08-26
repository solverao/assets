# Changelog

Todas las notas relevantes de este proyecto se documentarán en este archivo.

El formato se basa en [Keep a Changelog](https://keepachangelog.com/es-1.0.0/)
y el proyecto sigue [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Añadido

- Subcomando `extract` para extraer `.zip`, `.tar.gz`, `.tgz`, `.rar` y `.7z` de forma concurrente.
- Subcomando `normalize` para renombrar archivos y carpetas a *slugs*.
- Subcomando `checksum` que genera `checksums.txt` con SHA-256.
- Subcomando `pipeline` que encadena `Extract -> Normalize -> Checksum -> Move`.
- Flags globales `--verbose` y `--workers`.
- Flag `--dry-run` en `normalize` y `pipeline`.
- Flag `--output` en `checksum`.
- Protección contra zip-slip y symlinks, y límite de tamaño por archivo descomprimido.

### Corregido

- Colisión al extraer comprimidos con el mismo nombre en distintos subdirectorios.
- Checksums que se calculaban y se descartaban sin generar salida.
