package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"log"
	"os"

	"github.com/nbarena/bnrom/paletted"
	"github.com/nbarena/bnrom/sprites"
	"github.com/nbarena/gbarom"
	"github.com/nbarena/gbarom/bgr555"
)

type romInfo struct {
	Offset int64
	Count  int
}

func findROMInfo(romID string) *romInfo {
	switch romID {
	case "BR6E", "BR6P", "BR5E", "BR5P":
		return &romInfo{0x00021DD4, 410}
	case "BR6J", "BR5J":
		return &romInfo{0x00022214, 409}
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

	if _, err := f.Seek(int64(info.Offset), os.SEEK_SET); err != nil {
		log.Fatalf("%s", err)
	}

	os.Mkdir("chips", 0o700)

	chipInfos := make([]ChipInfo, info.Count)
	for i := 0; i < len(chipInfos); i++ {
		ci, err := ReadChipInfo(f)
		if err != nil {
			log.Fatalf("%s", err)
		}
		chipInfos[i] = ci
	}

	for i, ci := range chipInfos {
		if ci.ChipPalettePtr&0x08000000 != 0x08000000 {
			continue
		}

		if _, err := f.Seek(int64(ci.ChipImagePtr & ^uint32(0x08000000)), os.SEEK_SET); err != nil {
			log.Fatalf("%s", err)
		}

		img := image.NewPaletted(image.Rect(0, 0, 7*8, 6*8), nil)

		for j := 0; j < 6; j++ {
			for i := 0; i < 7; i++ {
				tileImg, err := sprites.ReadTile(f)
				if err != nil {
					log.Fatalf("%s", err)
				}

				paletted.DrawOver(img, image.Rect(i*8, j*8, (i+1)*8, (j+1)*8), tileImg, image.Point{})
			}
		}

		if _, err := f.Seek(int64(ci.ChipPalettePtr & ^uint32(0x08000000)), os.SEEK_SET); err != nil {
			log.Fatalf("%s", err)
		}

		var palette color.Palette
		for j := 0; j < 16; j++ {
			var c uint16
			if err := binary.Read(f, binary.LittleEndian, &c); err != nil {
				log.Fatalf("%s", err)
			}

			palette = append(palette, bgr555.ToRGBA(c))
		}

		img.Palette = palette

		outf, err := os.Create(fmt.Sprintf("chips/%03d.png", i))
		if err != nil {
			log.Fatalf("%s", err)
		}

		if err := png.Encode(outf, img); err != nil {
			log.Fatalf("%s", err)
		}
	}
}
