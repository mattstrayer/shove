package redis

import "github.com/alicebob/miniredis/v2"

type miniWrap struct{ s *miniredis.Miniredis }

func (m *miniWrap) addr() string { return m.s.Addr() }
func (m *miniWrap) close()       { m.s.Close() }

func startMiniredis() (*miniWrap, error) {
	s, err := miniredis.Run()
	if err != nil {
		return nil, err
	}
	return &miniWrap{s: s}, nil
}
