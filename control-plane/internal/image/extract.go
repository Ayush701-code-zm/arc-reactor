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
		if rel == "" {
			continue
		}
		target := filepath.Join(dest, rel)
		if !insideRoot(dest, destPrefix, target) {
			return fmt.Errorf("path escapes rootfs: %s", hdr.Name)
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg, tar.TypeRegA:
			parent := filepath.Dir(target)
			if !insideRoot(dest, destPrefix, parent) {
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
			linkTarget, err := rewriteSymlink(dest, destPrefix, target, hdr.Linkname)
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
	for len(pending) > 0 {
		var left []pendingLink
		progress := false
		for _, p := range pending {
			done, err := materializeLink(dest, destPrefix, p)
			if err != nil {
				return err
			}
			if done {
				progress = true
				continue
			}
			left = append(left, p)
		}
		if !progress {
			break
		}
		pending = left
	}
	return nil
}

type pendingLink struct {
	rel        string
	linkTarget string
	mode       os.FileMode
}

func materializeLink(dest, destPrefix string, p pendingLink) (bool, error) {
	target := filepath.Join(dest, p.rel)
	if !insideRoot(dest, destPrefix, target) {
		return false, fmt.Errorf("path escapes rootfs: %s", p.rel)
	}
	parent := filepath.Dir(target)
	if !insideRoot(dest, destPrefix, parent) {
		return false, fmt.Errorf("path escapes rootfs: %s", p.rel)
	}
	resolved := filepath.Clean(filepath.Join(parent, p.linkTarget))
	if !insideRoot(dest, destPrefix, resolved) {
		return false, fmt.Errorf("symlink escapes rootfs: %s", p.linkTarget)
	}
	st, err := os.Stat(resolved)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return false, err
	}
	_ = os.Remove(target)
	if st.IsDir() {
		return true, os.MkdirAll(target, 0o755)
	}
	data, err := os.ReadFile(resolved)
	if err != nil {
		return false, err
	}
	mode := p.mode
	if mode == 0 {
		mode = 0o755
	}
	return true, os.WriteFile(target, data, mode)
}

func sanitizeArchiveRel(name string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("empty archive path")
	}
	if filepath.IsAbs(name) || strings.HasPrefix(name, `\`) || strings.Contains(name, `:\`) {
		return "", fmt.Errorf("absolute path in archive rejected: %s", name)
	}
	name = strings.TrimPrefix(name, "./")
	clean := filepath.Clean(name)
	if clean == "." || clean == "" {
		return "", nil
	}
	if clean == ".." || strings.HasPrefix(clean, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("path escapes rootfs: %s", name)
	}
	if strings.Contains(clean, ".."+string(os.PathSeparator)) || strings.HasSuffix(clean, string(os.PathSeparator)+"..") {
		return "", fmt.Errorf("path escapes rootfs: %s", name)
	}
	return clean, nil
}

func rewriteSymlink(root, rootPrefix, linkPath, linkTarget string) (string, error) {
	var resolved string
	if filepath.IsAbs(linkTarget) {
		rel := strings.TrimPrefix(filepath.Clean(linkTarget), string(os.PathSeparator))
		clean, err := sanitizeArchiveRel(rel)
		if err != nil {
			return "", fmt.Errorf("symlink target escapes rootfs: %s", linkTarget)
		}
		if clean == "" {
			resolved = root
		} else {
			resolved = filepath.Join(root, clean)
		}
	} else {
		resolved = filepath.Clean(filepath.Join(filepath.Dir(linkPath), linkTarget))
	}
	if !insideRoot(root, rootPrefix, resolved) {
		return "", fmt.Errorf("symlink escapes rootfs: %s", linkTarget)
	}
	out, err := filepath.Rel(filepath.Dir(linkPath), resolved)
	if err != nil {
		return "", err
	}
	if filepath.IsAbs(out) {
		return "", fmt.Errorf("refusing absolute symlink after rewrite: %s", out)
	}
	check := filepath.Clean(filepath.Join(filepath.Dir(linkPath), out))
	if !insideRoot(root, rootPrefix, check) {
		return "", fmt.Errorf("symlink escapes rootfs: %s", linkTarget)
	}
	return out, nil
}

func insideRoot(root, rootPrefix, path string) bool {
	return path == root || strings.HasPrefix(path, rootPrefix)
}
