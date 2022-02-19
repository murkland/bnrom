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
	"github.com/yumland/gbarom/bgr555"
	"github.com/yumland/gbarom/lz77"
)

const paletteOffsetPtr = 0x0000C16C
const tilesOffsetPtr = 0x0000761C

var tilingSymmetrical = []int{
	1, 2, 3, -2, -1,
	4, 5, 5, 5, -4,
	6, 7, 8, -7, -6,
}
var tilingBroken0 = []int{
	1, 2, 3, 4, 5,
	6, 7, 7, 8, 9,
	10, 11, 12, 13, 14,
}
var tilingAsymmetrical = []int{
	1, 2, 3, 4, 5,
	6, 7, 8, 9, 10,
	11, 12, 13, 14, 15,
}
var tilingSymmetricalB = []int{
	1, 2, 3, -2, -1,
	4, 5, 6, -5, -4,
	7, 8, 9, -8, -7,
}

func offsetTiling(tiling []int, offset int) []int {
	t := make([]int, len(tiling))
	for i, x := range tiling {
		if x > 0 {
			t[i] = x + offset
		} else {
			t[i] = x - offset
		}
	}
	return t
}

func flipTiling(tiling []int) []int {
	t := make([]int, len(tiling))
	for i, x := range tiling {
		t[i] = -x
	}
	return t
}

var tileGroups = [][]int{
	// hole
	tilingSymmetrical,
	offsetTiling(tilingSymmetrical, 8),
	offsetTiling(tilingSymmetrical, 16),
	// broken
	offsetTiling(tilingBroken0, 24),
	offsetTiling(tilingAsymmetrical, 38),
	offsetTiling(tilingAsymmetrical, 53),
	// normal
	offsetTiling(tilingSymmetrical, 68),
	offsetTiling(tilingSymmetrical, 76),
	offsetTiling(tilingSymmetrical, 84),
	// cracked
	offsetTiling(tilingAsymmetrical, 92),
	offsetTiling(tilingAsymmetrical, 107),
	offsetTiling(tilingAsymmetrical, 122),
	// poison
	offsetTiling(tilingAsymmetrical, 137),
	offsetTiling(tilingAsymmetrical, 152),
	offsetTiling(tilingAsymmetrical, 167),
	// holy
	offsetTiling(tilingSymmetricalB, 182),
	offsetTiling(tilingSymmetricalB, 191),
	offsetTiling(tilingSymmetricalB, 200),
	// grass
	offsetTiling(tilingAsymmetrical, 209),
	offsetTiling(tilingAsymmetrical, 224),
	offsetTiling(tilingAsymmetrical, 239),
	// ice
	offsetTiling(tilingAsymmetrical, 254),
	offsetTiling(tilingAsymmetrical, 269),
	offsetTiling(tilingAsymmetrical, 284),
	// volcano
	offsetTiling(tilingAsymmetrical, 299),
	offsetTiling(tilingAsymmetrical, 314),
	offsetTiling(tilingAsymmetrical, 329),
	// road up
	offsetTiling(tilingSymmetricalB, 344),
	offsetTiling(tilingSymmetricalB, 353),
	offsetTiling(tilingSymmetricalB, 362),
	// road down
	offsetTiling(tilingSymmetricalB, 371),
	offsetTiling(tilingSymmetricalB, 380),
	offsetTiling(tilingSymmetricalB, 389),
	// road left
	offsetTiling(flipTiling(tilingAsymmetrical), 398),
	offsetTiling(flipTiling(tilingAsymmetrical), 413),
	offsetTiling(flipTiling(tilingAsymmetrical), 428),
	// road right
	offsetTiling(tilingAsymmetrical, 443),
	offsetTiling(tilingAsymmetrical, 458),
	offsetTiling(tilingAsymmetrical, 473),
	// edge
	{491, 492, 493, -492, -491},
}

func main() {
	flag.Parse()

	f, err := os.Open(flag.Arg(0))
	if err != nil {
		log.Fatalf("%s", err)
	}

	os.Mkdir("tiles", 0o700)

	if _, err := f.Seek(paletteOffsetPtr, os.SEEK_SET); err != nil {
		log.Fatalf("%s", err)
	}

	var palettePtr uint32
	if err := binary.Read(f, binary.LittleEndian, &palettePtr); err != nil {
		log.Fatalf("%s", err)
	}

	if _, err := f.Seek(int64(palettePtr & ^uint32(0x08000000)), os.SEEK_SET); err != nil {
		log.Fatalf("%s", err)
	}

	var palbanks []color.Palette
	for i := 0; i < 45; i++ {
		var raw [16 * 2]byte
		if _, err := io.ReadFull(f, raw[:]); err != nil {
			log.Fatalf("%s", err)
		}

		var palette color.Palette
		palR := bytes.NewBuffer(raw[:])
		for j := 0; j < 16; j++ {
			var c uint16
			if err := binary.Read(palR, binary.LittleEndian, &c); err != nil {
				log.Fatalf("%s", err)
			}

			palette = append(palette, bgr555.ToRGBA(c))
		}
		palette[0] = color.RGBA{}

		palbanks = append(palbanks, palette)
	}

	if _, err := f.Seek(tilesOffsetPtr, os.SEEK_SET); err != nil {
		log.Fatalf("%s", err)
	}

	var tilesPtr uint32
	if err := binary.Read(f, binary.LittleEndian, &tilesPtr); err != nil {
		log.Fatalf("%s", err)
	}

	if _, err := f.Seek(int64(tilesPtr & ^uint32(0x08000000)), os.SEEK_SET); err != nil {
		log.Fatalf("%s", err)
	}

	rawTiles, err := lz77.Decompress(f)
	if err != nil {
		log.Fatalf("%s", err)
	}

	img := image.NewPaletted(image.Rect(0, 0, 5*8, len(tileGroups)*3*8), palbanks[0])

	for j, tg := range tileGroups {
		for i, tIndex := range tg {
			flipH := false
			if tIndex < 0 {
				flipH = true
				tIndex = -tIndex
			}
			tIndex--

			tileImg, err := sprites.ReadTile(bytes.NewBuffer(rawTiles[tIndex*8*8/2 : (tIndex+1)*8*8/2]))
			if err != nil {
				log.Fatalf("%s", err)
			}

			tileImg.Palette = img.Palette
			if err != nil {
				log.Fatalf("%s", err)
			}

			if flipH {
				paletted.FlipHorizontal(tileImg)
			}

			x := (i % 5) * 8
			y := (i/5)*8 + j*3*8

			paletted.DrawOver(img, image.Rect(x, y, x+8, y+8), tileImg, image.Point{})
		}
	}

	for i, palbank := range palbanks {
		img.Palette = palbank

		outf, err := os.Create(fmt.Sprintf("tiles/%02d.png", i))
		if err != nil {
			log.Fatalf("%s", err)
		}

		if err := png.Encode(outf, img); err != nil {
			log.Fatalf("%s", err)
		}
	}
}
