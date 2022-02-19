package sprites

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/color"
	"io"
	"os"

	"github.com/yumland/bnrom/paletted"
	"github.com/yumland/gbarom/bgr555"
	"github.com/yumland/gbarom/lz77"
)

type Flip uint8

const (
	FlipNone Flip = 0
	FlipH    Flip = 0x4
	FlipV    Flip = 0x8
	FlipBoth      = FlipH | FlipV
)

type OAMEntry struct {
	TileIndex     int
	X             int
	Y             int
	WTiles        int
	HTiles        int
	PaletteOffset int
	Flip          Flip
}

func ReadOAMEntry(r io.Reader) (*OAMEntry, error) {
	var rawEnt struct {
		TileIndex   uint8
		X           int8
		Y           int8
		SizeAndFlip uint8
		POAndSM     uint8
	}

	if err := binary.Read(r, binary.LittleEndian, &rawEnt); err != nil {
		return nil, fmt.Errorf("%w while reading oam entry", err)
	}

	if rawEnt.TileIndex == 0xFF {
		// End of OAM entries.
		return nil, nil
	}

	var ent OAMEntry
	ent.TileIndex = int(rawEnt.TileIndex)
	ent.X = int(rawEnt.X)
	ent.Y = int(rawEnt.Y)
	ent.Flip = Flip(rawEnt.SizeAndFlip >> 4)
	ent.PaletteOffset = int(rawEnt.POAndSM >> 4)

	size := rawEnt.SizeAndFlip & 0x0F
	sizeModifier := rawEnt.POAndSM & 0x0F

	switch (size << 3) | sizeModifier {
	case (0 << 3) | 0:
		ent.WTiles = 1
		ent.HTiles = 1
	case (0 << 3) | 1:
		ent.WTiles = 2
		ent.HTiles = 1
	case (0 << 3) | 2:
		ent.WTiles = 1
		ent.HTiles = 2

	case (1 << 3) | 0:
		ent.WTiles = 2
		ent.HTiles = 2
	case (1 << 3) | 1:
		ent.WTiles = 4
		ent.HTiles = 1
	case (1 << 3) | 2:
		ent.WTiles = 1
		ent.HTiles = 4

	case (2 << 3) | 0:
		ent.WTiles = 4
		ent.HTiles = 4
	case (2 << 3) | 1:
		ent.WTiles = 4
		ent.HTiles = 2
	case (2 << 3) | 2:
		ent.WTiles = 2
		ent.HTiles = 4

	case (3 << 3) | 0:
		ent.WTiles = 8
		ent.HTiles = 8
	case (3 << 3) | 1:
		ent.WTiles = 8
		ent.HTiles = 4
	case (3 << 3) | 2:
		ent.WTiles = 4
		ent.HTiles = 8
	}
	return &ent, nil
}

type FrameAction uint16

const (
	FrameActionNext FrameAction = 0x00
	FrameActionLoop FrameAction = 0xC0
	FrameActionStop FrameAction = 0x80
)

type Frame struct {
	Palette    color.Palette
	Delay      uint16
	Action     FrameAction
	Tiles      []*image.Paletted
	OAMEntries []OAMEntry
}

func ReadTile(r io.Reader) (*image.Paletted, error) {
	var pixels [8 * 8 / 2]uint8
	if _, err := io.ReadFull(r, pixels[:]); err != nil {
		return nil, err
	}

	pimg := image.NewPaletted(image.Rect(0, 0, 8, 8), nil)
	for i, p := range pixels {
		pimg.Pix[i*2] = p & 0xF
		pimg.Pix[i*2+1] = p >> 4
	}

	return pimg, nil
}

func ReadPalette(r io.Reader) (color.Palette, error) {
	var palette color.Palette

	for {
		var c uint16
		if err := binary.Read(r, binary.LittleEndian, &c); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, err
		}

		palette = append(palette, bgr555.ToRGBA(c))
	}
	return palette, nil
}

