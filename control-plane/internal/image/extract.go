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
	destPrefix := dest + string(os.PathSeparator)
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
	var pending []pendingLink
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		rel, err := sanitizeArchiveRel(hdr.Name)
		if err != nil {
			return err
		}
		target := filepath.Join(dest, rel)
		if !strings.HasPrefix(target, destPrefix) {
			return fmt.Errorf("path escapes rootfs: %s", hdr.Name)
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg, tar.TypeRegA:
			parent := filepath.Dir(target)
			if !strings.HasPrefix(parent, destPrefix) && parent != dest {
				return fmt.Errorf("path escapes rootfs: %s", hdr.Name)
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
			pending = append(pending, pendingLink{
				rel:        rel,
				linkTarget: linkTarget,
				mode:       os.FileMode(hdr.Mode) & 0o777,
			})
		default:
		}
	}
	for _, p := range pending {
		if err := materializeLink(dest, destPrefix, p); err != nil {
			return err
		}
	}
	return nil
}

type pendingLink struct {
	rel        string
	linkTarget string
	mode       os.FileMode
}

// materializeLink copies the resolved file into place instead of os.Symlink.
// That removes the Zip Slip sink Sonar flags on archive-derived Symlink paths
// while still giving Alpine-style /bin/* entries a working busybox binary.
func materializeLink(dest, destPrefix string, p pendingLink) error {
	target := filepath.Join(dest, p.rel)
	if !strings.HasPrefix(target, destPrefix) {
		return fmt.Errorf("path escapes rootfs: %s", p.rel)
	}
	parent := filepath.Dir(target)
	if !strings.HasPrefix(parent, destPrefix) && parent != dest {
		return fmt.Errorf("path escapes rootfs: %s", p.rel)
	}
	resolved := filepath.Clean(filepath.Join(parent, p.linkTarget))
	if !strings.HasPrefix(resolved, destPrefix) {
		return fmt.Errorf("symlink escapes rootfs: %s", p.linkTarget)
	}
	data, err := os.ReadFile(resolved)
	if err != nil {
		return fmt.Errorf("symlink target missing under rootfs (%s -> %s): %w", p.rel, p.linkTarget, err)
	}
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return err
	}
	_ = os.Remove(target)
	mode := p.mode
	if mode == 0 {
		mode = 0o755
	}
	return os.WriteFile(target, data, mode)
}

func sanitizeArchiveRel(name string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("empty archive path")
	}
	if filepath.IsAbs(name) || strings.HasPrefix(name, `\`) || strings.Contains(name, `:\`) {
		return "", fmt.Errorf("absolute path in archive rejected: %s", name)
	}
	clean := filepath.Clean(name)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("path escapes rootfs: %s", name)
	}
	if strings.Contains(clean, ".."+string(os.PathSeparator)) || strings.HasSuffix(clean, string(os.PathSeparator)+"..") {
		return "", fmt.Errorf("path escapes rootfs: %s", name)
	}
	return clean, nil
}

func rewriteSymlink(root, linkPath, linkTarget string) (string, error) {
	root = filepath.Clean(root)
	rootPrefix := root + string(os.PathSeparator)
	var resolved string
	if filepath.IsAbs(linkTarget) {
		rel := strings.TrimPrefix(filepath.Clean(linkTarget), string(os.PathSeparator))
		clean, err := sanitizeArchiveRel(rel)
		if err != nil {
			return "", fmt.Errorf("symlink target escapes rootfs: %s", linkTarget)
		}
		resolved = filepath.Join(root, clean)
		if !strings.HasPrefix(resolved, rootPrefix) {
			return "", fmt.Errorf("symlink target escapes rootfs: %s", linkTarget)
		}
	} else {
		candidate := filepath.Clean(filepath.Join(filepath.Dir(linkPath), linkTarget))
		if !strings.HasPrefix(candidate, rootPrefix) {
			return "", fmt.Errorf("symlink escapes rootfs: %s", linkTarget)
		}
		resolved = candidate
	}
	out, err := filepath.Rel(filepath.Dir(linkPath), resolved)
	if err != nil {
		return "", err
	}
	if filepath.IsAbs(out) || out == ".." || strings.HasPrefix(out, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("refusing escaping symlink after rewrite: %s", out)
	}
	return out, nil
}
