package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"log"
	"os"

	"github.com/yumland/bnrom/battletiles"
	"github.com/yumland/bnrom/paletted"
	"github.com/yumland/pngchunks"
	"golang.org/x/sync/errgroup"
)

const (
	poisonFrameTime = 16
	holyFrameTime   = 10
	roadFrameTime   = 8
)

var frameTimes = []int{
	1,
	1,
	1,
	1,
	poisonFrameTime,
	holyFrameTime,
	1,
	1,
	1,
	roadFrameTime,
	roadFrameTime,
	roadFrameTime,
	roadFrameTime,
	1,
}

const tileWidth = 5 * 8
const tileHeight = 3 * 8

func main() {
	flag.Parse()

	f, err := os.Open(flag.Arg(0))
	if err != nil {
		log.Fatalf("%s", err)
	}

	os.Mkdir("tiles", 0o700)

	palbanks, err := battletiles.ReadPalbanks(f)
	if err != nil {
		log.Fatalf("%s", err)
	}

	redPal, m := battletiles.ConsolidatePalbank(palbanks, battletiles.RedTileByIndex)
	bluePal, _ := battletiles.ConsolidatePalbank(palbanks, battletiles.BlueTileByIndex)

	tiles, err := battletiles.ReadTiles(f)
	if err != nil {
		log.Fatalf("%s", err)
	}

	img := image.NewPaletted(image.Rect(0, 0, 9*tileWidth, 200*tileHeight), nil)

	idx := 0
	for j, tileImg := range tiles {
		for _, pIndex := range battletiles.RedTileByIndex[j/3] {
			tileImgCopy := battletiles.ShiftPalette(tileImg, m[pIndex])

			x := (idx % 9) * tileWidth
			y := (idx / 9) * tileHeight

			paletted.DrawOver(img, image.Rect(x, y, x+tileWidth, y+tileHeight), tileImgCopy, image.Point{})
			idx++
		}
	}
	img = img.SubImage(paletted.FindTrim(img)).(*image.Paletted)

	img.Palette = redPal
	outf, err := os.Create(fmt.Sprintf("tiles.png"))
	if err != nil {
		log.Fatalf("%s", err)
	}

	r, w := io.Pipe()

	var g errgroup.Group

	g.Go(func() error {
		defer w.Close()
		if err := png.Encode(w, img); err != nil {
			return err
		}
		return nil
	})

	pngr, err := pngchunks.NewReader(r)
	if err != nil {
		log.Fatalf("%s", err)
	}

	pngw, err := pngchunks.NewWriter(outf)
	if err != nil {
		log.Fatalf("%s", err)
	}

	for {
		chunk, err := pngr.NextChunk()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
		}

		if err := pngw.WriteChunk(chunk.Length(), chunk.Type(), chunk); err != nil {
			log.Fatalf("%s", err)
		}

		if chunk.Type() == "tRNS" {
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
					log.Fatalf("%s", err)
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

					x := (tileIdx % 9) * tileWidth
					y := (tileIdx / 9) * tileHeight

					binary.Write(&buf, binary.LittleEndian, struct {
						Left    int16
						Top     int16
						Right   int16
						Bottom  int16
						OriginX int16
						OriginY int16
						Delay   uint8
						Action  uint8
					}{
						int16(x),
						int16(y),
						int16(x + tileWidth),
						int16(y + tileHeight),
						int16(0),
						int16(0),
						uint8(fi.Delay),
						action,
					})

					tileIdx++
				}
				if err := pngw.WriteChunk(int32(buf.Len()), "zTXt", bytes.NewBuffer(buf.Bytes())); err != nil {
					log.Fatalf("%s", err)
				}
			}
		}

		if err := chunk.Close(); err != nil {
			log.Fatalf("%s", err)
		}
	}

	if err := g.Wait(); err != nil {
		log.Fatalf("%s", err)
	}

}
