package core

import "testing"

func TestBackupAutoFileNameValid(t *testing.T) {
	valid := []string{
		"sillygirl-auto-20260913-040000.zip",
		backupAutoFilePrefix + "x.zip",
	}
	for _, name := range valid {
		if !backupAutoFileNameValid(name) {
			t.Errorf("backupAutoFileNameValid(%q) = false; want true", name)
		}
	}
	invalid := []string{
		"",
		"other-backup.zip",
		"sillygirl-auto-x.tar.gz",
		"../sillygirl-auto-20260913.zip",
		"sillygirl-auto-a/../../etc.zip",
	}
	for _, name := range invalid {
		if backupAutoFileNameValid(name) {
			t.Errorf("backupAutoFileNameValid(%q) = true; want false", name)
		}
	}
}

func TestBackupAutoSettingDefaults(t *testing.T) {
	if backupAutoEnabled() {
		t.Error("auto backup should default to disabled")
	}
	if hour := backupAutoHour(); hour < 0 || hour > 23 {
		t.Errorf("backupAutoHour() = %d; out of range", hour)
	}
	if keep := backupAutoKeep(); keep < 1 {
		t.Errorf("backupAutoKeep() = %d; want >= 1", keep)
	}
}
