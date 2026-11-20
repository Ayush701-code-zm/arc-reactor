package image

import (
	"archive/tar"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
)

func writeTarGz(t *testing.T, path string, write func(*tar.Writer)) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	write(tw)
	_ = tw.Close()
	_ = gz.Close()
	_ = f.Close()
}

func TestExtractTarGz(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "img.tar.gz")
	writeTarGz(t, src, func(tw *tar.Writer) {
		body := []byte("hello")
		hdr := &tar.Header{Name: "bin/hello", Mode: 0o755, Size: int64(len(body))}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write(body); err != nil {
			t.Fatal(err)
		}
	})

	dest := filepath.Join(dir, "rootfs")
	if err := ExtractTarGz(src, dest); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(dest, "bin/hello"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "hello" {
		t.Fatalf("got %q", got)
	}
}

func TestExtractRejectsAbsoluteFilePath(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "bad.tar.gz")
	writeTarGz(t, src, func(tw *tar.Writer) {
		body := []byte("x")
		hdr := &tar.Header{Name: "/etc/passwd", Mode: 0o644, Size: int64(len(body))}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write(body); err != nil {
			t.Fatal(err)
		}
	})
	if err := ExtractTarGz(src, filepath.Join(dir, "rootfs")); err == nil {
		t.Fatal("expected absolute path rejection")
	}
}

func TestExtractRejectsEscapingRelativeSymlink(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "bad.tar.gz")
	writeTarGz(t, src, func(tw *tar.Writer) {
		hdr := &tar.Header{Typeflag: tar.TypeSymlink, Name: "link", Linkname: "../outside"}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
	})
	if err := ExtractTarGz(src, filepath.Join(dir, "rootfs")); err == nil {
		t.Fatal("expected symlink escape rejection")
	}
}

func TestExtractAcceptsDotSlashPrefix(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "img.tar.gz")
	writeTarGz(t, src, func(tw *tar.Writer) {
		for _, name := range []string{"./", "./bin/"} {
			hdr := &tar.Header{Typeflag: tar.TypeDir, Name: name, Mode: 0o755}
			if err := tw.WriteHeader(hdr); err != nil {
				t.Fatal(err)
			}
		}
		body := []byte("ok")
		hdr := &tar.Header{Name: "./bin/hello", Mode: 0o755, Size: int64(len(body))}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write(body); err != nil {
			t.Fatal(err)
		}
	})
	dest := filepath.Join(dir, "rootfs")
	if err := ExtractTarGz(src, dest); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(dest, "bin/hello"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "ok" {
		t.Fatalf("got %q", got)
	}
}

func TestExtractRewritesAbsoluteSymlinkIntoRootfs(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "img.tar.gz")
	writeTarGz(t, src, func(tw *tar.Writer) {
		body := []byte("busybox")
		hdr := &tar.Header{Name: "bin/busybox", Mode: 0o755, Size: int64(len(body))}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write(body); err != nil {
			t.Fatal(err)
		}
		link := &tar.Header{Typeflag: tar.TypeSymlink, Name: "bin/sleep", Linkname: "/bin/busybox"}
		if err := tw.WriteHeader(link); err != nil {
			t.Fatal(err)
		}
	})

	dest := filepath.Join(dir, "rootfs")
	if err := ExtractTarGz(src, dest); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(dest, "bin/sleep"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "busybox" {
		t.Fatalf("got %q", got)
	}
}
