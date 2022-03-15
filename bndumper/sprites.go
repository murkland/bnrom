package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io"
	"log"
	"os"
	"runtime"

	"github.com/schollz/progressbar/v3"
	"github.com/murkland/bnrom/paletted"
	"github.com/murkland/bnrom/sprites"
	"github.com/murkland/gbarom"
	"github.com/murkland/pngchunks"
	"golang.org/x/sync/errgroup"
)

func processOneSheet(outFn string, idx int, anims []sprites.Animation) error {
	type frameInfo struct {
		BBox   image.Rectangle
		Origin image.Point
		Delay  int
		Action sprites.FrameAction
	}

	left := 0
	top := 0

	var infos []frameInfo
	var fullPalette color.Palette
	spriteImg := image.NewPaletted(image.Rect(0, 0, 2048, 2048), nil)

	for _, anim := range anims {
		for _, frame := range anim.Frames {
			fullPalette = frame.Palette

			var fi frameInfo
			fi.Delay = int(frame.Delay)
			fi.Action = frame.Action

			img := frame.MakeImage()
			spriteImg.Palette = img.Palette

			trimBbox := paletted.FindTrim(img)

			fi.Origin.X = img.Rect.Dx()/2 - trimBbox.Min.X
			fi.Origin.Y = img.Rect.Dy()/2 - trimBbox.Min.Y

			if left+trimBbox.Dx() > spriteImg.Rect.Dx() {
				left = 0
				top = paletted.FindTrim(spriteImg).Max.Y
				top++
			}

			fi.BBox = image.Rectangle{image.Point{left, top}, image.Point{left + trimBbox.Dx(), top + trimBbox.Dy()}}

			draw.Draw(spriteImg, fi.BBox, img, trimBbox.Min, draw.Over)
			infos = append(infos, fi)

			if trimBbox.Dx() > 0 {
				left += trimBbox.Dx() + 1
			}
		}
	}

	if spriteImg.Palette == nil {
		return nil
	}

	subimg := spriteImg.SubImage(paletted.FindTrim(spriteImg))
	if subimg.Bounds().Dx() == 0 || subimg.Bounds().Dy() == 0 {
		return nil
	}
	f, err := os.Create(fmt.Sprintf("%s/%04d.png", outFn, idx))
	if err != nil {
		return err
	}
	defer f.Close()

	pipeR, pipeW := io.Pipe()

	var g errgroup.Group

	g.Go(func() error {
		defer pipeW.Close()
		if err := png.Encode(pipeW, subimg); err != nil {
			return err
		}
		return nil
	})

	pngr, err := pngchunks.NewReader(pipeR)
	if err != nil {
		return err
	}

	pngw, err := pngchunks.NewWriter(f)
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
			if len(fullPalette) > 256 {
				var buf bytes.Buffer
				buf.WriteString("extra")
				buf.WriteByte('\x00')
				buf.WriteByte('\x08')
				for _, c := range fullPalette[256:] {
					binary.Write(&buf, binary.LittleEndian, c.(color.RGBA))
					buf.WriteByte('\x00')
					buf.WriteByte('\x00')
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
				for _, info := range infos {
					var action uint8
					switch info.Action {
					case sprites.FrameActionNext:
						action = 0
					case sprites.FrameActionLoop:
						action = 1
					case sprites.FrameActionStop:
						action = 2
					}

					binary.Write(&buf, binary.LittleEndian, fctrlFrameInfo{
						int16(info.BBox.Min.X),
						int16(info.BBox.Min.Y),
						int16(info.BBox.Max.X),
						int16(info.BBox.Max.Y),
						int16(info.Origin.X),
						int16(info.Origin.Y),
						uint8(info.Delay),
						action,
					})
				}
				if err := pngw.WriteChunk(int32(buf.Len()), "zTXt", bytes.NewBuffer(buf.Bytes())); err != nil {
					return err
				}

				metaWritten = true
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

func dumpSprites(r io.ReadSeeker, outFn string) error {
	romID, err := gbarom.ReadROMID(r)
	if err != nil {
		return err
	}

	info := sprites.FindROMInfo(romID)
	if info == nil {
		return errors.New("unsupported game")
	}

	if _, err := r.Seek(info.Offset, os.SEEK_SET); err != nil {
		return err
	}

	s := make([][]sprites.Animation, 0, info.Count)

	bar1 := progressbar.Default(int64(info.Count))
	bar1.Describe("decode")
	for i := 0; i < info.Count; i++ {
		bar1.Add(1)
		bar1.Describe(fmt.Sprintf("decode: %04d", i))
		anims, err := sprites.ReadNext(r)
		if err != nil {
			log.Printf("error reading %04d: %s", i, err)
			continue
		}
		s = append(s, anims)
	}

	os.Mkdir(outFn, 0o700)

	bar2 := progressbar.Default(int64(len(s)))
	bar2.Describe("dump")
	type work struct {
		idx   int
		anims []sprites.Animation
	}

	ch := make(chan work, runtime.NumCPU())

	var g errgroup.Group
	for i := 0; i < runtime.NumCPU(); i++ {
		g.Go(func() error {
			for w := range ch {
				bar2.Add(1)
				bar2.Describe(fmt.Sprintf("dump: %04d", w.idx))
				if err := processOneSheet(outFn, w.idx, w.anims); err != nil {
					return err
				}
			}
			return nil
		})
	}

	for spriteIdx, anims := range s {
		ch <- work{spriteIdx, anims}
	}
	close(ch)

	if err := g.Wait(); err != nil {
		return err
	}

	return nil
}
