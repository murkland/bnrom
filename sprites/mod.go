package sprites

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"io"
	"os"
	"strings"

	"github.com/nbarena/gbarom/bgr555"
	"github.com/nbarena/gbarom/lz77"
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
	var ent OAMEntry

	var tileIdx uint8
	var x, y int8
	var sizeAndFlip, poAndSm uint8

	if err := binary.Read(r, binary.LittleEndian, &tileIdx); err != nil {
		return &ent, fmt.Errorf("%w while reading tile index", err)
	}

	if tileIdx == 0xFF {
		// End of OAM entries.
		return nil, nil
	}

	ent.TileIndex = int(tileIdx)

	if err := binary.Read(r, binary.LittleEndian, &x); err != nil {
		return &ent, fmt.Errorf("%w while reading x", err)
	}
	ent.X = int(x)

	if err := binary.Read(r, binary.LittleEndian, &y); err != nil {
		return &ent, fmt.Errorf("%w while reading y", err)
	}
	ent.Y = int(y)

	if err := binary.Read(r, binary.LittleEndian, &sizeAndFlip); err != nil {
		return &ent, fmt.Errorf("%w while reading size and flip", err)
	}
	ent.Flip = Flip(sizeAndFlip >> 4)

	if err := binary.Read(r, binary.LittleEndian, &poAndSm); err != nil {
		return &ent, fmt.Errorf("%w while reading palette offset and size modifier", err)
	}
	ent.PaletteOffset = int(poAndSm >> 4)

	size := sizeAndFlip & 0x0F
	sizeModifier := poAndSm & 0x0F

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

	var tilesPtr uint32
	if err := binary.Read(r, binary.LittleEndian, &tilesPtr); err != nil {
		return fr, fmt.Errorf("%w while reading tiles pointer", err)
	}

	var palPtr uint32
	if err := binary.Read(r, binary.LittleEndian, &palPtr); err != nil {
		return fr, fmt.Errorf("%w while reading palette pointer", err)
	}

	if _, err := io.CopyN(io.Discard, r, 4); err != nil {
		return fr, fmt.Errorf("%w while reading junk pointer", err)
	}

	var oamPtrPtr uint32
	if err := binary.Read(r, binary.LittleEndian, &oamPtrPtr); err != nil {
		return fr, fmt.Errorf("%w while reading OAM pointer pointer", err)
	}

	if err := binary.Read(r, binary.LittleEndian, &fr.Delay); err != nil {
		return fr, fmt.Errorf("%w while reading delay", err)
	}

	if err := binary.Read(r, binary.LittleEndian, &fr.Action); err != nil {
		return fr, fmt.Errorf("%w while reading action", err)
	}

	retOffset, err := r.Seek(0, os.SEEK_CUR)
	if err != nil {
		return fr, fmt.Errorf("%w while remembering offset", err)
	}
	defer func() {
		r.Seek(retOffset, os.SEEK_SET)
	}()

	// Decode tiles.
	if _, err := r.Seek(offset+4+int64(tilesPtr), os.SEEK_SET); err != nil {
		return fr, fmt.Errorf("%w while seeking to tiles at tile pointer 0x%08x", err, tilesPtr)
	}

	var tilesByteSize uint32
	if err := binary.Read(r, binary.LittleEndian, &tilesByteSize); err != nil {
		return fr, fmt.Errorf("%w reading tiles at tile pointer 0x%08x", err, tilesPtr)
	}

	numTiles := tilesByteSize / (8 * 8 / 2)

	fr.Tiles = make([]*image.Paletted, numTiles)
	for i := 0; i < int(numTiles); i++ {
		var err error
		fr.Tiles[i], err = ReadTile(r)
		if err != nil {
			return fr, fmt.Errorf("%w while reading tile %d at pointer 0x%08x", err, i, tilesPtr)
		}
	}

	// Decode palette.
	if _, err := r.Seek(offset+4+int64(palPtr), os.SEEK_SET); err != nil {
		return fr, fmt.Errorf("%w while seeking to palette at palette pointer 0x%08x", err, palPtr)
	}

	var paletteByteSize uint32
	if err := binary.Read(r, binary.LittleEndian, &paletteByteSize); err != nil {
		return fr, fmt.Errorf("%w while reading palette header at palette pointer 0x%08x", err, palPtr)
	}

	// TODO: Something useful with paletteByteSize?
	// TODO: Surely nothing has more than 64 palettes?
	for i := 0; i < 64; i++ {
		var raw [16 * 2]byte
		if _, err := io.ReadFull(r, raw[:]); err != nil {
			if errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, io.EOF) {
				break
			}
			return fr, fmt.Errorf("%w while reading palbank %d at palette pointer 0x%08x", err, i, palPtr)
		}

		if binary.LittleEndian.Uint32(raw[:4]) == 4 {
			break
		}

		palette, err := ReadPalette(bytes.NewBuffer(raw[:]))
		if err != nil {
			return fr, fmt.Errorf("%w while reading palbank %d at palette pointer 0x%08x", err, i, palPtr)
		}

		// Palette entry 0 is always transparent.
		palette[0] = color.RGBA{}
		fr.Palette = append(fr.Palette, palette...)
	}

	// Decode OAM entries.
	if _, err := r.Seek(offset+4+int64(oamPtrPtr), os.SEEK_SET); err != nil {
		return fr, fmt.Errorf("%w while seeking to OAM pointer at OAM pointer pointer 0x%08x", err, oamPtrPtr)
	}

	var oamPtr uint32
	if err := binary.Read(r, binary.LittleEndian, &oamPtr); err != nil {
		return fr, fmt.Errorf("%w while reading OAM pointer at OAM pointer pointer 0x%08x", err, oamPtrPtr)
	}

	if _, err := r.Seek(offset+4+int64(oamPtrPtr+oamPtr), os.SEEK_SET); err != nil {
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
				mask := image.NewAlpha(tile.Rect)
				for k := 0; k < len(tile.Pix); k++ {
					a := uint8(0)
					if tile.Pix[k] != 0 {
						a = 0xFF
					}
					mask.Pix[k] = a
				}

				tileCopy := image.NewPaletted(image.Rect(0, 0, 8, 8), img.Palette)
				for k := 0; k < len(tile.Pix); k++ {
					tileCopy.Pix[k] = tile.Pix[k] + uint8(16*oamEntry.PaletteOffset)
				}
				draw.DrawMask(oamImg, image.Rect(i*8, j*8, (i+1)*8, (j+1)*8), tileCopy, image.Point{}, mask, image.Point{}, draw.Over)
			}
		}

		ow := oamImg.Rect.Dx()
		oh := oamImg.Rect.Dy()

		if oamEntry.Flip&FlipH != 0 {
			// Horizontal flips suck!
			for j := 0; j < oh; j++ {
				for i := 0; i < ow/2; i++ {
					oamImg.Pix[j*ow+i], oamImg.Pix[j*ow+(ow-1-i)] = oamImg.Pix[j*ow+(ow-1-i)], oamImg.Pix[j*ow+i]
				}
			}
		}

		if oamEntry.Flip&FlipV != 0 {
			// Vertical flips rule!
			for j := 0; j < oh/2; j++ {
				upper := make([]uint8, ow)
				copy(upper, oamImg.Pix[j*ow:(j+1)*ow])
				copy(oamImg.Pix[j*ow:(j+1)*ow], oamImg.Pix[(oh-j-1)*ow:(oh-j-1+1)*ow])
				copy(oamImg.Pix[(oh-j-1)*ow:(oh-j-1+1)*ow], upper)
			}
		}

		mask := image.NewAlpha(oamImg.Rect)
		for k := 0; k < len(oamImg.Pix); k++ {
			a := uint8(0)
			if oamImg.Pix[k] != 0 {
				a = 0xFF
			}
			mask.Pix[k] = a
		}

		draw.DrawMask(img, image.Rect(
			oamEntry.X+img.Rect.Dx()/2,
			oamEntry.Y+img.Rect.Dy()/2,
			oamEntry.X+img.Rect.Dx()/2+oamImg.Rect.Dx(),
			oamEntry.Y+img.Rect.Dy()/2+oamImg.Rect.Dy(),
		), oamImg, image.Point{}, mask, image.Point{}, draw.Over)
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

