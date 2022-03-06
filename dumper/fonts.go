package main

import (
	"errors"
	"fmt"
	"image"
	"io"
	"os"
	"strconv"

	"github.com/murkland/bnrom/fonts"
	"github.com/murkland/bnrom/fonts/bdf"
	"github.com/murkland/gbarom"
)

func dumpFonts(r io.ReadSeeker, outFn string) error {
	romID, err := gbarom.ReadROMID(r)
	if err != nil {
		return err
	}

	info := fonts.FindROMInfo(romID)
	if info == nil {
		return errors.New("unsupported game")
	}

	os.Mkdir(outFn, 0o700)

	if _, err := r.Seek(info.EnemyHPOffset, io.SeekStart); err != nil {
		return fmt.Errorf("%w while seeking to enemy hp font pointer", err)
	}

	if err := dumpEnemyHPFont(r, outFn+"/enemyhp.bdf"); err != nil {
		return fmt.Errorf("%w while dumping enemy hp font", err)
	}

	return nil
}

func dumpEnemyHPFont(r io.ReadSeeker, enemyHPOutFn string) error {
	enemyHPGlyphs, err := fonts.ReadEnemyHPGlyphs(r)
	if err != nil {
		return fmt.Errorf("%w while reading enemy hp font", err)
	}

	outF, err := os.Create(enemyHPOutFn)
	if err != nil {
		return err
	}
	defer outF.Close()

	p := bdf.Properties{
		XLFD:      "-murkland-enemyhp-medium-r-normal--16-160-75-75-c-80-iso10646-1",
		Size:      16,
		DPI:       image.Point{75, 75},
		BBox:      image.Rect(0, 0, 8, 16),
		Ascent:    12,
		Descent:   2,
		NumGlyphs: len(enemyHPGlyphs),
	}
	if err := bdf.WriteProperties(outF, p); err != nil {
		return fmt.Errorf("%w while writing bdf properties", err)
	}

	for i, glyph := range enemyHPGlyphs {
		if err := bdf.WriteGlyph(outF, p, rune(strconv.Itoa(i)[0]), glyph); err != nil {
			return fmt.Errorf("%w while writing bdf properties", err)
		}
	}

	if err := bdf.WriteTrailer(outF); err != nil {
		return fmt.Errorf("%w while writing bdf trailer", err)
	}

	return nil
}