func ReadFrame(r io.ReadSeeker, offset int64) (Frame, error) {
	var fr Frame

	var rawFr struct {
		TilesPtr  uint32
		PalPtr    uint32
		JunkPtr   uint32
		OAMPtrPtr uint32
		Delay     uint16
		Action    uint16
	}

	if err := binary.Read(r, binary.LittleEndian, &rawFr); err != nil {
		return fr, fmt.Errorf("%w while reading frame", err)
	}

	retOffset, err := r.Seek(0, os.SEEK_CUR)
	if err != nil {
		return fr, fmt.Errorf("%w while remembering offset", err)
	}
	defer func() {
		r.Seek(retOffset, os.SEEK_SET)
	}()

	fr.Delay = rawFr.Delay
	fr.Action = FrameAction(rawFr.Action)

	// Decode tiles.
	if _, err := r.Seek(offset+4+int64(rawFr.TilesPtr), os.SEEK_SET); err != nil {
		return fr, fmt.Errorf("%w while seeking to tiles at tile pointer 0x%08x", err, rawFr.TilesPtr)
	}

	var tilesByteSize uint32
	if err := binary.Read(r, binary.LittleEndian, &tilesByteSize); err != nil {
		return fr, fmt.Errorf("%w reading tiles at tile pointer 0x%08x", err, rawFr.TilesPtr)
	}

	numTiles := tilesByteSize / (8 * 8 / 2)

	fr.Tiles = make([]*image.Paletted, numTiles)
	for i := 0; i < int(numTiles); i++ {
		var err error
		fr.Tiles[i], err = ReadTile(r)
		if err != nil {
			return fr, fmt.Errorf("%w while reading tile %d at pointer 0x%08x", err, i, rawFr.TilesPtr)
		}
	}

	// Decode palette.
	if _, err := r.Seek(offset+4+int64(rawFr.PalPtr), os.SEEK_SET); err != nil {
		return fr, fmt.Errorf("%w while seeking to palette at palette pointer 0x%08x", err, rawFr.PalPtr)
	}

	var paletteByteSize uint32
	if err := binary.Read(r, binary.LittleEndian, &paletteByteSize); err != nil {
		return fr, fmt.Errorf("%w while reading palette header at palette pointer 0x%08x", err, rawFr.PalPtr)
	}

	// TODO: Something useful with paletteByteSize?
	// TODO: Surely nothing has more than 64 palettes?
	for i := 0; i < 64; i++ {
		var raw [16 * 2]byte
		if _, err := io.ReadFull(r, raw[:]); err != nil {
			if errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, io.EOF) {
				break
			}
			return fr, fmt.Errorf("%w while reading palbank %d at palette pointer 0x%08x", err, i, rawFr.PalPtr)
		}

		if binary.LittleEndian.Uint32(raw[:4]) == 4 {
			break
		}

		palette, err := ReadPalette(bytes.NewBuffer(raw[:]))
		if err != nil {
			return fr, fmt.Errorf("%w while reading palbank %d at palette pointer 0x%08x", err, i, rawFr.PalPtr)
		}

		// Palette entry 0 is always transparent.
		palette[0] = color.RGBA{}
		fr.Palette = append(fr.Palette, palette...)
	}

	// Decode OAM entries.
	if _, err := r.Seek(offset+4+int64(rawFr.OAMPtrPtr), os.SEEK_SET); err != nil {
		return fr, fmt.Errorf("%w while seeking to OAM pointer at OAM pointer pointer 0x%08x", err, rawFr.OAMPtrPtr)
	}

	var oamPtr uint32
	if err := binary.Read(r, binary.LittleEndian, &oamPtr); err != nil {
		return fr, fmt.Errorf("%w while reading OAM pointer at OAM pointer pointer 0x%08x", err, rawFr.OAMPtrPtr)
	}

	if _, err := r.Seek(offset+4+int64(rawFr.OAMPtrPtr+oamPtr), os.SEEK_SET); err != nil {
		return fr, fmt.Errorf("%w while reading OAM at OAM pointer 0x%08x", err, oamPtr)
	}

	for i := 0; ; i++ {
		oamEntry, err := ReadOAMEntry(r)
		if err != nil {
			return fr, fmt.Errorf("%w while reading OAM entry %d at OAM pointer 0x%08x", err, i, oamPtr)
		}

		if oamEntry == nil {
			break
		}

		fr.OAMEntries = append(fr.OAMEntries, *oamEntry)
	}

	return fr, nil
}

