package pathsafe

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// ErrInvalidPath indicates a relative path is not safe to use under an export directory.
var ErrInvalidPath = errors.New("invalid path")

// ValidateRelativeRef checks a user-supplied relative path (no traversal, absolute paths, or NUL).
func ValidateRelativeRef(ref string) error {
	if ref == "" {
		return ErrInvalidPath
	}
	if strings.ContainsRune(ref, 0) {
		return ErrInvalidPath
	}
	if filepath.IsAbs(ref) {
		return ErrInvalidPath
	}
	if strings.HasPrefix(ref, "/") || strings.HasPrefix(ref, "\\") {
		return ErrInvalidPath
	}
	normalized := filepath.ToSlash(filepath.Clean(ref))
	if normalized == ".." || strings.HasPrefix(normalized, "../") || strings.Contains(normalized, "/../") {
		return ErrInvalidPath
	}
	for _, seg := range strings.Split(normalized, "/") {
		if seg == ".." {
			return ErrInvalidPath
		}
	}
	return nil
}

// ResolveUnderBase maps a relative path to an absolute path guaranteed to stay under baseDir.
// Symlinks are evaluated and must still resolve inside baseDir.
func ResolveUnderBase(baseDir, relative string) (string, error) {
	if err := ValidateRelativeRef(relative); err != nil {
		return "", err
	}
	baseAbs, err := filepath.Abs(baseDir)
	if err != nil {
		return "", err
	}
	targetAbs, err := filepath.Abs(filepath.Join(baseAbs, relative))
	if err != nil {
		return "", err
	}
	if err := assertUnderBase(baseAbs, targetAbs); err != nil {
		return "", err
	}
	eval, err := filepath.EvalSymlinks(targetAbs)
	if err != nil {
		if os.IsNotExist(err) {
			return targetAbs, nil
		}
		return "", err
	}
	evalAbs, err := filepath.Abs(eval)
	if err != nil {
		return "", err
	}
	if err := assertUnderBase(baseAbs, evalAbs); err != nil {
		return "", ErrInvalidPath
	}
	return evalAbs, nil
}

func assertUnderBase(baseAbs, targetAbs string) error {
	rel, err := filepath.Rel(baseAbs, targetAbs)
	if err != nil {
		return ErrInvalidPath
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return ErrInvalidPath
	}
	return nil
}

// OpenRegularFileUnderBase opens an existing regular file under baseDir (not a symlink itself).
func OpenRegularFileUnderBase(baseDir, relative string) (*os.File, error) {
	resolved, err := ResolveUnderBase(baseDir, relative)
	if err != nil {
		return nil, err
	}
	fi, err := os.Lstat(resolved)
	if err != nil {
		return nil, err
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		return nil, ErrInvalidPath
	}
	if !fi.Mode().IsRegular() {
		return nil, ErrInvalidPath
	}
	return os.Open(resolved)
}
