package e2e

import (
	"os"
	"os/exec"
	"testing"
)

// TestFamilies는 LANBOX_E2E=1 이고 Docker 데몬이 있을 때만 실제 이미지를 받는다.
// 계획서의 완료 조건(다른 PC에서 SSH)은 이 환경에서 자동으로 확인하지 않는다.
func TestFamilies(t *testing.T) {
	if os.Getenv("LANBOX_E2E") != "1" {
		t.Skip("LANBOX_E2E=1 이 아니면 실제 이미지 풀과 컨테이너 생성은 건너뜁니다")
	}
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker CLI 없음")
	}
	t.Log("e2e 자리입니다. 대상 Linux 호스트에서 init 후 create/purge를 실행하세요.")
}