func (f *Frame) MakeImage() *image.Paletted {
	palSize := 256
	if len(f.Palette) < palSize {
		palSize = len(f.Palette)
	}

	img := image.NewPaletted(image.Rect(0, 0, 512, 512), f.Palette[:palSize])

	for _, oamEntry := range f.OAMEntries {
		oamImg := image.NewPaletted(image.Rect(0, 0, oamEntry.WTiles*8, oamEntry.HTiles*8), img.Palette)

		for j := 0; j < oamEntry.HTiles; j++ {
			for i := 0; i < oamEntry.WTiles; i++ {
				tile := f.Tiles[oamEntry.TileIndex+j*oamEntry.WTiles+i]
				tileCopy := image.NewPaletted(image.Rect(0, 0, 8, 8), nil)
				for k := 0; k < len(tile.Pix); k++ {
					if tile.Pix[k] != 0 {
						tileCopy.Pix[k] = tile.Pix[k] + uint8(16*oamEntry.PaletteOffset)
					} else {
						tileCopy.Pix[k] = 0
					}
				}
				paletted.DrawOver(oamImg, image.Rect(i*8, j*8, (i+1)*8, (j+1)*8), tileCopy, image.Point{})
			}
		}

		if oamEntry.Flip&FlipH != 0 {
			paletted.FlipHorizontal(oamImg)
		}

		if oamEntry.Flip&FlipV != 0 {
			paletted.FlipVertical(oamImg)
		}

		paletted.DrawOver(img, image.Rect(
			oamEntry.X+img.Rect.Dx()/2,
			oamEntry.Y+img.Rect.Dy()/2,
			oamEntry.X+img.Rect.Dx()/2+oamImg.Rect.Dx(),
			oamEntry.Y+img.Rect.Dy()/2+oamImg.Rect.Dy(),
		), oamImg, image.Point{})
	}

	return img
}

type Animation struct {
	Frames []Frame
}

func ReadAnimation(r io.ReadSeeker, offset int64) (Animation, error) {
	var anim Animation

	var animPtr uint32
	if err := binary.Read(r, binary.LittleEndian, &animPtr); err != nil {
		return anim, fmt.Errorf("%w while reading animation pointer", err)
	}

	retOffset, err := r.Seek(0, os.SEEK_CUR)
	if err != nil {
		return anim, fmt.Errorf("%w while remembering offset", err)
	}
	defer func() {
		r.Seek(retOffset, os.SEEK_SET)
	}()

	if _, err := r.Seek(offset+4+int64(animPtr), os.SEEK_SET); err != nil {
		return anim, fmt.Errorf("%w while seeking to animation pointer 0x%08x", err, animPtr)
	}

	for i := 0; ; i++ {
		frame, err := ReadFrame(r, offset)
		if err != nil {
			return anim, fmt.Errorf("%w while reading frame %d at animation pointer 0x%08x", err, i, animPtr)
		}

		anim.Frames = append(anim.Frames, frame)
		if frame.Action != FrameActionNext {
			break
		}
	}

	return anim, nil
}

func ReadAnimations(r io.ReadSeeker, offset int64) ([]Animation, error) {
	if _, err := io.CopyN(io.Discard, r, 3); err != nil {
		return nil, fmt.Errorf("%w while discarding header", err)
	}

	var n uint8
	if err := binary.Read(r, binary.LittleEndian, &n); err != nil {
		return nil, fmt.Errorf("%w while animation count", err)
	}

	anims := make([]Animation, n)
	for i := 0; i < len(anims); i++ {
		anim, err := ReadAnimation(r, offset)
		if err != nil {
			return nil, fmt.Errorf("%w while reading animation %d", err, i)
		}
		anims[i] = anim
	}

	return anims, nil
}

func ReadNext(r io.ReadSeeker) ([]Animation, error) {
	var animPtr uint32
	if err := binary.Read(r, binary.LittleEndian, &animPtr); err != nil {
		return nil, fmt.Errorf("%w while reading sprite pointer 0x%08x", err, animPtr)
	}

	retOffset, err := r.Seek(0, os.SEEK_CUR)
	if err != nil {
		return nil, fmt.Errorf("%w while remembering offset for sprite pointer 0x%08x", err, animPtr)
	}

	defer func() {
		r.Seek(retOffset, os.SEEK_SET)
	}()

	animR := r

	isLZ77 := animPtr&0x80000000 == 0x80000000
	realPtr := animPtr & ^uint32(0x88000000)

	if isLZ77 {
		if _, err := r.Seek(int64(realPtr), os.SEEK_SET); err != nil {
			return nil, fmt.Errorf("%w while seeking to LZ77 sprite pointer 0x%08x", err, animPtr)
		}

		buf, err := lz77.Decompress(r)
		if err != nil {
			return nil, fmt.Errorf("%w while decompressing LZ77 sprite pointer 0x%08x", err, animPtr)
		}

		animR = bytes.NewReader(buf)
		realPtr = 4
	}

	if _, err := animR.Seek(int64(realPtr), os.SEEK_SET); err != nil {
		return nil, fmt.Errorf("%w while seeking sprite pointer 0x%08x", err, animPtr)
	}

	anims, err := ReadAnimations(animR, int64(realPtr))
	if err != nil {
		return nil, fmt.Errorf("%w while reading sprite at sprite pointer 0x%08x", err, animPtr)
	}

	return anims, nil
}
