package reconciler

import (
	"bytes"
	"testing"

	"github.com/google/go-cmp/cmp"
	"gopkg.in/yaml.v3"
)

func TestUpdate(t *testing.T) {
	m := map[string]interface{}{
		"resources": []interface{}{
			map[string]interface{}{
				"virtual_hosts": []interface{}{
					map[string]interface{}{
						"routes": []interface{}{
							map[string]interface{}{
								"route": map[string]interface{}{
									"weighted_clusters": map[string]interface{}{
										"clusters": []interface{}{
											map[string]interface{}{
												"name":   "foo",
												"weight": 10,
											},
											map[string]interface{}{
												"name":   "bar",
												"weight": 90,
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
	found := find(m, []string{"resources", "*", "virtual_hosts", "*", "routes", "*", "route", "weighted_clusters", "clusters"}, func(m interface{}) {
		find(m, []string{"name=foo"}, func(m interface{}) {
			set(m, "weight", 80)
		})
		find(m, []string{"name=bar"}, func(m interface{}) {
			set(m, "weight", 20)
		})
	})
	if !found {
		t.Errorf("unexpected return value: expected true, got %v", found)
	}
	buf := bytes.Buffer{}
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	err := enc.Encode(m)
	if err != nil {
		panic(err)
	}
	bs := buf.String()
	expected := `resources:
- virtual_hosts:
  - routes:
    - route:
        weighted_clusters:
          clusters:
          - name: foo
            weight: 80
          - name: bar
            weight: 20
`
	diff := cmp.Diff(expected, bs)
	if diff != "" {
		t.Errorf(diff)
	}
}
