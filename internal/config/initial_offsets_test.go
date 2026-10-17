package config

import "testing"

func TestInitialOffsetModes(t *testing.T) {
	t.Setenv("INITIAL_OFFSET_MODE", "")
	t.Setenv("INITIAL_OFFSET_ROOTS", "")
	c, err := Load()
	if err != nil || c.InitialOffsetMode != "beginning" {
		t.Fatalf("default: %+v %v", c, err)
	}
	for _, tc := range []struct {
		mode, roots string
		valid       bool
	}{
		{"tail", "/mnt/prod/srvinfo", true}, {"tail", "", false}, {"tail", "/mnt/dev", false},
		{"tail", "/mnt/prod/../srvinfo", false}, {"typo", "", false}, {"beginning", "/mnt/prod/srvinfo", false},
	} {
		t.Run(tc.mode+tc.roots, func(t *testing.T) {
			t.Setenv("INITIAL_OFFSET_MODE", tc.mode)
			t.Setenv("INITIAL_OFFSET_ROOTS", tc.roots)
			t.Setenv("LOG_DIRS", "/mnt/srvinfo;/mnt/prod/srvinfo")
			cfg, err := Load()
			if (err == nil) != tc.valid {
				t.Fatalf("unexpected validation: %v", err)
			}
			if err == nil && (!cfg.TailForPath("/mnt/prod/srvinfo/reg_1541/base/1Cv8Log") || cfg.TailForPath("/mnt/srvinfo/reg_1541/base/1Cv8Log") || cfg.TailForPath("/mnt/prod/srvinfo-other")) {
				t.Fatal("source isolation")
			}
		})
	}
}
