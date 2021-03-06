package bdf

import (
	"errors"
	"fmt"
	"image"
	"io"
)

type Properties struct {
	XLFD    string
	Size    int
	DPI     image.Point
	BPP     int
	BBox    image.Rectangle
	Ascent  int
	Descent int

	NumGlyphs int
}

var ErrUnsupportedBPP = errors.New("bdf: unsupported bpp, must be one of 1, 2, 4, 8")

func checkBPP(p Properties) error {
	if 8%p.BPP != 0 {
		return ErrUnsupportedBPP
	}
	return nil
}

func WriteProperties(w io.Writer, p Properties) error {
	if err := checkBPP(p); err != nil {
		return err
	}

	if _, err := fmt.Fprintf(w, "STARTFONT 2.3\n"); err != nil {
		return err
	}

	if _, err := fmt.Fprintf(w, "FONT %s\n", p.XLFD); err != nil {
		return err
	}

	if _, err := fmt.Fprintf(w, "SIZE %d %d %d %d\n", p.Size, p.DPI.X, p.DPI.Y, p.BPP); err != nil {
		return err
	}

	if _, err := fmt.Fprintf(w, "BITS_PER_PIXEL %d\n", p.BPP); err != nil {
		return err
	}

	if _, err := fmt.Fprintf(w, "FONTBOUNDINGBOX %d %d %d %d\n", p.BBox.Dx(), p.BBox.Dy(), p.BBox.Min.X, p.BBox.Min.Y); err != nil {
		return err
	}

	if _, err := fmt.Fprintf(w, "STARTPROPERTIES 2\n"); err != nil {
		return err
	}

	if _, err := fmt.Fprintf(w, "FONT_ASCENT %d\n", p.Ascent); err != nil {
		return err
	}

	if _, err := fmt.Fprintf(w, "FONT_DESCENT %d\n", p.Descent); err != nil {
		return err
	}

	if _, err := fmt.Fprintf(w, "ENDPROPERTIES\n"); err != nil {
		return err
	}

	if _, err := fmt.Fprintf(w, "CHARS %d\n", p.NumGlyphs); err != nil {
		return err
	}

	return nil
}

func WriteGlyph(w io.Writer, p Properties, width int, codepoint rune, img *image.Alpha) error {
	if err := checkBPP(p); err != nil {
		return err
	}

	ppb := 8 / p.BPP

	if _, err := fmt.Fprintf(w, "STARTCHAR U+%04X\n", codepoint); err != nil {
		return err
	}

	if _, err := fmt.Fprintf(w, "ENCODING %d\n", codepoint); err != nil {
		return err
	}

	if _, err := fmt.Fprintf(w, "SWIDTH %d 0\n", width*1000/p.BBox.Dx()); err != nil {
		return err
	}

	if _, err := fmt.Fprintf(w, "DWIDTH %d 0\n", width); err != nil {
		return err
	}

	if _, err := fmt.Fprintf(w, "BBX %d %d %d %d\n", img.Rect.Dx(), img.Rect.Dy(), p.BBox.Min.X, p.BBox.Min.Y); err != nil {
		return err
	}

	if _, err := fmt.Fprintf(w, "BITMAP\n"); err != nil {
		return err
	}

	for j := 0; j < img.Bounds().Dy(); j++ {
		row := img.Pix[j*img.Bounds().Dx() : (j+1)*img.Bounds().Dx()]

		if r := len(row) % ppb; r != 0 {
			row = append(row, make([]uint8, ppb-r)...)
		}

		for j := 0; j < len(row); j += ppb {
			var mask uint8
			for i, b := range row[j : j+ppb] {
				mask |= uint8((uint32(b) * ((1 << p.BPP) - 1) / 0xff)) << ((ppb - i - 1) * p.BPP)
			}
			fmt.Fprintf(w, "%02X", mask)
		}
		fmt.Fprintf(w, "\n")
	}

	if _, err := fmt.Fprintf(w, "ENDCHAR\n"); err != nil {
		return err
	}

	return nil
}

func WriteTrailer(w io.Writer) error {
	if _, err := fmt.Fprintf(w, "ENDFONT\n"); err != nil {
		return err
	}

	return nil
}
