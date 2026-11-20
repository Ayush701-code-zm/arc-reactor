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
		// Sink-local Zip Slip barrier (shape Sonar/CodeQL recognize).
		if err := assertInside(dest, target); err != nil {
			return err
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg, tar.TypeRegA:
			parent := filepath.Dir(target)
			if err := assertInside(dest, parent); err != nil {
				return err
			}
			if err := os.MkdirAll(parent, 0o755); err != nil {
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
			parent := filepath.Dir(target)
			if err := assertInside(dest, parent); err != nil {
				return err
			}
			resolved := filepath.Clean(filepath.Join(parent, linkTarget))
			if err := assertInside(dest, resolved); err != nil {
				return err
			}
			if err := os.MkdirAll(parent, 0o755); err != nil {
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
	if filepath.IsAbs(name) || strings.HasPrefix(name, `\`) || strings.Contains(name, `:\`) {
		return "", fmt.Errorf("absolute path in archive rejected: %s", name)
	}
	root = filepath.Clean(root)
	target := filepath.Clean(filepath.Join(root, name))
	rel, err := filepath.Rel(root, target)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("path escapes rootfs: %s", name)
	}
	if err := assertInside(root, target); err != nil {
		return "", err
	}
	return target, nil
}

func assertInside(root, path string) error {
	root = filepath.Clean(root)
	path = filepath.Clean(path)
	if path != root && !strings.HasPrefix(path, root+string(os.PathSeparator)) {
		return fmt.Errorf("path escapes rootfs: %s", path)
	}
	return nil
}

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
		if err := assertInside(root, candidate); err != nil {
			return "", fmt.Errorf("symlink escapes rootfs: %s", linkTarget)
		}
		resolved = candidate
	}
	out, err := filepath.Rel(filepath.Dir(linkPath), resolved)
	if err != nil {
		return "", err
	}
	if filepath.IsAbs(out) {
		return "", fmt.Errorf("refusing absolute symlink after rewrite: %s", out)
	}
	return out, nil
}
