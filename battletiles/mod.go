package battletiles

type ROMInfo struct {
	TilesOffset int64
	PalOffset   int64
}

func FindROMInfo(romID string) *ROMInfo {
	switch romID {
	case "BR6E", "BR6P", "BR5E", "BR5P":
		return &ROMInfo{0x0000761C, 0x0000C16C}
	case "BR6J", "BR5J":
		return &ROMInfo{0x00007610, 0x0000C788}
	}
	return nil
}
