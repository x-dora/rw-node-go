package system

type Stats struct {
	Info      map[string]any
	Stats     map[string]any
	Interface map[string]any
}

func EmptyStats() Stats {
	return Stats{
		Info:      map[string]any{},
		Stats:     map[string]any{},
		Interface: map[string]any{},
	}
}
