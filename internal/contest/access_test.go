package contest

import "testing"

func TestHasAssignAccess(t *testing.T) {
	item := &Contest{Owner: 10, Maintainer: []int64{11}, Assign: []string{"2026", "staff"}}
	tests := []struct {
		name       string
		uid        int64
		groups     []string
		privileged bool
		want       bool
	}{
		{name: "owner", uid: 10, want: true},
		{name: "maintainer", uid: 11, want: true},
		{name: "admin", uid: 99, privileged: true, want: true},
		{name: "matched group", uid: 7, groups: []string{"2026"}, want: true},
		{name: "non member", uid: 7, groups: []string{"2025"}, want: false},
		{name: "anonymous", want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := HasAssignAccess(item, test.uid, test.groups, test.privileged); got != test.want {
				t.Fatalf("HasAssignAccess() = %v, want %v", got, test.want)
			}
		})
	}
	if !HasAssignAccess(&Contest{}, 0, nil, false) {
		t.Fatal("empty assignment should be public")
	}
}
