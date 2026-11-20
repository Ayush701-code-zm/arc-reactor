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

func mustExtract(t *testing.T, entries func(*tar.Writer)) string {
	t.Helper()
	dir := t.TempDir()
	src := filepath.Join(dir, "img.tar.gz")
	writeTarGz(t, src, entries)
	dest := filepath.Join(dir, "rootfs")
	if err := ExtractTarGz(src, dest); err != nil {
		t.Fatal(err)
	}
	return dest
}

func TestExtractTarGz(t *testing.T) {
	dest := mustExtract(t, func(tw *tar.Writer) {
		for _, name := range []string{"./", "./bin/"} {
			if err := tw.WriteHeader(&tar.Header{Typeflag: tar.TypeDir, Name: name, Mode: 0o755}); err != nil {
				t.Fatal(err)
			}
		}
		body := []byte("hello")
		hdr := &tar.Header{Name: "./bin/hello", Mode: 0o755, Size: int64(len(body))}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write(body); err != nil {
			t.Fatal(err)
		}
	})
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
		if err := tw.WriteHeader(&tar.Header{Typeflag: tar.TypeSymlink, Name: "link", Linkname: "../outside"}); err != nil {
			t.Fatal(err)
		}
	})
	if err := ExtractTarGz(src, filepath.Join(dir, "rootfs")); err == nil {
		t.Fatal("expected symlink escape rejection")
	}
}

func TestExtractKeepsInRootSymlinks(t *testing.T) {
	dest := mustExtract(t, func(tw *tar.Writer) {
		for _, name := range []string{"bin/", "run/", "var/", "etc/", "proc/"} {
			if err := tw.WriteHeader(&tar.Header{Typeflag: tar.TypeDir, Name: name, Mode: 0o755}); err != nil {
				t.Fatal(err)
			}
		}
		body := []byte("busybox")
		hdr := &tar.Header{Name: "bin/busybox", Mode: 0o755, Size: int64(len(body))}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write(body); err != nil {
			t.Fatal(err)
		}
		links := []struct {
			name, target string
		}{
			{"bin/sleep", "/bin/busybox"},
			{"var/run", "../run"},
			{"etc/mtab", "../proc/mounts"},
		}
		for _, l := range links {
			if err := tw.WriteHeader(&tar.Header{Typeflag: tar.TypeSymlink, Name: l.name, Linkname: l.target}); err != nil {
				t.Fatal(err)
			}
		}
	})

	got, err := os.Readlink(filepath.Join(dest, "bin/sleep"))
	if err != nil {
		t.Fatal(err)
	}
	if filepath.IsAbs(got) {
		t.Fatalf("expected relative symlink, got %q", got)
	}
	resolved := filepath.Clean(filepath.Join(dest, "bin", got))
	if resolved != filepath.Join(dest, "bin/busybox") {
		t.Fatalf("resolved=%s", resolved)
	}

	got, err = os.Readlink(filepath.Join(dest, "var/run"))
	if err != nil {
		t.Fatal(err)
	}
	if got != "../run" {
		t.Fatalf("var/run -> %q", got)
	}

	got, err = os.Readlink(filepath.Join(dest, "etc/mtab"))
	if err != nil {
		t.Fatal(err)
	}
	if got != "../proc/mounts" {
		t.Fatalf("etc/mtab -> %q", got)
	}
}
