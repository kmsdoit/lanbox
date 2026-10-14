package dockerx

import (
	"context"
	"errors"
	"testing"
)

type fakeDeleter struct {
	containers []string
	volumes    []string
	failVolume string
}

func (f *fakeDeleter) RemoveContainer(ctx context.Context, id string) error {
	f.containers = append(f.containers, id)
	return nil
}

func (f *fakeDeleter) RemoveVolume(ctx context.Context, name string) error {
	if name == f.failVolume {
		return errors.New("busy")
	}
	f.volumes = append(f.volumes, name)
	return nil
}

func TestRollbackOrder(t *testing.T) {
	rb := &Rollback{}
	rb.AddContainer("c1")
	rb.AddContainer("c2")
	rb.AddVolume("v1")
	rb.AddVolume("v2")
	fake := &fakeDeleter{}
	if err := rb.Undo(context.Background(), fake); err != nil {
		t.Fatal(err)
	}
	if len(fake.containers) != 2 || fake.containers[0] != "c2" || fake.containers[1] != "c1" {
		t.Fatalf("containers %v", fake.containers)
	}
	if fake.volumes[0] != "v2" || fake.volumes[1] != "v1" {
		t.Fatalf("volumes %v", fake.volumes)
	}
}

func TestRollbackContinuesAfterVolumeError(t *testing.T) {
	rb := &Rollback{}
	rb.AddVolume("keep")
	rb.AddVolume("busy")
	fake := &fakeDeleter{failVolume: "busy"}
	if err := rb.Undo(context.Background(), fake); err == nil {
		t.Fatal("expected error")
	}
	if len(fake.volumes) != 1 || fake.volumes[0] != "keep" {
		t.Fatalf("volumes %v", fake.volumes)
	}
}
