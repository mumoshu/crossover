package reconciler

import "strings"

func set(m interface{}, k string, v interface{}) {
	mm, ok := m.(map[string]interface{})
	if ok {
		mm[k] = v
	}
}

func find(m interface{}, path []string, f func(interface{})) bool {
	if len(path) == 0 {
		f(m)
		return true
	}
	switch t := m.(type) {
	case []interface{}:
		next := path[0]
		if next == "*" {
			any := false
			for _, v := range t {
				r := find(v, path[1:], f)
				any = any || r
			}
			return any
		} else {
			cond := strings.Split(next, "=")
			left := cond[0]
			right := cond[1]
			for _, v := range t {
				m, ok := v.(map[string]interface{})
				if ok {
					if m[left] == right {
						return find(m, path[1:], f)
					}
				}
			}

		}
	case map[string]interface{}:
		return find(t[path[0]], path[1:], f)
	}
	return false
}
