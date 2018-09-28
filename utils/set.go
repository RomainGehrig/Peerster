package utils

type StringSet struct {
	strMap map[string]struct{}
}

func StringSetInitSingleton(val string) *StringSet {
	return StringSetInit([]string{val})
}

func StringSetInit(vals []string) *StringSet {
	s := StringSet{make(map[string]struct{})}
	for _, val := range vals {
		s.Add(val)
	}
	return &s
}

func (s *StringSet) Add(val string) {
	s.strMap[val] = struct{}{}
}

func (s *StringSet) Has(val string) bool {
	_, present := s.strMap[val]
	return present
}

// Adapted from https://stackoverflow.com/a/35810932
func (s *StringSet) Iterate() <-chan string {
	c := make(chan string)
	go func() {
		for k, _ := range s.strMap {
			c <- k
		}
		close(c)
	}()
	return c
}

func (s *StringSet) ToSlice() []string {
	str := make([]string, 0)
	for p := range s.Iterate() {
		str = append(str, p)
	}
	return str
}