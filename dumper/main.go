package main

import (
	"flag"
	"log"
	"os"

	"github.com/yumland/gbarom"
)

var (
	outputDir = flag.String("output_dir", "out", "output directory")
)

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

	spritesOutFn := *outputDir + "/sprites"
	log.Printf("Dumping sprites: %s", spritesOutFn)
	if err := dumpSheets(f, spritesOutFn); err != nil {
		log.Fatalf("%s", err)
	}

	tilesOutFn := *outputDir + "/tiles.png"
	log.Printf("Dumping tiles: %s", tilesOutFn)
	if err := dumpBattleTiles(f, tilesOutFn); err != nil {
		log.Fatalf("%s", err)
	}

	log.Printf("Done!")
}
