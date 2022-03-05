package chips

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"io"
	"log"
	"os"

	"github.com/murkland/bnrom/paletted"
	"github.com/murkland/bnrom/sprites"
	"github.com/murkland/gbarom/bgr555"
)

type ROMInfo struct {
	Offset        int64
	Count         int
	IconPalOffset int64
}

func mustDecodePalette(raw []uint8) color.Palette {
	r := bytes.NewBuffer(raw)

	var palette color.Palette
	for i := 0; i < 16; i++ {
		var c uint16
		if err := binary.Read(r, binary.LittleEndian, &c); err != nil {
			panic(err)
		}

		palette = append(palette, bgr555.ToRGBA(c))
	}

	return palette
}

var falzarGigaPalette = mustDecodePalette([]uint8{0x00, 0x00, 0xDE, 0x7B, 0x74, 0x77, 0xCC, 0x49, 0xEB, 0x44, 0x07, 0x77, 0x43, 0x6E, 0x82, 0x55, 0xC1, 0x40, 0x3E, 0x1B, 0x59, 0x26, 0xB4, 0x09, 0xDD, 0x76, 0x7B, 0x55, 0x39, 0x20, 0x05, 0x18})
var gregarGigaPalette = mustDecodePalette([]uint8{0x00, 0x00, 0xFF, 0x77, 0x9E, 0x47, 0x3F, 0x1F, 0x7D, 0x0A, 0x77, 0x0D, 0xF4, 0x04, 0x51, 0x00, 0x89, 0x10, 0xA3, 0x18, 0x5F, 0x4D, 0x87, 0x37, 0x90, 0x7F, 0xCC, 0x5A, 0x09, 0x36, 0x26, 0x21})
var dblBeastPalette = mustDecodePalette([]uint8{0x7F, 0x7D, 0x9F, 0x13, 0x5E, 0x22, 0x1F, 0x0D, 0xB1, 0x00, 0xD0, 0x41, 0x0D, 0x3D, 0x30, 0x13, 0x99, 0x61, 0xFF, 0x77, 0xA8, 0x4E, 0xA8, 0x39, 0x03, 0x21, 0xF9, 0x5A, 0x30, 0x5F, 0x61, 0x0C})

func FindROMInfo(romID string) *ROMInfo {
	switch romID {
	case "BR6E", "BR6P", "BR5E", "BR5P":
		return &ROMInfo{0x00021DD4, 410, 0x000270C4}
	case "BR6J", "BR5J":
		return &ROMInfo{0x000221E8, 410, 0x000274D8}
	}
	return nil
}

func EReaderGigaPalette(romTitle string) color.Palette {
	switch romTitle {
	case "ROCKEXE6_GXX":
		return gregarGigaPalette
	case "ROCKEXE6_RXX":
		return falzarGigaPalette
	}
	return nil
}

type ChipInfo struct {
	ChipCodes           uint32 // 0
	AttackElement       uint8  // 4
	Rarity              uint8  // 5
	ElementIcon         uint8  // 6
	Library             uint8  // 7
	MB                  uint8  // 8
	EffectFlags         uint8  // 9
	Counter             uint8  // a
	AttackFamily        uint8  // b
	AttackSubfamily     uint8  // c
	DarkSoulBehavior    uint8  // d
	Unknown             uint8  // e
	LocksOn             uint8  // f
	AttackParam1        uint8  // 10
	AttackParam2        uint8  // 11
	AttackParam3        uint8  // 12
	AttackParam4        uint8  // 13
	Delay               uint8  // 14
	LibraryNumber       uint8  // 15
	LibraryFlags        uint8  // 16
	LockOnType          uint8  // 17
	AlphaPos            uint16 // 18
	Damage              uint16 // 1a
	IDPos               uint16 // 1c
	BattleChipGateLimit uint8  // 1e
	DarkchipID          uint8  // 1f
	ChipIconPtr         uint32 // 20
	ChipImagePtr        uint32 // 24
	ChipPalettePtr      uint32 // 28
}

func ReadChipInfo(r io.Reader) (ChipInfo, error) {
	var d ChipInfo
	if err := binary.Read(r, binary.LittleEndian, &d); err != nil {
		return ChipInfo{}, err
	}
	return d, nil
}

