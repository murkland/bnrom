package battletiles

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"io"
	"os"

	"github.com/yumland/bnrom/paletted"
	"github.com/yumland/bnrom/sprites"
	"github.com/yumland/gbarom/lz77"
)

const tilesOffsetPtr = 0x0000761C

var tilingSymmetrical = []int{
	1, 2, 3, -2, -1,
	4, 5, 5, 5, -4,
	6, 7, 8, -7, -6,
}
var tilingBroken0 = []int{
	1, 2, 3, 4, 5,
	6, 7, 7, 8, 9,
	10, 11, 12, 13, 14,
}
var tilingAsymmetrical = []int{
	1, 2, 3, 4, 5,
	6, 7, 8, 9, 10,
	11, 12, 13, 14, 15,
}
var tilingSymmetricalB = []int{
	1, 2, 3, -2, -1,
	4, 5, 6, -5, -4,
	7, 8, 9, -8, -7,
}

func offsetTiling(tiling []int, offset int) []int {
	t := make([]int, len(tiling))
	for i, x := range tiling {
		if x > 0 {
			t[i] = x + offset
		} else {
			t[i] = x - offset
		}
	}
	return t
}

func flipTiling(tiling []int) []int {
	t := make([]int, len(tiling))
	for i, x := range tiling {
		t[i] = -x
	}
	return t
}

var tileGroups = [][]int{
	// hole
	tilingSymmetrical,
	offsetTiling(tilingSymmetrical, 8),
	offsetTiling(tilingSymmetrical, 16),
	// broken
	offsetTiling(tilingBroken0, 24),
	offsetTiling(tilingAsymmetrical, 38),
	offsetTiling(tilingAsymmetrical, 53),
	// normal
	offsetTiling(tilingSymmetrical, 68),
	offsetTiling(tilingSymmetrical, 76),
	offsetTiling(tilingSymmetrical, 84),
	// cracked
	offsetTiling(tilingAsymmetrical, 92),
	offsetTiling(tilingAsymmetrical, 107),
	offsetTiling(tilingAsymmetrical, 122),
	// poison
	offsetTiling(tilingAsymmetrical, 137),
	offsetTiling(tilingAsymmetrical, 152),
	offsetTiling(tilingAsymmetrical, 167),
	// holy
	offsetTiling(tilingSymmetricalB, 182),
	offsetTiling(tilingSymmetricalB, 191),
	offsetTiling(tilingSymmetricalB, 200),
	// grass
	offsetTiling(tilingAsymmetrical, 209),
	offsetTiling(tilingAsymmetrical, 224),
	offsetTiling(tilingAsymmetrical, 239),
	// ice
	offsetTiling(tilingAsymmetrical, 254),
	offsetTiling(tilingAsymmetrical, 269),
	offsetTiling(tilingAsymmetrical, 284),
	// volcano
	offsetTiling(tilingAsymmetrical, 299),
	offsetTiling(tilingAsymmetrical, 314),
	offsetTiling(tilingAsymmetrical, 329),
	// road up
	offsetTiling(tilingSymmetricalB, 344),
	offsetTiling(tilingSymmetricalB, 353),
	offsetTiling(tilingSymmetricalB, 362),
	// road down
	offsetTiling(tilingSymmetricalB, 371),
	offsetTiling(tilingSymmetricalB, 380),
	offsetTiling(tilingSymmetricalB, 389),
	// road left
	offsetTiling(flipTiling(tilingAsymmetrical), 398),
	offsetTiling(flipTiling(tilingAsymmetrical), 413),
	offsetTiling(flipTiling(tilingAsymmetrical), 428),
	// road right
	offsetTiling(tilingAsymmetrical, 443),
	offsetTiling(tilingAsymmetrical, 458),
	offsetTiling(tilingAsymmetrical, 473),
	// edge
	{491, 492, 493, -492, -491},
}

const Width = 5 * 8
const Height = 3 * 8

func ReadTiles(r io.ReadSeeker, ri ROMInfo) ([]*image.Paletted, error) {
	if _, err := r.Seek(ri.TilesOffset, os.SEEK_SET); err != nil {
		return nil, fmt.Errorf("%w while seeking to tile offset pointer", err)
	}

	var tilesPtr uint32
	if err := binary.Read(r, binary.LittleEndian, &tilesPtr); err != nil {
		return nil, fmt.Errorf("%w while reading tile offset", err)
	}

	if _, err := r.Seek(int64(tilesPtr & ^uint32(0x08000000)), os.SEEK_SET); err != nil {
		return nil, fmt.Errorf("%w while reading tile offset", err)
	}

	rawTiles, err := lz77.Decompress(r)
	if err != nil {
		return nil, fmt.Errorf("%w while decompressing tiles", err)
	}

	tiles := make([]*image.Paletted, len(tileGroups))
	for j, tg := range tileGroups {
		tiles[j] = image.NewPaletted(image.Rect(0, 0, Width, Height), nil)

		for i, tIndex := range tg {
			flipH := false
			if tIndex < 0 {
				flipH = true
				tIndex = -tIndex
			}
			tIndex--

			tileImg, err := sprites.ReadTile(bytes.NewBuffer(rawTiles[tIndex*8*8/2 : (tIndex+1)*8*8/2]))
			if err != nil {
				return nil, fmt.Errorf("%w while reading tile %d", err, tIndex)
			}

			if flipH {
				paletted.FlipHorizontal(tileImg)
			}

			x := (i % 5) * 8
			y := (i / 5) * 8

			paletted.DrawOver(tiles[j], image.Rect(x, y, x+8, y+8), tileImg, image.Point{})
		}
	}

	return tiles, nil
}

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

type FrameInfo struct {
	Delay int
	IsEnd bool
}

var FrameInfos = func() []FrameInfo {
	frameInfos := make([]FrameInfo, 0, len(RedTileByIndex)*3)
	for k, tiles := range RedTileByIndex {
		for j := 0; j < 3; j++ {
			for i := 0; i < len(tiles); i++ {
				fi := FrameInfo{
					Delay: frameTimes[k],
					IsEnd: i == len(tiles)-1,
				}
				frameInfos = append(frameInfos, fi)
			}
		}
	}
	return frameInfos
}()