type ROMInfo struct {
	ID     string
	Offset int64
	Count  int
}

func ReadROMInfo(r io.ReadSeeker) (*ROMInfo, error) {
	romID, err := ReadROMID(r)
	if err != nil {
		return nil, err
	}

	switch romID {
	case "BR6E", "BR6P", "BR5E", "BR5P":
		return &ROMInfo{romID, 0x00031CEC, 815}, nil
	case "BR6J", "BR5J":
		return &ROMInfo{romID, 0x00032CA8, 815}, nil
	case "BRBE":
		return &ROMInfo{romID, 0x00032750, 664}, nil
	case "BRKE":
		return &ROMInfo{romID, 0x00032754, 664}, nil
	case "BRBJ":
		return &ROMInfo{romID, 0x000326e8, 664}, nil
	case "BRKJ":
		return &ROMInfo{romID, 0x000326ec, 664}, nil
	case "BR4J":
		return &ROMInfo{romID, 0x0002b39c, 568}, nil
	case "B4BE":
		return &ROMInfo{romID, 0x00027968, 616}, nil
	case "B4WE":
		return &ROMInfo{romID, 0x00027964, 616}, nil
	case "B4BJ":
		return &ROMInfo{romID, 0x00027880, 616}, nil
	case "B4WJ":
		return &ROMInfo{romID, 0x0002787c, 616}, nil
	case "A6BE":
		return &ROMInfo{romID, 0x000247a0, 821}, nil
	case "A3XE":
		return &ROMInfo{romID, 0x00024788, 821}, nil
	case "A6BJ":
		return &ROMInfo{romID, 0x000248f8, 565}, nil
	case "A3XJ":
		return &ROMInfo{romID, 0x000248e0, 564}, nil
	case "AE2E":
		return &ROMInfo{romID, 0x0001e9fc, 501}, nil
	case "AE2J":
		return &ROMInfo{romID, 0x0001e888, 501}, nil
	case "AREE":
		return &ROMInfo{romID, 0x00012690, 344}, nil
	case "AREP":
		return &ROMInfo{romID, 0x0001269c, 344}, nil
	case "AREJ":
		return &ROMInfo{romID, 0x00012614, 344}, nil
	}
	return nil, nil
}

func ReadROMID(r io.ReadSeeker) (string, error) {
	var romID [4]byte
	if _, err := r.Seek(0x000000AC, os.SEEK_SET); err != nil {
		return "", err
	}

	if _, err := io.ReadFull(r, romID[:]); err != nil {
		return "", err
	}

	return string(romID[:]), nil
}

func ReadROMTitle(r io.ReadSeeker) (string, error) {
	var romTitle [12]byte
	if _, err := r.Seek(0x000000A0, os.SEEK_SET); err != nil {
		return "", err
	}

	if _, err := io.ReadFull(r, romTitle[:]); err != nil {
		return "", err
	}

	return strings.TrimRight(string(romTitle[:]), "\x00"), nil
}
