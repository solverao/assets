package asset

import (
	"database/sql"
	"strings"
)

// Blob representa un archivo físico (asset_blobs).
type Blob struct {
	ID              int64
	Path            string
	Size            int64
	Checksum        string
	PartialChecksum string
	MimeType        string
	Extension       string
	OriginalName    string
}

// Asset representa una entrada lógica del árbol (assets).
type Asset struct {
	ID          int64
	UserID      string
	ParentID    sql.NullInt64
	AssetBlobID sql.NullInt64
	Type        string
	Slug        string
	Name        string
}

// Repository define las operaciones de base de datos del escáner.
type Repository interface {
	EnsureUser(userID string) error
	// WithTx ejecuta fn dentro de una única transacción de escritura.
	WithTx(fn func(Tx) error) error
	// SearchAssets busca assets por nombre/descripción vía FTS5.
	SearchAssets(userID, query string, limit int) ([]Asset, error)
}

// Tx agrupa las operaciones de escritura que se ejecutan dentro de WithTx.
type Tx interface {
	FindBlobCandidates(size int64, partial string) ([]Blob, error)
	CreateBlob(b Blob) (int64, error)
	CreateAsset(a Asset) (int64, error)
	AssetExists(userID, slug string) (bool, error)
}

// sqliteRepo es la implementación concreta.
type sqliteRepo struct {
	db *sql.DB
}

// sqliteTx implementa Tx sobre una transacción *sql.Tx.
type sqliteTx struct {
	tx *sql.Tx
}

// NewSQLiteRepo retorna el struct, que implementa Repository implícitamente.
func NewSQLiteRepo(db *sql.DB) *sqliteRepo {
	return &sqliteRepo{db: db}
}

func (r *sqliteRepo) EnsureUser(userID string) error {
	_, err := r.db.Exec(`INSERT INTO user (id, name, email) VALUES (?, ?, ?)
		ON CONFLICT(id) DO NOTHING`, userID, userID, userID+"@local")
	return err
}

func (r *sqliteRepo) WithTx(fn func(Tx) error) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := fn(&sqliteTx{tx: tx}); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *sqliteRepo) SearchAssets(userID, query string, limit int) ([]Asset, error) {
	if limit <= 0 {
		limit = 50
	}
	// Se acota el término como frase para evitar errores de sintaxis FTS.
	match := `"` + strings.ReplaceAll(query, `"`, `""`) + `"`
	rows, err := r.db.Query(`
		SELECT a.id, a.user_id, a.parent_id, a.asset_blob_id, a.type, a.slug, a.name
		FROM assets_fts f
		JOIN assets a ON a.id = f.docid
		WHERE f.assets_fts MATCH ? AND a.user_id = ?
		ORDER BY a.name LIMIT ?`, match, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var assets []Asset
	for rows.Next() {
		var a Asset
		if err := rows.Scan(&a.ID, &a.UserID, &a.ParentID, &a.AssetBlobID, &a.Type, &a.Slug, &a.Name); err != nil {
			return nil, err
		}
		assets = append(assets, a)
	}
	return assets, rows.Err()
}

func (t *sqliteTx) FindBlobCandidates(size int64, partial string) ([]Blob, error) {
	rows, err := t.tx.Query(`SELECT id, checksum FROM asset_blobs WHERE size = ? AND partial_checksum = ?`, size, partial)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var blobs []Blob
	for rows.Next() {
		var b Blob
		if err := rows.Scan(&b.ID, &b.Checksum); err != nil {
			return nil, err
		}
		b.Size = size
		b.PartialChecksum = partial
		blobs = append(blobs, b)
	}
	return blobs, rows.Err()
}

func (t *sqliteTx) CreateBlob(b Blob) (int64, error) {
	res, err := t.tx.Exec(`INSERT INTO asset_blobs (path, size, checksum, partial_checksum, mime_type, extension, original_name)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		b.Path, b.Size, b.Checksum, b.PartialChecksum, b.MimeType, b.Extension, b.OriginalName)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (t *sqliteTx) CreateAsset(a Asset) (int64, error) {
	res, err := t.tx.Exec(`INSERT INTO assets (user_id, parent_id, asset_blob_id, type, slug, name)
		VALUES (?, ?, ?, ?, ?, ?)`,
		a.UserID, a.ParentID, a.AssetBlobID, a.Type, a.Slug, a.Name)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (t *sqliteTx) AssetExists(userID, slug string) (bool, error) {
	var exists bool
	err := t.tx.QueryRow(`SELECT EXISTS(SELECT 1 FROM assets WHERE user_id = ? AND slug = ?)`,
		userID, slug).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}
