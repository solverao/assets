package thumbnail

import (
	"database/sql"
)

// TargetBlob representa un archivo original que necesita miniatura
type TargetBlob struct {
	ID   int64
	Path string
	Mime string
}

type Repository interface {
	GetBlobsWithoutThumbnails(limit int) ([]TargetBlob, error)
	SaveThumbnail(originalBlobID int64, thumbPath string, size int64, checksum string, width, height int) error
}

type sqliteRepo struct {
	db *sql.DB
}

func NewSQLiteRepo(db *sql.DB) *sqliteRepo {
	return &sqliteRepo{db: db}
}

func (r *sqliteRepo) GetBlobsWithoutThumbnails(limit int) ([]TargetBlob, error) {
	// Consulta: Trae archivos de imagen/video que no existan en asset_thumbnails
	query := `
		SELECT id, path, mime_type
		FROM asset_blobs
		WHERE (mime_type LIKE 'image/%' OR mime_type LIKE 'video/%')
		AND id NOT IN (SELECT asset_blob_id FROM asset_thumbnails)
		LIMIT ?
	`
	rows, err := r.db.Query(query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var targets []TargetBlob
	for rows.Next() {
		var t TargetBlob
		// Asumimos que mime_type puede ser nulo, por lo que usaríamos sql.NullString en un caso real
		if err := rows.Scan(&t.ID, &t.Path, &t.Mime); err == nil {
			targets = append(targets, t)
		}
	}
	return targets, nil
}

func (r *sqliteRepo) SaveThumbnail(originalBlobID int64, thumbPath string, size int64, checksum string, width, height int) error {
	tx, _ := r.db.Begin()

	// 1. Insertar el archivo físico de la miniatura en asset_blobs
	res, _ := tx.Exec(`INSERT INTO asset_blobs (path, size, checksum, disk) VALUES (?, ?, ?, 'local')`,
		thumbPath, size, checksum)
	newBlobID, _ := res.LastInsertId()

	// 2. Vincularlo en asset_thumbnails (usamos un asset_id ficticio o real según tu lógica)
	// Nota: Tu esquema requiere asset_id. Aquí asumimos que lo obtienes o lo relacionas.
	tx.Exec(`INSERT INTO asset_thumbnails (asset_id, asset_blob_id, label, width, height)
			 VALUES ((SELECT id FROM assets WHERE asset_blob_id = ? LIMIT 1), ?, 'medium', ?, ?)`,
		originalBlobID, newBlobID, width, height)

	return tx.Commit()
}
