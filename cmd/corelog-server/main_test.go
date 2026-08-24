package main

import "testing"

func TestResolveAddress(t *testing.T) {
	tests := []struct {
		explicit, port, want string
		valid                bool
	}{
		{"", "", "127.0.0.1:19081", true},
		{"", "19123", "127.0.0.1:19123", true},
		{"127.0.0.1:19234", "19123", "127.0.0.1:19234", true},
		{"0.0.0.0:19081", "", "", false},
		{"127.0.0.1:0", "", "", false},
		{"", "not-port", "", false},
	}
	for _, test := range tests {
		got, err := resolveAddress(test.explicit, test.port)
		if test.valid && (err != nil || got != test.want) {
			t.Errorf("resolveAddress(%q,%q)=%q,%v", test.explicit, test.port, got, err)
		}
		if !test.valid && err == nil {
			t.Errorf("resolveAddress(%q,%q) 应失败", test.explicit, test.port)
		}
	}
}
