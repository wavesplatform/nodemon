package analysis

type Alert struct {
	Description string
}

func (s Alert) String() string {
	return s.Description
}
