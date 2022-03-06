package fonts

import (
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"io"

	"github.com/murkland/bnrom/sprites"
)

var enemyHPFontPalette = color.Palette{
	color.RGBA{0, 0, 0, 0},
	color.RGBA{0, 0, 0, 255},
}

func ReadEnemyHPGlyphs(r io.ReadSeeker) ([]*image.Paletted, error) {
	var offsets []int64
	for i := 0; i < 10; i++ {
		var offset uint32
		if err := binary.Read(r, binary.LittleEndian, &offset); err != nil {
			return nil, fmt.Errorf("%w while reading offset to tiny font", err)
		}
		offsets = append(offsets, int64(offset&^0x08000000))
	}

	glyphs := make([]*image.Paletted, len(offsets))

	for i, offset := range offsets {
		if _, err := r.Seek(offset, io.SeekStart); err != nil {
			return nil, fmt.Errorf("%w while seeking to tiny font pointer", err)
		}

		glyph := image.NewPaletted(image.Rect(0, 0, 8, 16), enemyHPFontPalette)
		for o := 0; o < 2; o++ {
			tile, err := sprites.ReadTile(r)
			if err != nil {
				return nil, fmt.Errorf("%w while reading tile %d at pointer 0x%08x", err, o, offset)
			}

			for j := 0; j < 8; j++ {
				for i := 0; i < 8; i++ {
					if tile.Pix[j*8+i] == 5 {
						glyph.Pix[(j+o*8)*8+i] = 1
					}
				}
			}
		}

		glyphs[i] = glyph
	}

	return glyphs, nil
}
