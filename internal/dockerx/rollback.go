package dockerx

import (
	"context"
	"errors"
	"fmt"
)

// Rollback은 한 번의 생성에서 만든 컨테이너와 볼륨을 실패 시 되돌린다.
type Rollback struct {
	Containers []string
	Volumes    []string
}

// AddContainer는 롤백 대상 컨테이너를 기록한다.
func (r *Rollback) AddContainer(id string) {
	if id == "" {
		return
	}
	r.Containers = append(r.Containers, id)
}

// AddVolume는 이번에 새로 만든 볼륨만 기록한다.
func (r *Rollback) AddVolume(name string) {
	if name == "" {
		return
	}
	r.Volumes = append(r.Volumes, name)
}

// Remover는 롤백이 지우는 동작이다. *Client가 구현한다.
type Remover interface {
	RemoveContainer(ctx context.Context, id string) error
	RemoveVolume(ctx context.Context, name string) error
}

// Undo는 나중에 만든 것부터 지운다. 일부 실패해도 나머지를 계속 지운다.
func (r *Rollback) Undo(ctx context.Context, c Remover) error {
	if r == nil || c == nil {
		return nil
	}
	var errs []error
	for i := len(r.Containers) - 1; i >= 0; i-- {
		if err := c.RemoveContainer(ctx, r.Containers[i]); err != nil {
			errs = append(errs, err)
		}
	}
	for i := len(r.Volumes) - 1; i >= 0; i-- {
		if err := c.RemoveVolume(ctx, r.Volumes[i]); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) == 0 {
		return nil
	}
	return fmt.Errorf("롤백 중 일부 삭제에 실패했습니다: %w", errors.Join(errs...))
}
