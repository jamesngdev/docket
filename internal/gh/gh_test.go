package gh

import "testing"

func TestRollup(t *testing.T) {
	done := func(c string) CheckItem { return CheckItem{Status: "COMPLETED", Conclusion: c} }
	for name, c := range map[string]struct {
		items []CheckItem
		want  string
	}{
		"no checks":       {nil, "pass"},
		"all green":       {[]CheckItem{done("SUCCESS"), done("SKIPPED"), {State: "SUCCESS"}}, "pass"},
		"running":         {[]CheckItem{done("SUCCESS"), {Status: "IN_PROGRESS"}}, "pending"},
		"status pending":  {[]CheckItem{{State: "PENDING"}}, "pending"},
		"failed":          {[]CheckItem{done("FAILURE")}, "fail"},
		"fail beats wait": {[]CheckItem{done("FAILURE"), {Status: "QUEUED"}}, "fail"},
		"wait then fail":  {[]CheckItem{{Status: "QUEUED"}, {State: "ERROR"}}, "fail"},
	} {
		if got := rollup(c.items); got != c.want {
			t.Errorf("%s: got %s, want %s", name, got, c.want)
		}
	}
}
