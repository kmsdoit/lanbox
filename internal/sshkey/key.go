// Package sshkey는 공개키 파일을 찾고 읽는다. 키 생성은 하지 않는다.
package sshkey

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// DefaultCandidates는 설정에 키가 없을 때 찾는 순서다.
func DefaultCandidates() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	return []string{
		filepath.Join(home, ".ssh", "id_ed25519.pub"),
		filepath.Join(home, ".ssh", "id_rsa.pub"),
		filepath.Join(home, ".ssh", "id_ecdsa.pub"),
	}
}

// Expand는 ~ 를 홈 디렉터리로 바꾼다.
func Expand(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if path == "~" || strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		if path == "~" {
			return home
		}
		return filepath.Join(home, strings.TrimPrefix(path, "~/"))
	}
	return path
}

// Resolve는 명시 경로, 설정 경로, 기본 키 순으로 존재하는 파일을 고른다.
// 명시 경로가 없으면 오류다. 아무것도 없으면 빈 경로와 false.
func Resolve(explicit, configured string) (string, bool, error) {
	if strings.TrimSpace(explicit) != "" {
		path := Expand(explicit)
		if _, err := os.Stat(path); err != nil {
			return "", false, fmt.Errorf("SSH 공개키를 찾지 못했습니다: %s", path)
		}
		return path, true, nil
	}
	if configured != "" {
		path := Expand(configured)
		if _, err := os.Stat(path); err == nil {
			return path, true, nil
		}
	}
	for _, path := range DefaultCandidates() {
		if _, err := os.Stat(path); err == nil {
			return path, true, nil
		}
	}
	return "", false, nil
}

// Read는 공개키 파일 내용을 한 줄로 정리해 읽는다.
func Read(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("SSH 공개키 읽기: %w", err)
	}
	text := strings.TrimSpace(string(raw))
	if text == "" {
		return "", fmt.Errorf("SSH 공개키가 비어 있습니다: %s", path)
	}
	return text, nil
}
