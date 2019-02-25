package testing

import (
	"k8s.io/client-go/tools/cache"
)

var _ cache.Store = (*MockStore)(nil)

type MockStore struct {
	m  map[string]interface{}
	kf cache.KeyFunc
}

func NewMockStore(m map[string]interface{}, keyFunc cache.KeyFunc) cache.Store {
	return &MockStore{
		m:  m,
		kf: keyFunc,
	}
}

func (s *MockStore) Add(obj interface{}) error {
	key, err := s.kf(obj)
	if err != nil {
		return err
	}
	s.m[key] = obj
	return nil
}
func (s *MockStore) Update(obj interface{}) error {
	return s.Add(obj)
}
func (s *MockStore) Delete(obj interface{}) error {
	key, err := s.kf(obj)
	if err != nil {
		return err
	}
	delete(s.m, key)
	return nil
}
func (s *MockStore) List() []interface{} {
	values := make([]interface{}, 0, len(s.m))
	for _, v := range s.m {
		values = append(values, v)
	}
	return values
}
func (s *MockStore) ListKeys() []string {
	keys := make([]string, 0, len(s.m))
	for k := range s.m {
		keys = append(keys, k)
	}
	return keys
}
func (s *MockStore) Get(obj interface{}) (item interface{}, exists bool, err error) {
	key, err := s.kf(obj)
	if err != nil {
		return nil, false, err
	}
	return s.GetByKey(key)
}
func (s *MockStore) GetByKey(key string) (item interface{}, exists bool, err error) {
	item, exists = s.m[key]
	err = nil
	return
}
func (s *MockStore) Replace([]interface{}, string) error {
	// No op. If needed in test then implement it or just create a new map
	return nil
}
func (s *MockStore) Resync() error {
	// No Op
	return nil
}
