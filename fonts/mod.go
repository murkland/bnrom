package fonts

type ROMInfo struct {
	EnemyHPOffset int64
}

func FindROMInfo(romID string) *ROMInfo {
	switch romID {
	case "BR6J", "BR5J":
		return &ROMInfo{0x0001DC78}
	}
	return nil
}
