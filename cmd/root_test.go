package cmd

import "testing"

func TestResolveInsecure(t *testing.T) {
	tests := []struct {
		name         string
		insecureFlag bool
		secureFlag   bool
		want         bool
	}{
		{name: "default skip", insecureFlag: true, secureFlag: false, want: true},
		{name: "explicit insecure", insecureFlag: true, secureFlag: false, want: true},
		{name: "secure wins", insecureFlag: true, secureFlag: true, want: false},
		{name: "insecure false", insecureFlag: false, secureFlag: false, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolveInsecure(tt.insecureFlag, tt.secureFlag); got != tt.want {
				t.Fatalf("resolveInsecure(%v, %v) = %v, want %v", tt.insecureFlag, tt.secureFlag, got, tt.want)
			}
		})
	}
}

func TestInsecureFlagDefaultTrue(t *testing.T) {
	flag := rootCmd.PersistentFlags().Lookup("insecure")
	if flag == nil {
		t.Fatal("missing --insecure")
	}
	if flag.DefValue != "true" {
		t.Fatalf("--insecure default = %q, want true", flag.DefValue)
	}
}
