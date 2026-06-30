// Copyright (c) the go-ruby-sinatra/sinatra authors
//
// SPDX-License-Identifier: BSD-3-Clause

package sinatra

import (
	"reflect"
	"testing"
)

func TestSettingsSetGet(t *testing.T) {
	s := New().Settings()
	s.Set("port", 4567)
	if v, ok := s.Get("port"); !ok || v.(int) != 4567 {
		t.Errorf("port = %v %v", v, ok)
	}
	if _, ok := s.Get("missing"); ok {
		t.Error("missing reported present")
	}
}

func TestSettingsEnableDisableBool(t *testing.T) {
	s := New().Settings()
	s.Enable("logging")
	if !s.Bool("logging") {
		t.Error("enable did not set true")
	}
	s.Disable("logging")
	if s.Bool("logging") {
		t.Error("disable did not set false")
	}
	// Unset and non-bool keys are false.
	if s.Bool("never_set") {
		t.Error("unset Bool should be false")
	}
	s.Set("count", 3)
	if s.Bool("count") {
		t.Error("non-bool Bool should be false")
	}
}

func TestSettingsString(t *testing.T) {
	s := New().Settings()
	if s.String("default_content_type") != "text/html" {
		t.Errorf("default_content_type = %q", s.String("default_content_type"))
	}
	if s.String("never_set") != "" {
		t.Error("unset String should be empty")
	}
	s.Set("num", 7)
	if s.String("num") != "" {
		t.Error("non-string String should be empty")
	}
}

func TestSettingsKeysOrdered(t *testing.T) {
	s := newSettings()
	s.Set("a", 1)
	s.Set("b", 2)
	s.Set("a", 3) // re-set keeps position
	keys := s.Keys()
	want := []string{"default_content_type", "default_encoding", "a", "b"}
	if !reflect.DeepEqual(keys, want) {
		t.Errorf("keys = %v want %v", keys, want)
	}
}

func TestAppSettingsAccessor(t *testing.T) {
	s := New()
	if s.Settings() == nil {
		t.Error("nil settings")
	}
	s.Settings().Set("foo", "bar")
	if s.Settings().String("foo") != "bar" {
		t.Error("settings not shared")
	}
}

func TestParamsHasAndKeys(t *testing.T) {
	p := newOrderedParams()
	p.set("a", "1")
	p.set("b", "2")
	if !p.has("a") || p.has("z") {
		t.Error("has wrong")
	}
	if got := p.keysInOrder(); !reflect.DeepEqual(got, []string{"a", "b"}) {
		t.Errorf("keys = %v", got)
	}
	// mergeFrom with a nil source is a no-op.
	p.mergeFrom(nil)
	if p.Len() != 2 {
		t.Errorf("len after nil merge = %d", p.Len())
	}
}

func TestMergeFromNilKeepsExisting(t *testing.T) {
	base := newOrderedParams()
	base.set("id", "query-id")
	base.set("only-query", "q")
	other := newOrderedParams()
	other.set("id", nil)      // route capture absent -> keep base
	other.set("new", "fresh") // a real new value
	other.set("absent", nil)  // not in base -> added as nil
	base.mergeFrom(other)
	if v, _ := base.get("id"); v != "query-id" {
		t.Errorf("id = %v want query-id (v2||v1)", v)
	}
	if v, _ := base.get("new"); v != "fresh" {
		t.Errorf("new = %v", v)
	}
	if v, ok := base.get("absent"); !ok || v != nil {
		t.Errorf("absent = %v %v want present nil", v, ok)
	}
}

func TestParamsLen(t *testing.T) {
	p := newOrderedParams()
	if p.Len() != 0 {
		t.Errorf("empty len = %d", p.Len())
	}
}
