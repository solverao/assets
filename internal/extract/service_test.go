package extract

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
)

func TestDetectArchiveType(t *testing.T) {
	cases := map[string]string{
		"a.zip":    "zip",
		"a.tar.gz": "targz",
		"a.tgz":    "targz",
		"a.rar":    "rar",
		"a.7z":     "7z",
		"A.ZIP":    "zip",
		"a.gz":     "",
		"a.txt":    "",
		"a.tar":    "",
		"sin-ext":  "",
	}
	for path, want := range cases {
		if got := detectArchiveType(path); got != want {
			t.Errorf("detectArchiveType(%q) = %q, want %q", path, got, want)
		}
	}
}

func makeZip(t *testing.T, path string, files map[string]string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	w := zip.NewWriter(f)
	for name, content := range files {
		fw, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fw.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
}

func makeTarGz(t *testing.T, path string, files map[string]string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	for name, content := range files {
		hdr := &tar.Header{Name: name, Mode: 0644, Size: int64(len(content)), Typeflag: tar.TypeReg}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestExtractZipFlattensSingleDir(t *testing.T) {
	src := filepath.Join(t.TempDir(), "proyecto.zip")
	makeZip(t, src, map[string]string{"proyecto/archivo.txt": "hola"})

	dest := t.TempDir()
	if err := extractZip(src, filepath.Join(dest, "proyecto"), false); err != nil {
		t.Fatal(err)
	}
	if err := flattenSingleDirectory(filepath.Join(dest, "proyecto"), false); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(dest, "proyecto", "archivo.txt"))
	if err != nil {
		t.Fatalf("archivo aplanado no encontrado: %v", err)
	}
	if string(data) != "hola" {
		t.Fatalf("contenido = %q, want %q", data, "hola")
	}
	if _, err := os.Stat(filepath.Join(dest, "proyecto", "proyecto")); !os.IsNotExist(err) {
		t.Fatalf("se esperaba que la carpeta anidada no existiera")
	}
}

func TestExtractTarGz(t *testing.T) {
	src := filepath.Join(t.TempDir(), "proyecto.tar.gz")
	makeTarGz(t, src, map[string]string{"dir/archivo.txt": "hola"})

	dest := t.TempDir()
	if err := extractTarGz(src, filepath.Join(dest, "proyecto"), false); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(dest, "proyecto", "dir", "archivo.txt"))
	if err != nil {
		t.Fatalf("archivo extraído no encontrado: %v", err)
	}
	if string(data) != "hola" {
		t.Fatalf("contenido = %q, want %q", data, "hola")
	}
}

func TestExtractAllPreservesSubdirs(t *testing.T) {
	src := t.TempDir()
	for _, d := range []string{"a", "b"} {
		if err := os.MkdirAll(filepath.Join(src, d), 0755); err != nil {
			t.Fatal(err)
		}
	}
	makeZip(t, filepath.Join(src, "a", "foo.zip"), map[string]string{"x.txt": "a"})
	makeZip(t, filepath.Join(src, "b", "foo.zip"), map[string]string{"x.txt": "b"})

	dest := t.TempDir()
	if err := NewExtractorService().ExtractAll(src, dest, 2, false, 0); err != nil {
		t.Fatal(err)
	}

	for _, d := range []string{"a", "b"} {
		data, err := os.ReadFile(filepath.Join(dest, d, "foo", "x.txt"))
		if err != nil {
			t.Fatalf("archivo de %s no encontrado: %v", d, err)
		}
		if string(data) != d {
			t.Fatalf("contenido de %s = %q, want %q", d, data, d)
		}
	}
}

func TestExtractZipZipSlip(t *testing.T) {
	src := filepath.Join(t.TempDir(), "evil.zip")
	makeZip(t, src, map[string]string{"../evil.txt": "boom"})

	if err := extractZip(src, t.TempDir(), false); err == nil {
		t.Fatal("se esperaba error de zipslip")
	}
}

func TestExtractTarGzZipSlip(t *testing.T) {
	src := filepath.Join(t.TempDir(), "evil.tar.gz")
	makeTarGz(t, src, map[string]string{"../evil.txt": "boom"})

	if err := extractTarGz(src, t.TempDir(), false); err == nil {
		t.Fatal("se esperaba error de zipslip")
	}
}

func TestExtractZipSkipsSymlink(t *testing.T) {
	src := filepath.Join(t.TempDir(), "link.zip")
	f, err := os.Create(src)
	if err != nil {
		t.Fatal(err)
	}
	w := zip.NewWriter(f)
	hdr := &zip.FileHeader{Name: "link"}
	hdr.SetMode(os.ModeSymlink | 0777)
	fw, err := w.CreateHeader(hdr)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fw.Write([]byte("target")); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	dest := t.TempDir()
	if err := extractZip(src, dest, false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(dest, "link")); !os.IsNotExist(err) {
		t.Fatalf("el symlink no debería haberse creado: %v", err)
	}
}

func TestSanitizeMode(t *testing.T) {
	m := sanitizeMode(os.ModeSetuid | os.ModeSetgid | os.ModeSticky | 0755)
	if m != 0755 {
		t.Fatalf("sanitizeMode = %o, want 0755", m)
	}
}

func TestExtractAllCleansPartialOnError(t *testing.T) {
	src := t.TempDir()
	makeZip(t, filepath.Join(src, "evil.zip"), map[string]string{
		"dir/":        "",
		"../evil.txt": "boom",
	})

	dest := t.TempDir()
	if err := NewExtractorService().ExtractAll(src, dest, 1, false, 0); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(dest, "evil")); !os.IsNotExist(err) {
		t.Fatalf("se esperaba que la carpeta parcial fuera eliminada: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dest, "evil.txt")); !os.IsNotExist(err) {
		t.Fatalf("no debería haberse creado evil.txt: %v", err)
	}
}

func TestExtractZipRefusesSymlinkInDest(t *testing.T) {
	src := filepath.Join(t.TempDir(), "a.zip")
	makeZip(t, src, map[string]string{"link/evil.txt": "boom"})

	dest := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(dest, "link")); err != nil {
		t.Skipf("no se pudieron crear symlinks: %v", err)
	}

	if err := extractZip(src, dest, false); err == nil {
		t.Fatal("se esperaba error por symlink preexistente en dest")
	}

	if _, err := os.Stat(filepath.Join(outside, "evil.txt")); !os.IsNotExist(err) {
		t.Fatalf("no debería escribirse fuera del destino: %v", err)
	}
}

func TestMkdirAllNoSymlink(t *testing.T) {
	root := t.TempDir()
	if err := mkdirAllNoSymlink(root, filepath.Join(root, "a", "b", "c"), 0755); err != nil {
		t.Fatal(err)
	}
	if fi, err := os.Stat(filepath.Join(root, "a", "b", "c")); err != nil || !fi.IsDir() {
		t.Fatalf("directorio no creado: %v", err)
	}
}

func TestResolveExtractWorkers(t *testing.T) {
	if got := resolveExtractWorkers(t.TempDir(), 7); got != 7 {
		t.Fatalf("resolveExtractWorkers(7) = %d, want 7", got)
	}
	if got := resolveExtractWorkers(t.TempDir(), 0); got < 1 {
		t.Fatalf("resolveExtractWorkers(0) = %d, want >= 1", got)
	}
}

func TestCheckFreeSpace(t *testing.T) {
	dest := t.TempDir()
	if err := checkFreeSpace(dest, 0); err != nil {
		t.Fatalf("minFree=0 debería desactivar la comprobación: %v", err)
	}
	if err := checkFreeSpace(dest, int64(1)<<60); err == nil {
		t.Fatal("se esperaba error por espacio insuficiente")
	}
}
