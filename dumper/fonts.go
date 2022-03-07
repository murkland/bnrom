package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"io"
	"os"
	"strconv"

	"github.com/murkland/bnrom/fonts"
	"github.com/murkland/bnrom/fonts/bdf"
	"github.com/murkland/bnrom/sprites"
	"github.com/murkland/gbarom"
)

func dumpFonts(r io.ReadSeeker, outFn string) error {
	romID, err := gbarom.ReadROMID(r)
	if err != nil {
		return err
	}

	romTitle, err := gbarom.ReadROMTitle(r)
	if err != nil {
		return err
	}

	info := fonts.FindROMInfo(romID, romTitle)
	if info == nil {
		return errors.New("unsupported game")
	}

	os.Mkdir(outFn, 0o700)

	if _, err := r.Seek(info.TinyOffset, io.SeekStart); err != nil {
		return fmt.Errorf("%w while seeking to tiny font pointer", err)
	}

	if err := dumpTinyFont(r, outFn+"/tiny.bdf"); err != nil {
		return fmt.Errorf("%w while dumping tiny font", err)
	}

	if _, err := r.Seek(info.TallOffset, io.SeekStart); err != nil {
		return fmt.Errorf("%w while seeking to tall font pointer", err)
	}

	if err := dumpTallFont(r, info.Charmap, outFn+"/tall.bdf"); err != nil {
		return fmt.Errorf("%w while dumping tall font", err)
	}

	if err := dumpTall2Font(r, info, outFn+"/tall2.bdf"); err != nil {
		return fmt.Errorf("%w while dumping tall font", err)
	}

	return nil
}

func dumpTinyFont(r io.ReadSeeker, outFn string) error {
	const n = 10

	var offsets []int64
	for i := 0; i < n; i++ {
		var offset uint32
		if err := binary.Read(r, binary.LittleEndian, &offset); err != nil {
			return fmt.Errorf("%w while reading offset to tiny font", err)
		}
		offsets = append(offsets, int64(offset&^0x08000000))
	}

	glyphs := make([]*image.Paletted, len(offsets))

	for i, offset := range offsets {
		if _, err := r.Seek(offset, io.SeekStart); err != nil {
			return fmt.Errorf("%w while seeking to tiny font pointer", err)
		}

		glyph, err := fonts.ReadGlyph(r, 5)
		if err != nil {
			return fmt.Errorf("%w while reading tiny font", err)
		}

		glyphs[i] = glyph
	}

	outF, err := os.Create(outFn)
	if err != nil {
		return err
	}
	defer outF.Close()

	p := bdf.Properties{
		XLFD:      "-murkland-tiny-medium-r-normal--16-160-75-75-c-80-iso10646-1",
		Size:      16,
		DPI:       image.Point{75, 75},
		BBox:      image.Rect(0, 0, 8, 16),
		Ascent:    12,
		Descent:   2,
		NumGlyphs: len(glyphs),
	}
	if err := bdf.WriteProperties(outF, p); err != nil {
		return fmt.Errorf("%w while writing bdf properties", err)
	}

	for i, glyph := range glyphs {
		if err := bdf.WriteGlyph(outF, p, 8, rune(strconv.Itoa(i)[0]), glyph); err != nil {
			return fmt.Errorf("%w while writing bdf properties", err)
		}
	}

	if err := bdf.WriteTrailer(outF); err != nil {
		return fmt.Errorf("%w while writing bdf trailer", err)
	}

	return nil
}

func dumpTallFont(r io.ReadSeeker, charmap []rune, outFn string) error {
	var offset uint32
	if err := binary.Read(r, binary.LittleEndian, &offset); err != nil {
		return fmt.Errorf("%w while reading offset to tall font", err)
	}

	if _, err := r.Seek(int64(offset&^0x08000000), io.SeekStart); err != nil {
		return fmt.Errorf("%w while seeking to tall font pointer", err)
	}

	outF, err := os.Create(outFn)
	if err != nil {
		return err
	}
	defer outF.Close()

	p := bdf.Properties{
		XLFD:      "-murkland-tall-medium-r-normal--16-160-75-75-c-80-iso10646-1",
		Size:      16,
		DPI:       image.Point{75, 75},
		BBox:      image.Rect(0, 0, 8, 16),
		Ascent:    12,
		Descent:   2,
		NumGlyphs: 448,
	}
	if err := bdf.WriteProperties(outF, p); err != nil {
		return fmt.Errorf("%w while writing bdf properties", err)
	}

	for i := 0; i < p.NumGlyphs; i++ {
		glyph, err := fonts.ReadGlyph(r, 1)
		if err != nil {
			return fmt.Errorf("%w while reading tall font glyph %d", err, i)
		}

		if err := bdf.WriteGlyph(outF, p, 8, charmap[i], glyph); err != nil {
			return fmt.Errorf("%w while writing bdf properties", err)
		}
	}

	if err := bdf.WriteTrailer(outF); err != nil {
		return fmt.Errorf("%w while writing bdf trailer", err)
	}

	return nil
}

func dumpTall2Font(r io.ReadSeeker, info *fonts.ROMInfo, outFn string) error {
	outF, err := os.Create(outFn)
	if err != nil {
		return err
	}
	defer outF.Close()

	p := bdf.Properties{
		XLFD:      "-murkland-tall2-thin-r-normal--16-160-75-75-c-80-iso10646-1",
		Size:      16,
		DPI:       image.Point{75, 75},
		BBox:      image.Rect(0, 0, 16, 12),
		Ascent:    12,
		Descent:   2,
		NumGlyphs: 448,
	}
	if err := bdf.WriteProperties(outF, p); err != nil {
		return fmt.Errorf("%w while writing bdf properties", err)
	}

	if _, err := r.Seek(info.Tall2MetricsOffset, io.SeekStart); err != nil {
		return fmt.Errorf("%w while seeking to tall font pointer", err)
	}

	metrics, err := fonts.ReadMetrics(r, p.NumGlyphs)
	if err != nil {
		return fmt.Errorf("%w while reading metrics properties", err)
	}

	if _, err := r.Seek(info.Tall2Offset+0x60, io.SeekStart); err != nil {
		return fmt.Errorf("%w while seeking to tall font pointer", err)
	}

	for i := 0; i < p.NumGlyphs; i++ {
		var glyph *image.Paletted
		if i > 0 {
			var err error
			glyph, err = sprites.ReadTile(r, image.Rect(0, 0, 16, 12))
			if err != nil {
				return err
			}
		} else {
			glyph = image.NewPaletted(p.BBox, nil)
		}

		// TODO: Find the font metrics.
		if err := bdf.WriteGlyph(outF, p, metrics[i], info.Charmap[i], glyph); err != nil {
			return fmt.Errorf("%w while writing bdf properties", err)
		}
	}

	if err := bdf.WriteTrailer(outF); err != nil {
		return fmt.Errorf("%w while writing bdf trailer", err)
	}

	return nil
}
