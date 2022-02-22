package main

import (
	"errors"
	"fmt"
	"image/png"
	"io"
	"log"
	"os"

	"github.com/schollz/progressbar/v3"
	"github.com/yumland/bnrom/chips"
	"github.com/yumland/gbarom"
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

	os.Mkdir(chipsOutFn, 0o700)
	os.Mkdir(iconsOutFn, 0o700)

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
	for i, ci := range chipInfos {
		bar2.Add(1)
		bar2.Describe(fmt.Sprintf("dump: %04d", i))

		chipIconImg, err := chips.ReadChipIcon(r, ci)
		if err != nil {
			return err
		}
		chipIconImg.Palette = iconPalette

		chipImg, err := chips.ReadChipImage(r, ci, ereaderGigaPalette)
		if err != nil {
			return err
		}

		{
			outf, err := os.Create(fmt.Sprintf(chipsOutFn+"/%03d.png", i))
			if err != nil {
				log.Fatalf("%s", err)
			}

			if err := png.Encode(outf, chipImg); err != nil {
				log.Fatalf("%s", err)
			}
		}

		{
			outf, err := os.Create(fmt.Sprintf(iconsOutFn+"/%03d.png", i))
			if err != nil {
				log.Fatalf("%s", err)
			}

			if err := png.Encode(outf, chipIconImg); err != nil {
				log.Fatalf("%s", err)
			}
		}
	}
	return nil
}