const Width = 7 * 8
const Height = 6 * 8

func ReadChipImage(r io.ReadSeeker, ci ChipInfo, ereaderGigaPalette color.Palette) (*image.Paletted, error) {
	retOffset, err := r.Seek(0, os.SEEK_CUR)
	if err != nil {
		return nil, fmt.Errorf("%w while remembering offset", err)
	}
	defer func() {
		r.Seek(retOffset, os.SEEK_SET)
	}()

	if _, err := r.Seek(int64(ci.ChipImagePtr & ^uint32(0x08000000)), os.SEEK_SET); err != nil {
		return nil, fmt.Errorf("%w while seeking to chip image pointer", err)
	}

	img := image.NewPaletted(image.Rect(0, 0, Width, Height), nil)
	for j := 0; j < 6; j++ {
		for i := 0; i < 7; i++ {
			tileImg, err := sprites.ReadTile(r)
			if err != nil {
				return nil, fmt.Errorf("%w while reading tile (%d, %d)", err, i, j)
			}

			paletted.DrawOver(img, image.Rect(i*8, j*8, (i+1)*8, (j+1)*8), tileImg, image.Point{})
		}
	}

	var palette color.Palette
	if ci.ChipPalettePtr&0x08000000 == 0x08000000 {
		if _, err := r.Seek(int64(ci.ChipPalettePtr & ^uint32(0x08000000)), os.SEEK_SET); err != nil {
			return nil, fmt.Errorf("%w while seeking to palette pointer", err)
		}
		for i := 0; i < 16; i++ {
			var c uint16
			if err := binary.Read(r, binary.LittleEndian, &c); err != nil {
				return nil, fmt.Errorf("%w while reading palette entry %d", err, i)
			}

			palette = append(palette, bgr555.ToRGBA(c))
		}
	} else {
		if ci.ChipPalettePtr == 0x02000b10 {
			palette = ereaderGigaPalette
		} else if ci.ChipPalettePtr == 0x02000af0 {
			palette = dblBeastPalette
		}
	}

	img.Palette = palette

	return img, nil
}

func ReadChipIconPalette(r io.ReadSeeker, ri ROMInfo) (color.Palette, error) {
	if _, err := r.Seek(int64(ri.IconPalOffset), os.SEEK_SET); err != nil {
		return nil, fmt.Errorf("%w while seeking to chip icon palette pointer", err)
	}

	var iconPalPtr uint32
	if err := binary.Read(r, binary.LittleEndian, &iconPalPtr); err != nil {
		return nil, fmt.Errorf("%w while reading to chip icon palette pointer", err)
	}

	if _, err := r.Seek(int64(iconPalPtr & ^uint32(0x08000000)), os.SEEK_SET); err != nil {
		return nil, fmt.Errorf("%w while seeking to chip icon palette", err)
	}

	var palette color.Palette
	for j := 0; j < 16; j++ {
		var c uint16
		if err := binary.Read(r, binary.LittleEndian, &c); err != nil {
			return nil, fmt.Errorf("%w while reading to chip icon palette", err)
		}

		palette = append(palette, bgr555.ToRGBA(c))
	}

	return palette, nil
}

const IconWidth = 16
const IconHeight = 16

func ReadChipIcon(r io.ReadSeeker, ci ChipInfo) (*image.Paletted, error) {
	retOffset, err := r.Seek(0, os.SEEK_CUR)
	if err != nil {
		return nil, fmt.Errorf("%w while remembering offset", err)
	}
	defer func() {
		r.Seek(retOffset, os.SEEK_SET)
	}()

	if _, err := r.Seek(int64(ci.ChipIconPtr & ^uint32(0x08000000)), os.SEEK_SET); err != nil {
		log.Fatalf("%s", err)
	}

	img := image.NewPaletted(image.Rect(0, 0, IconWidth, IconHeight), nil)
	for j := 0; j < 2; j++ {
		for i := 0; i < 2; i++ {
			tileImg, err := sprites.ReadTile(r)
			if err != nil {
				log.Fatalf("%s", err)
			}

			paletted.DrawOver(img, image.Rect(i*8, j*8, (i+1)*8, (j+1)*8), tileImg, image.Point{})
		}
	}

	return img, nil
}
