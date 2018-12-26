package set

import (
	"testing"
)

func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

func TestSet(t *testing.T) {
	s := NewSet()
	if s.Size() != 0 {
		t.Error("set s should have 0 element.")
	}

	s.Add("hello")
	if s.Size() != 1 || !s.Has("hello") {
		t.Error("set s should only have a element: hello.")
	}

	s.Add("world")
	items := s.List()
	if len(items) != 2 || !contains(items, "hello") || !contains(items, "hello") {
		t.Error("list of set s should have a length of 2")
	}

	s.Del("hello")
	if s.Size() != 1 || !s.Has("world") {
		t.Error("set s should only have a element: world.")
	}
}
