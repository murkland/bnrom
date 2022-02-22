package main

import (
	"flag"
	"log"
	"os"

	"github.com/yumland/gbarom"
)

var (
	outputDir        = flag.String("output_dir", "out", "output directory")
	dumpSpritesF     = flag.Bool("dump_sprites", true, "dump sprites")
	dumpBattletilesF = flag.Bool("dump_battletiles", true, "dump battletiles")
	dumpChipsF       = flag.Bool("dump_chips", true, "dump chips")
)

type fctrlFrameInfo struct {
	Left    int16
	Top     int16
	Right   int16
	Bottom  int16
	OriginX int16
	OriginY int16
	Delay   uint8
	Action  uint8
}

func main() {
	flag.Parse()

	f, err := os.Open(flag.Arg(0))
	if err != nil {
		log.Fatalf("%s", err)
	}
	defer f.Close()

	os.Mkdir(*outputDir, 0o700)

	romTitle, err := gbarom.ReadROMTitle(f)
	if err != nil {
		log.Fatalf("%s", err)
	}

	log.Printf("Game title: %s", romTitle)

	if *dumpSpritesF {
		spritesOutFn := *outputDir + "/sprites"
		log.Printf("Dumping sprites: %s", spritesOutFn)
		if err := dumpSprites(f, spritesOutFn); err != nil {
			log.Fatalf("%s", err)
		}
	}

	if *dumpBattletilesF {
		tilesOutFn := *outputDir + "/battletiles.png"
		log.Printf("Dumping battletiles: %s", tilesOutFn)
		if err := dumpBattletiles(f, tilesOutFn); err != nil {
			log.Fatalf("%s", err)
		}
	}

	if *dumpChipsF {
		chipsOutFn := *outputDir + "/chips.png"
		chipIconsOutFn := *outputDir + "/chipicons.png"
		log.Printf("Dumping chips: %s + %s", chipsOutFn, chipIconsOutFn)
		if err := dumpChips(f, chipsOutFn, chipIconsOutFn); err != nil {
			log.Fatalf("%s", err)
		}
	}

	log.Printf("Done!")
}
