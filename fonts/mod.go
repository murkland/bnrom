package fonts

type ROMInfo struct {
	EnemyHPOffset int64
}

func FindROMInfo(romID string) *ROMInfo {
	switch romID {
	case "BR6E", "BR6P", "BR5E", "BR5P":
		return &ROMInfo{0x0001D854}
	case "BR6J", "BR5J":
		return &ROMInfo{0x0001DC78}
	}
	return nil
}
