package battletiles

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"io"
	"log"
	"os"

	"github.com/yumland/gbarom/bgr555"
)

const paletteOffsetPtr = 0x0000C16C

var RedTileByIndex = [][]int{
	{35},
	{35},
	{35},
	{35},
	{36, 2, 3, 4, 5, 6},
	{38, 10, 11, 12, 13, 14, 15},
	{35},
	{37},
	{36},
	{37, 7, 8, 9},
	{37, 7, 8, 9},
	{37, 7, 8, 9},
	{37, 7, 8, 9},
	{35},
}

var BlueTileByIndex = [][]int{
	{39},
	{39},
	{39},
	{39},
	{40, 18, 19, 20, 21, 22},
	{42, 26, 27, 28, 29, 30, 31},
	{39},
	{41},
	{40},
	{41, 23, 24, 25},
	{41, 23, 24, 25},
	{41, 23, 24, 25},
	{41, 23, 24, 25},
	{39},
}

func ReadPalbanks(r io.ReadSeeker) ([]color.Palette, error) {
	if _, err := r.Seek(paletteOffsetPtr, os.SEEK_SET); err != nil {
		return nil, fmt.Errorf("%w while seeking to palette offset pointer", err)
	}

	var palettePtr uint32
	if err := binary.Read(r, binary.LittleEndian, &palettePtr); err != nil {
		return nil, fmt.Errorf("%w while reading to palette offset pointer", err)
	}

	if _, err := r.Seek(int64(palettePtr & ^uint32(0x08000000)), os.SEEK_SET); err != nil {
		return nil, fmt.Errorf("%w while seeking to palette offset", err)
	}

	var palbanks []color.Palette
	for i := 0; i < 45; i++ {
		var raw [16 * 2]byte
		if _, err := io.ReadFull(r, raw[:]); err != nil {
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

	return palbanks, nil
}

func ConsolidatePalbank(palbanks []color.Palette, tilePaletteses [][]int) (color.Palette, map[int]int) {
	var consolidated color.Palette
	m := map[int]int{}
	consolidated = append(consolidated, palbanks[tilePaletteses[0][0]][:7]...)
	for _, tilePalettes := range tilePaletteses {
		for _, paletteIdx := range tilePalettes {
			if _, ok := m[paletteIdx]; !ok {
				m[paletteIdx] = len(consolidated)
				consolidated = append(consolidated, palbanks[paletteIdx][7:]...)
			}
		}
	}
	return consolidated, m
}

func ShiftPalette(img *image.Paletted, offset int) *image.Paletted {
	imgCopy := image.NewPaletted(img.Rect, img.Palette)

	for i, pix := range img.Pix {
		if pix >= 7 {
			pix = pix - 7 + uint8(offset)
		}
		imgCopy.Pix[i] = pix
	}

	return imgCopy
}
