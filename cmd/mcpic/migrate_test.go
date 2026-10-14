package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"testing"
)

type mockMigrateSrc struct {
	data   map[string][]byte
	getErr map[string]error
}

func (m *mockMigrateSrc) Get(_ context.Context, key string) (io.ReadCloser, error) {
	if err := m.getErr[key]; err != nil {
		return nil, err
	}
	return io.NopCloser(bytes.NewReader(m.data[key])), nil
}

type mockMigrateDst struct {
	data     map[string][]byte
	putErr   map[string]error
	headSize map[string]int64 // 覆盖 Head 返回的大小，模拟校验不一致
}

func (m *mockMigrateDst) Put(_ context.Context, key string, r io.Reader) error {
	if err := m.putErr[key]; err != nil {
		return err
	}
	data, _ := io.ReadAll(r)
	m.data[key] = data
	return nil
}

func (m *mockMigrateDst) Head(_ context.Context, key string) (int64, error) {
	data, ok := m.data[key]
	if !ok {
		return 0, os.ErrNotExist
	}
	if m.headSize != nil {
		if s, ok := m.headSize[key]; ok {
			return s, nil
		}
	}
	return int64(len(data)), nil
}

func TestMigrateAllSuccess(t *testing.T) {
	src := &mockMigrateSrc{data: map[string][]byte{"a": []byte("AAA"), "b": []byte("BB")}, getErr: map[string]error{}}
	dst := &mockMigrateDst{data: map[string][]byte{}, putErr: map[string]error{}}

	res := migrateAll(context.Background(), src, dst, []string{"a", "b"}, false, false)
	if res.Success != 2 || res.Skipped != 0 || len(res.Failed) != 0 {
		t.Fatalf("res = %+v", res)
	}
	if string(dst.data["a"]) != "AAA" {
		t.Fatalf("dst a = %q", dst.data["a"])
	}
}

func TestMigrateAllSkipsExisting(t *testing.T) {
	src := &mockMigrateSrc{data: map[string][]byte{"a": []byte("NEW")}, getErr: map[string]error{}}
	dst := &mockMigrateDst{data: map[string][]byte{"a": []byte("OLD")}, putErr: map[string]error{}}

	res := migrateAll(context.Background(), src, dst, []string{"a"}, false, false)
	if res.Skipped != 1 || res.Success != 0 {
		t.Fatalf("res = %+v", res)
	}
	if string(dst.data["a"]) != "OLD" {
		t.Fatalf("跳过时不应覆盖, got %q", dst.data["a"])
	}
}

func TestMigrateAllForceOverwrites(t *testing.T) {
	src := &mockMigrateSrc{data: map[string][]byte{"a": []byte("NEW")}, getErr: map[string]error{}}
	dst := &mockMigrateDst{data: map[string][]byte{"a": []byte("OLD")}, putErr: map[string]error{}}

	res := migrateAll(context.Background(), src, dst, []string{"a"}, true, false)
	if res.Success != 1 {
		t.Fatalf("res = %+v", res)
	}
	if string(dst.data["a"]) != "NEW" {
		t.Fatalf("force 应覆盖, got %q", dst.data["a"])
	}
}

func TestMigrateAllContinuesOnFailure(t *testing.T) {
	src := &mockMigrateSrc{
		data:   map[string][]byte{"ok": []byte("X"), "bad": []byte("Y")},
		getErr: map[string]error{"bad": errors.New("源文件丢失")},
	}
	dst := &mockMigrateDst{data: map[string][]byte{}, putErr: map[string]error{}}

	res := migrateAll(context.Background(), src, dst, []string{"ok", "bad"}, false, false)
	if res.Success != 1 || len(res.Failed) != 1 {
		t.Fatalf("res = %+v", res)
	}
	if res.Failed[0].Key != "bad" {
		t.Fatalf("失败 key = %q", res.Failed[0].Key)
	}
}

func TestMigrateAllVerifyMismatch(t *testing.T) {
	src := &mockMigrateSrc{data: map[string][]byte{"a": []byte("AAA")}, getErr: map[string]error{}}
	dst := &mockMigrateDst{data: map[string][]byte{}, putErr: map[string]error{}}

	res := migrateAll(context.Background(), src, dst, []string{"a"}, false, true)
	if res.Success != 1 || len(res.Failed) != 0 {
		t.Fatalf("verify 一致应成功: %+v", res)
	}
}

func TestMigrateAllVerifyMismatchFails(t *testing.T) {
	src := &mockMigrateSrc{data: map[string][]byte{"a": []byte("AAA")}, getErr: map[string]error{}}
	dst := &mockMigrateDst{
		data:     map[string][]byte{},
		putErr:   map[string]error{},
		headSize: map[string]int64{"a": 999}, // 模拟远端大小与本地不一致
	}

	res := migrateAll(context.Background(), src, dst, []string{"a"}, false, true)
	if len(res.Failed) != 1 || res.Success != 0 {
		t.Fatalf("verify 不一致应记失败: %+v", res)
	}
	if res.Failed[0].Key != "a" {
		t.Fatalf("失败 key = %q", res.Failed[0].Key)
	}
}
