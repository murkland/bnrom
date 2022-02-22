package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"

	"github.com/yumland/bnrom/battletiles"
	"github.com/yumland/bnrom/paletted"
	"github.com/yumland/gbarom"
	"github.com/yumland/pngchunks"
	"golang.org/x/sync/errgroup"
)

func dumpBattletiles(r io.ReadSeeker, outFn string) error {
	romID, err := gbarom.ReadROMID(r)
	if err != nil {
		return err
	}

	info := battletiles.FindROMInfo(romID)
	if info == nil {
		return errors.New("unsupported game")
	}

	palbanks, err := battletiles.ReadPalbanks(r, *info)
	if err != nil {
		return err
	}

	redPal, m := battletiles.ConsolidatePalbank(palbanks, battletiles.RedTileByIndex)
	bluePal, _ := battletiles.ConsolidatePalbank(palbanks, battletiles.BlueTileByIndex)

	tiles, err := battletiles.ReadTiles(r, *info)
	if err != nil {
		return err
	}

	img := image.NewPaletted(image.Rect(0, 0, 9*battletiles.Width, 200*battletiles.Height), nil)

	idx := 0
	for j, tileImg := range tiles {
		for _, pIndex := range battletiles.RedTileByIndex[j/3] {
			tileImgCopy := battletiles.ShiftPalette(tileImg, m[pIndex])

			x := (idx % 9) * battletiles.Width
			y := (idx / 9) * battletiles.Height

			paletted.DrawOver(img, image.Rect(x, y, x+battletiles.Width, y+battletiles.Height), tileImgCopy, image.Point{})
			idx++
		}
	}
	img = img.SubImage(paletted.FindTrim(img)).(*image.Paletted)

	img.Palette = redPal
	outf, err := os.Create(outFn)
	if err != nil {
		return err
	}

	pipeR, pipeW := io.Pipe()

	var g errgroup.Group

	g.Go(func() error {
		defer pipeW.Close()
		if err := png.Encode(pipeW, img); err != nil {
			return err
		}
		return nil
	})

	pngr, err := pngchunks.NewReader(pipeR)
	if err != nil {
		return err
	}

	pngw, err := pngchunks.NewWriter(outf)
	if err != nil {
		return err
	}

	var metaWritten bool
	for {
		chunk, err := pngr.NextChunk()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
		}

		if chunk.Type() == "IDAT" && !metaWritten {
			// Pack metadata in here.
			{
				var buf bytes.Buffer
				buf.WriteString("alt")
				buf.WriteByte('\x00')
				buf.WriteByte('\x08')
				for _, c := range bluePal {
					binary.Write(&buf, binary.LittleEndian, c.(color.RGBA))
					buf.WriteByte('\xff')
					buf.WriteByte('\xff')
				}
				if err := pngw.WriteChunk(int32(buf.Len()), "sPLT", bytes.NewBuffer(buf.Bytes())); err != nil {
					return err
				}
			}

			{
				var buf bytes.Buffer
				buf.WriteString("fctrl")
				buf.WriteByte('\x00')
				buf.WriteByte('\xff')
				for tileIdx, fi := range battletiles.FrameInfos {
					action := uint8(0)
					if fi.IsEnd {
						action = 0x01
					}

					x := (tileIdx % 9) * battletiles.Width
					y := (tileIdx / 9) * battletiles.Height

					binary.Write(&buf, binary.LittleEndian, fctrlFrameInfo{
						int16(x),
						int16(y),
						int16(x + battletiles.Width),
						int16(y + battletiles.Height),
						int16(0),
						int16(0),
						uint8(fi.Delay),
						action,
					})

					tileIdx++
				}
				if err := pngw.WriteChunk(int32(buf.Len()), "zTXt", bytes.NewBuffer(buf.Bytes())); err != nil {
					return err
				}
			}
		}

		if err := pngw.WriteChunk(chunk.Length(), chunk.Type(), chunk); err != nil {
			return err
		}

		if err := chunk.Close(); err != nil {
			return err
		}
	}

	if err := g.Wait(); err != nil {
		return err
	}

	return nil
}
