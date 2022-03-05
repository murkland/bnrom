package main

import (
	"flag"
	"log"
	"os"

	"github.com/murkland/gbarom"
)

var (
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

	romTitle, err := gbarom.ReadROMTitle(f)
	if err != nil {
		log.Fatalf("%s", err)
	}

	log.Printf("Game title: %s", romTitle)

	if *dumpSpritesF {
		log.Printf("Dumping sprites...")
		if err := dumpSprites(f, "sprites"); err != nil {
			log.Fatalf("%s", err)
		}
	}

	if *dumpBattletilesF {
		log.Printf("Dumping battletiles...")
		if err := dumpBattletiles(f, "battletiles.png"); err != nil {
			log.Fatalf("%s", err)
		}
	}

	if *dumpChipsF {
		log.Printf("Dumping chips...")
		if err := dumpChips(f, "chips.png", "chipicons.png"); err != nil {
			log.Fatalf("%s", err)
		}
	}

	log.Printf("Done!")
}
