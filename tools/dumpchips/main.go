package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
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
		return &romInfo{0x00021DF4, 138}
	case "BR6J", "BR5J":
		return &romInfo{0x00022234, 146}
	}
	return nil
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

	var mysteryPtr uint32
	if err := binary.Read(f, binary.LittleEndian, &mysteryPtr); err != nil {
		log.Fatalf("%s", err)
	}

	_ = mysteryPtr

	var tilesPtr uint32
	if err := binary.Read(f, binary.LittleEndian, &tilesPtr); err != nil {
		log.Fatalf("%s", err)
	}
	tilesPos := int64(tilesPtr & ^uint32(0x08000000))

	var palettePtr uint32
	if err := binary.Read(f, binary.LittleEndian, &palettePtr); err != nil {
		log.Fatalf("%s", err)
	}
	palettePos := int64(palettePtr & ^uint32(0x08000000))

	os.Mkdir("chips", 0o700)

	for i := 0; i < info.Count; i++ {
		if _, err := f.Seek(int64(tilesPos), os.SEEK_SET); err != nil {
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

		tilesPos, err = f.Seek(0, os.SEEK_CUR)
		if err != nil {
			log.Fatalf("%s", err)
		}

		if _, err := f.Seek(int64(palettePos), os.SEEK_SET); err != nil {
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

		palettePos, err = f.Seek(0, os.SEEK_CUR)
		if err != nil {
			log.Fatalf("%s", err)
		}

		outf, err := os.Create(fmt.Sprintf("chips/%03d.png", i))
		if err != nil {
			log.Fatalf("%s", err)
		}

		if err := png.Encode(outf, img); err != nil {
			log.Fatalf("%s", err)
		}
	}
}
