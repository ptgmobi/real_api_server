package set

var Exist = struct{}{}

type Set struct {
	set map[string]struct{}
}

func NewSet() *Set {
	return &Set{
		set: make(map[string]struct{}, 8),
	}
}

func (s *Set) Add(key string) {
	s.set[key] = Exist
}

func (s *Set) Del(key string) {
	delete(s.set, key)
}

func (s *Set) Has(key string) bool {
	_, ok := s.set[key]
	return ok
}

func (s *Set) Size() int {
	return len(s.set)
}

func (s *Set) List() []string {
	res := make([]string, 0, s.Size())
	for key, _ := range s.set {
		res = append(res, key)
	}
	return res
}
