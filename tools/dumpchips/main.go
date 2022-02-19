package main

import (
	"bytes"
	"encoding/binary"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"log"
	"os"

	"github.com/yumland/bnrom/paletted"
	"github.com/yumland/bnrom/sprites"
	"github.com/yumland/gbarom"
	"github.com/yumland/gbarom/bgr555"
	"github.com/schollz/progressbar/v3"
)

type romInfo struct {
	Offset        int64
	Count         int
	IconPalOffset int64
}

var falzarGigaPalette = []uint8{0x00, 0x00, 0xDE, 0x7B, 0x74, 0x77, 0xCC, 0x49, 0xEB, 0x44, 0x07, 0x77, 0x43, 0x6E, 0x82, 0x55, 0xC1, 0x40, 0x3E, 0x1B, 0x59, 0x26, 0xB4, 0x09, 0xDD, 0x76, 0x7B, 0x55, 0x39, 0x20, 0x05, 0x18}
var gregarGigaPalette = []uint8{0x00, 0x00, 0xFF, 0x77, 0x9E, 0x47, 0x3F, 0x1F, 0x7D, 0x0A, 0x77, 0x0D, 0xF4, 0x04, 0x51, 0x00, 0x89, 0x10, 0xA3, 0x18, 0x5F, 0x4D, 0x87, 0x37, 0x90, 0x7F, 0xCC, 0x5A, 0x09, 0x36, 0x26, 0x21}
var dblBeastPalette = []uint8{0x7F, 0x7D, 0x9F, 0x13, 0x5E, 0x22, 0x1F, 0x0D, 0xB1, 0x00, 0xD0, 0x41, 0x0D, 0x3D, 0x30, 0x13, 0x99, 0x61, 0xFF, 0x77, 0xA8, 0x4E, 0xA8, 0x39, 0x03, 0x21, 0xF9, 0x5A, 0x30, 0x5F, 0x61, 0x0C}

func findROMInfo(romID string) *romInfo {
	switch romID {
	case "BR6E", "BR6P", "BR5E", "BR5P":
		return &romInfo{0x00021DD4, 410, 0x000270C4}
	case "BR6J", "BR5J":
		return &romInfo{0x00022214, 409, 0x000274D8}
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

func main() {
	flag.Parse()

	f, err := os.Open(flag.Arg(0))
	if err != nil {
		log.Fatalf("%s", err)
	}

	romTitle, err := gbarom.ReadROMTitle(f)
	if err != nil {
		log.Fatalf("%s", err)
	}

	log.Printf("Game title: %s", romTitle)

	romID, err := gbarom.ReadROMID(f)
	if err != nil {
		log.Fatalf("%s", err)
	}

	info := findROMInfo(romID)
	if info == nil {
		log.Fatalf("unknown rom ID: %s", romID)
	}

	if _, err := f.Seek(int64(info.IconPalOffset), os.SEEK_SET); err != nil {
		log.Fatalf("%s", err)
	}

	var iconPalPtr uint32
	if err := binary.Read(f, binary.LittleEndian, &iconPalPtr); err != nil {
		log.Fatalf("%s", err)
	}

	if _, err := f.Seek(int64(iconPalPtr & ^uint32(0x08000000)), os.SEEK_SET); err != nil {
		log.Fatalf("%s", err)
	}

	var iconPalette color.Palette
	for j := 0; j < 16; j++ {
		var c uint16
		if err := binary.Read(f, binary.LittleEndian, &c); err != nil {
			log.Fatalf("%s", err)
		}

		iconPalette = append(iconPalette, bgr555.ToRGBA(c))
	}

	var ereaderGigaPalette []uint8
	switch romTitle {
	case "ROCKEXE6_GXX":
		ereaderGigaPalette = gregarGigaPalette
	case "ROCKEXE6_RXX":
		ereaderGigaPalette = falzarGigaPalette
	}

	if _, err := f.Seek(int64(info.Offset), os.SEEK_SET); err != nil {
		log.Fatalf("%s", err)
	}

	os.Mkdir("chips", 0o700)

	chipInfos := make([]ChipInfo, info.Count)

	bar1 := progressbar.Default(int64(info.Count))
	bar1.Describe("decode")
	for i := 0; i < len(chipInfos); i++ {
		bar1.Add(1)
		bar1.Describe(fmt.Sprintf("decode: %04d", i))
		ci, err := ReadChipInfo(f)
		if err != nil {
			log.Fatalf("%s", err)
		}
		chipInfos[i] = ci
	}

	bar2 := progressbar.Default(int64(len(chipInfos)))
	bar2.Describe("dump")
	for i, ci := range chipInfos {
		bar2.Add(1)
		bar2.Describe(fmt.Sprintf("dump: %04d", i))

		if _, err := f.Seek(int64(ci.ChipIconPtr & ^uint32(0x08000000)), os.SEEK_SET); err != nil {
			log.Fatalf("%s", err)
		}
		chipIconImg := image.NewPaletted(image.Rect(0, 0, 2*8, 2*8), iconPalette)
		for j := 0; j < 2; j++ {
			for i := 0; i < 2; i++ {
				tileImg, err := sprites.ReadTile(f)
				if err != nil {
					log.Fatalf("%s", err)
				}

				paletted.DrawOver(chipIconImg, image.Rect(i*8, j*8, (i+1)*8, (j+1)*8), tileImg, image.Point{})
			}
		}

		if _, err := f.Seek(int64(ci.ChipImagePtr & ^uint32(0x08000000)), os.SEEK_SET); err != nil {
			log.Fatalf("%s", err)
		}
		chipImg := image.NewPaletted(image.Rect(0, 0, 7*8, 6*8), nil)
		for j := 0; j < 6; j++ {
			for i := 0; i < 7; i++ {
				tileImg, err := sprites.ReadTile(f)
				if err != nil {
					log.Fatalf("%s", err)
				}

				paletted.DrawOver(chipImg, image.Rect(i*8, j*8, (i+1)*8, (j+1)*8), tileImg, image.Point{})
			}
		}

		var palette color.Palette
		var palR io.Reader
		if ci.ChipPalettePtr&0x08000000 == 0x08000000 {
			if _, err := f.Seek(int64(ci.ChipPalettePtr & ^uint32(0x08000000)), os.SEEK_SET); err != nil {
				log.Fatalf("%s", err)
			}
			palR = f
		} else {
			var customPal []uint8
			if ci.ChipPalettePtr == 0x02000b10 {
				customPal = ereaderGigaPalette
			} else if ci.ChipPalettePtr == 0x02000af0 {
				customPal = dblBeastPalette
			}
			palR = bytes.NewBuffer(customPal)
		}

		for j := 0; j < 16; j++ {
			var c uint16
			if err := binary.Read(palR, binary.LittleEndian, &c); err != nil {
				log.Fatalf("%s", err)
			}

			palette = append(palette, bgr555.ToRGBA(c))
		}

		chipImg.Palette = palette

		{
			outf, err := os.Create(fmt.Sprintf("chips/%03d.png", i))
			if err != nil {
				log.Fatalf("%s", err)
			}

			if err := png.Encode(outf, chipImg); err != nil {
				log.Fatalf("%s", err)
			}
		}

		{
			outf, err := os.Create(fmt.Sprintf("chips/%03d_icon.png", i))
			if err != nil {
				log.Fatalf("%s", err)
			}

			if err := png.Encode(outf, chipIconImg); err != nil {
				log.Fatalf("%s", err)
			}
		}
	}
}
