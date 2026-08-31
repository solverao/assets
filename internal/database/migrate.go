package database

import (
	"crypto/sha256"
	"database/sql"
	"embed"
	"encoding/hex"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// triggerStartRe detecta el comienzo de un CREATE TRIGGER (con o sin TEMP).
var triggerStartRe = regexp.MustCompile(`(?is)^\s*CREATE\s+(?:TEMP(?:ORARY)?\s+)?TRIGGER\b`)

// Migrate aplica las migraciones pendientes de migrationsFS en orden por
// nombre, registrando las versiones y su checksum en schema_migrations.
// Detecta drift si un archivo ya aplicado fue modificado.
func Migrate(db *sql.DB) error {
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version text PRIMARY KEY NOT NULL,
		checksum text,
		applied_at integer DEFAULT (cast(unixepoch('subsecond') * 1000 as integer)) NOT NULL
	)`); err != nil {
		return fmt.Errorf("creando schema_migrations: %w", err)
	}

	// Base de datos creada con una versión anterior sin columna checksum.
	if err := ensureColumn(db, "schema_migrations", "checksum", "text"); err != nil {
		return err
	}

	applied, err := appliedVersions(db)
	if err != nil {
		return err
	}

	names, err := migrationNames()
	if err != nil {
		return err
	}

	contents := make(map[string]string, len(names))
	for _, name := range names {
		content, err := migrationsFS.ReadFile("migrations/" + name)
		if err != nil {
			return fmt.Errorf("leyendo %s: %w", name, err)
		}
		contents[name] = string(content)
	}

	// Verifica drift y rellena checksums pendientes de migraciones ya aplicadas.
	for version, storedChecksum := range applied {
		content, ok := contents[version]
		if !ok {
			return fmt.Errorf("migración aplicada %s no existe en el binario", version)
		}
		want := checksumOf(content)
		if storedChecksum != "" && storedChecksum != want {
			return fmt.Errorf("migración %s fue modificada después de aplicarse", version)
		}
		if storedChecksum == "" {
			if _, err := db.Exec(`UPDATE schema_migrations SET checksum = ? WHERE version = ?`, want, version); err != nil {
				return fmt.Errorf("rellenando checksum de %s: %w", version, err)
			}
		}
	}

	for _, name := range names {
		if _, ok := applied[name]; ok {
			continue
		}
		if err := applyMigration(db, name, contents[name]); err != nil {
			return err
		}
	}
	return nil
}

func migrationNames() ([]string, error) {
	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return nil, fmt.Errorf("leyendo migraciones: %w", err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names, nil
}

func checksumOf(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}

func appliedVersions(db *sql.DB) (map[string]string, error) {
	rows, err := db.Query(`SELECT version, checksum FROM schema_migrations`)
	if err != nil {
		return nil, fmt.Errorf("consultando schema_migrations: %w", err)
	}
	defer rows.Close()

	applied := make(map[string]string)
	for rows.Next() {
		var v string
		var cs sql.NullString
		if err := rows.Scan(&v, &cs); err != nil {
			return nil, err
		}
		applied[v] = cs.String
	}
	return applied, rows.Err()
}

func ensureColumn(db *sql.DB, table, column, typ string) error {
	rows, err := db.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			cid    int
			name   string
			ctype  string
			notnul int
			dflt   sql.NullString
			pk     int
		)
		if err := rows.Scan(&cid, &name, &ctype, &notnul, &dflt, &pk); err != nil {
			return err
		}
		if name == column {
			return nil
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}

	_, err = db.Exec(fmt.Sprintf(`ALTER TABLE %s ADD COLUMN %s %s`, table, column, typ))
	return err
}

func applyMigration(db *sql.DB, name, content string) error {
	statements, err := splitSQL(content)
	if err != nil {
		return fmt.Errorf("parseando %s: %w", name, err)
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, stmt := range statements {
		if _, err := tx.Exec(stmt); err != nil {
			return fmt.Errorf("aplicando %s: %w", name, err)
		}
	}

	if _, err := tx.Exec(`INSERT INTO schema_migrations (version, checksum) VALUES (?, ?)`, name, checksumOf(content)); err != nil {
		return err
	}
	return tx.Commit()
}

// splitSQL divide un script en statements separados por ';' de nivel superior,
// ignorando comentarios y respetando literales, identificadores y bloques
// CREATE TRIGGER ... BEGIN ... END; (cuyo ';' interno no separa).
func splitSQL(content string) ([]string, error) {
	var statements []string
	var current strings.Builder

	flush := func() {
		if s := strings.TrimSpace(current.String()); s != "" {
			statements = append(statements, s)
		}
		current.Reset()
	}

	i, n := 0, len(content)
	inTrigger := false

	for i < n {
		c := content[i]

		// Comentario de línea.
		if c == '-' && i+1 < n && content[i+1] == '-' {
			for i < n && content[i] != '\n' {
				i++
			}
			continue
		}

		// Comentario de bloque.
		if c == '/' && i+1 < n && content[i+1] == '*' {
			i += 2
			for i+1 < n && !(content[i] == '*' && content[i+1] == '/') {
				i++
			}
			i += 2
			continue
		}

		// Literales e identificadores entre comillas.
		switch c {
		case '\'':
			current.WriteByte(c)
			i++
			for i < n {
				current.WriteByte(content[i])
				if content[i] == '\'' {
					if i+1 < n && content[i+1] == '\'' {
						current.WriteByte(content[i+1])
						i += 2
						continue
					}
					i++
					break
				}
				i++
			}
			continue
		case '"':
			current.WriteByte(c)
			i++
			for i < n {
				current.WriteByte(content[i])
				if content[i] == '"' {
					i++
					break
				}
				i++
			}
			continue
		case '`':
			current.WriteByte(c)
			i++
			for i < n {
				current.WriteByte(content[i])
				if content[i] == '`' {
					i++
					break
				}
				i++
			}
			continue
		case '[':
			current.WriteByte(c)
			i++
			for i < n {
				current.WriteByte(content[i])
				if content[i] == ']' {
					i++
					break
				}
				i++
			}
			continue
		}

		if c == ';' {
			if inTrigger {
				if isTriggerEnd(current.String()) {
					current.WriteByte(c)
					flush()
					inTrigger = false
				} else {
					current.WriteByte(c)
				}
			} else {
				current.WriteByte(c)
				flush()
			}
			i++
			continue
		}

		current.WriteByte(c)
		if !inTrigger && triggerStartRe.MatchString(current.String()) {
			inTrigger = true
		}
		i++
	}

	if inTrigger {
		return nil, fmt.Errorf("CREATE TRIGGER sin terminar (falta END;)")
	}
	if s := strings.TrimSpace(current.String()); s != "" {
		return nil, fmt.Errorf("sentencia sin terminar en ';': %q", s)
	}
	return statements, nil
}

// isTriggerEnd indica si el buffer actual termina con la palabra clave END,
// lo que cierra el cuerpo de un trigger.
func isTriggerEnd(buf string) bool {
	fields := strings.Fields(strings.ToUpper(buf))
	if len(fields) == 0 {
		return false
	}
	return fields[len(fields)-1] == "END"
}
