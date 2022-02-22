package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"io"
	"os"

	"github.com/schollz/progressbar/v3"
	"github.com/yumland/bnrom/chips"
	"github.com/yumland/bnrom/paletted"
	"github.com/yumland/gbarom"
	"github.com/yumland/pngchunks"
	"golang.org/x/sync/errgroup"
)

func dumpChips(r io.ReadSeeker, chipsOutFn string, iconsOutFn string) error {
	romID, err := gbarom.ReadROMID(r)
	if err != nil {
		return err
	}

	romTitle, err := gbarom.ReadROMTitle(r)
	if err != nil {
		return err
	}

	info := chips.FindROMInfo(romID)
	if info == nil {
		return errors.New("unsupported game")
	}

	if _, err := r.Seek(info.Offset, os.SEEK_SET); err != nil {
		return err
	}

	chipInfos := make([]chips.ChipInfo, info.Count)

	bar1 := progressbar.Default(int64(info.Count))
	bar1.Describe("decode")
	for i := 0; i < len(chipInfos); i++ {
		bar1.Add(1)
		bar1.Describe(fmt.Sprintf("decode: %04d", i))
		ci, err := chips.ReadChipInfo(r)
		if err != nil {
			return err
		}
		chipInfos[i] = ci
	}

	iconPalette, err := chips.ReadChipIconPalette(r, *info)
	if err != nil {
		return err
	}

	ereaderGigaPalette := chips.EReaderGigaPalette(romTitle)

	bar2 := progressbar.Default(int64(len(chipInfos)))
	bar2.Describe("dump")

	numRows := (len(chipInfos) + 10 - 1) / 10

	img := image.NewRGBA(image.Rect(0, 0, chips.Width*10, chips.Height*numRows))
	iconsImg := image.NewPaletted(image.Rect(0, 0, chips.IconWidth*10, chips.IconHeight*numRows), iconPalette)

	for i, ci := range chipInfos {
		bar2.Add(1)
		bar2.Describe(fmt.Sprintf("dump: %04d", i))

		chipIconImg, err := chips.ReadChipIcon(r, ci)
		if err != nil {
			return err
		}

		x := i % 10
		y := i / 10

		paletted.DrawOver(iconsImg, image.Rect(x*chips.IconWidth, y*chips.IconHeight, (x+1)*chips.IconWidth, (y+1)*chips.IconHeight), chipIconImg, image.Point{})

		chipImg, err := chips.ReadChipImage(r, ci, ereaderGigaPalette)
		if err != nil {
			return err
		}

		draw.Draw(img, image.Rect(x*chips.Width, y*chips.Height, (x+1)*chips.Width, (y+1)*chips.Height), chipImg, image.Point{}, draw.Over)
	}

	if err := func() error {
		f, err := os.Create(chipsOutFn)
		if err != nil {
			return err
		}
		defer f.Close()

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
				metaWritten = true
				// Pack metadata in here.
				{
					var buf bytes.Buffer
					buf.WriteString("fctrl")
					buf.WriteByte('\x00')
					buf.WriteByte('\xff')
					for i := 0; i < len(chipInfos); i++ {
						x := i % 10
						y := i / 10

						binary.Write(&buf, binary.LittleEndian, fctrlFrameInfo{
							int16(x * chips.Width),
							int16(y * chips.Height),
							int16((x + 1) * chips.Width),
							int16((y + 1) * chips.Height),
							int16(0),
							int16(0),
							uint8(1),
							2,
						})
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
	}(); err != nil {
		return err
	}

	if err := func() error {
		f, err := os.Create(iconsOutFn)
		if err != nil {
			return err
		}
		defer f.Close()

		pipeR, pipeW := io.Pipe()

		var g errgroup.Group

		g.Go(func() error {
			defer pipeW.Close()
			if err := png.Encode(pipeW, iconsImg); err != nil {
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
				metaWritten = true
				// Pack metadata in here.
				{
					var buf bytes.Buffer
					buf.WriteString("fctrl")
					buf.WriteByte('\x00')
					buf.WriteByte('\xff')
					for i := 0; i < len(chipInfos); i++ {
						x := i % 10
						y := i / 10

						binary.Write(&buf, binary.LittleEndian, fctrlFrameInfo{
							int16(x * chips.IconWidth),
							int16(y * chips.IconHeight),
							int16((x + 1) * chips.IconWidth),
							int16((y + 1) * chips.IconHeight),
							int16(0),
							int16(0),
							uint8(1),
							2,
						})
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
	}(); err != nil {
		return err
	}

	return nil
}
