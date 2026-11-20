package image

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func ExtractTarGz(src, dest string) error {
	dest = filepath.Clean(dest)
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return err
	}
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("gzip: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		target, err := safeJoin(dest, hdr.Name)
		if err != nil {
			return err
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg, tar.TypeRegA:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode)&0o777)
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, tr); err != nil {
				_ = out.Close()
				return err
			}
			if err := out.Close(); err != nil {
				return err
			}
		case tar.TypeSymlink:
			linkTarget, err := rewriteSymlink(dest, target, hdr.Linkname)
			if err != nil {
				return err
			}
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			_ = os.Remove(target)
			if err := os.Symlink(linkTarget, target); err != nil {
				return err
			}
		default:
		}
	}
	return nil
}

func safeJoin(root, name string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("empty archive path")
	}
	if filepath.IsAbs(name) {
		return "", fmt.Errorf("absolute path in archive rejected: %s", name)
	}
	clean := filepath.Clean(name)
	if clean == ".." || strings.HasPrefix(clean, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("path escapes rootfs: %s", name)
	}
	target := filepath.Join(root, clean)
	rel, err := filepath.Rel(root, target)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("path escapes rootfs: %s", name)
	}
	return target, nil
}

// rewriteSymlink maps archive symlink targets into the rootfs.
// Absolute targets like /bin/busybox mean "inside the guest root", not the host.
// We store a relative link so the host never gets an absolute escape symlink.
func rewriteSymlink(root, linkPath, linkTarget string) (string, error) {
	var resolved string
	if filepath.IsAbs(linkTarget) {
		rel := strings.TrimPrefix(filepath.Clean(linkTarget), string(os.PathSeparator))
		var err error
		resolved, err = safeJoin(root, rel)
		if err != nil {
			return "", fmt.Errorf("symlink target escapes rootfs: %s", linkTarget)
		}
	} else {
		candidate := filepath.Clean(filepath.Join(filepath.Dir(linkPath), linkTarget))
		rel, err := filepath.Rel(root, candidate)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
			return "", fmt.Errorf("symlink escapes rootfs: %s", linkTarget)
		}
		resolved = candidate
	}
	out, err := filepath.Rel(filepath.Dir(linkPath), resolved)
	if err != nil {
		return "", err
	}
	return out, nil
}
