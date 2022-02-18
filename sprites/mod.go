package sprites

import (
	"bytes"
	"encoding/binary"
	"errors"
	"image"
	"image/color"
	"image/draw"
	"io"
	"os"

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
		return &ent, err
	}

	if tileIdx == 0xFF {
		// End of OAM entries.
		return nil, nil
	}

	ent.TileIndex = int(tileIdx)

	if err := binary.Read(r, binary.LittleEndian, &x); err != nil {
		return &ent, err
	}
	ent.X = int(x)

	if err := binary.Read(r, binary.LittleEndian, &y); err != nil {
		return &ent, err
	}
	ent.Y = int(y)

	if err := binary.Read(r, binary.LittleEndian, &sizeAndFlip); err != nil {
		return &ent, err
	}
	ent.Flip = Flip(sizeAndFlip >> 4)

	if err := binary.Read(r, binary.LittleEndian, &poAndSm); err != nil {
		return &ent, err
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

func GuessIsPalbank(raw []byte) bool {
	if len(raw) != 0x20 {
		return false
	}

	ptr := binary.LittleEndian.Uint32(raw[:4])
	raw = raw[4:]

	if ptr == 0x80010004 {
		return false
	}

	if ptr%4 != 0 {
		return true
	}

	if ptr == 0 {
		return true
	}

	for i := 0; i < int(ptr/4); i++ {
		if len(raw) == 0 {
			return true
		}

		nextPtr := binary.LittleEndian.Uint32(raw[:4])
		raw = raw[4:]

		if nextPtr%4 != 0 {
			return true
		}

		if nextPtr < ptr {
			return true
		}

		ptr = nextPtr
	}

	return false
}

type FrameAction uint16

const (
	FrameActionNext FrameAction = 0x00
	FrameActionLoop FrameAction = 0xC0
	FrameActionStop FrameAction = 0x80
)

type Frame struct {
	Image   *image.Paletted
	Palette color.Palette
	Delay   uint16
	Action  FrameAction
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
		return fr, err
	}

	var palPtr uint32
	if err := binary.Read(r, binary.LittleEndian, &palPtr); err != nil {
		return fr, err
	}

	if _, err := io.CopyN(io.Discard, r, 4); err != nil {
		return fr, err
	}

	var oamPtrPtr uint32
	if err := binary.Read(r, binary.LittleEndian, &oamPtrPtr); err != nil {
		return fr, err
	}

	if err := binary.Read(r, binary.LittleEndian, &fr.Delay); err != nil {
		return fr, err
	}

	if err := binary.Read(r, binary.LittleEndian, &fr.Action); err != nil {
		return fr, err
	}

	retOffset, err := r.Seek(0, os.SEEK_CUR)
	if err != nil {
		return fr, err
	}
	defer func() {
		r.Seek(retOffset, os.SEEK_SET)
	}()

	// Decode tiles.
	if _, err := r.Seek(offset+4+int64(tilesPtr), os.SEEK_SET); err != nil {
		return fr, err
	}

	var tilesByteSize uint32
	if err := binary.Read(r, binary.LittleEndian, &tilesByteSize); err != nil {
		return fr, err
	}

	numTiles := tilesByteSize / (8 * 8 / 2)

	tiles := make([]*image.Paletted, numTiles)
	for i := 0; i < int(numTiles); i++ {
		var err error
		tiles[i], err = ReadTile(r)
		if err != nil {
			return fr, err
		}
	}

	// Decode palette.
	if _, err := r.Seek(offset+4+int64(palPtr), os.SEEK_SET); err != nil {
		return fr, err
	}

	var paletteByteSize uint32
	if err := binary.Read(r, binary.LittleEndian, &paletteByteSize); err != nil {
		return fr, err
	}
	// TODO: Something useful with paletteByteSize?
	for {
		var raw [16 * 2]byte
		if _, err := io.ReadFull(r, raw[:]); err != nil {
			if errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, io.EOF) {
				break
			}
			return fr, err
		}

		if !GuessIsPalbank(raw[:]) {
			break
		}

		palette, err := ReadPalette(bytes.NewReader(raw[:]))
		if err != nil {
			return fr, err
		}
		// Palette entry 0 is always transparent.
		palette[0] = color.RGBA{}
		fr.Palette = append(fr.Palette, palette...)
	}

	// Decode OAM entries.
	if _, err := r.Seek(offset+4+int64(oamPtrPtr), os.SEEK_SET); err != nil {
		return fr, err
	}

	var oamPtr uint32
	if err := binary.Read(r, binary.LittleEndian, &oamPtr); err != nil {
		return fr, err
	}

	if _, err := r.Seek(offset+4+int64(oamPtrPtr+oamPtr), os.SEEK_SET); err != nil {
		return fr, err
	}

	palSize := 256
	if len(fr.Palette) < palSize {
		palSize = len(fr.Palette)
	}

	fr.Image = image.NewPaletted(image.Rect(0, 0, 512, 512), fr.Palette[:palSize])
	for {
		oamEntry, err := ReadOAMEntry(r)
		if err != nil {
			return fr, nil
		}

		if oamEntry == nil {
			break
		}

		oamImg := image.NewPaletted(image.Rect(0, 0, oamEntry.WTiles*8, oamEntry.HTiles*8), fr.Image.Palette)

		for j := 0; j < oamEntry.HTiles; j++ {
			for i := 0; i < oamEntry.WTiles; i++ {
				tile := tiles[oamEntry.TileIndex+j*oamEntry.WTiles+i]
				mask := image.NewAlpha(tile.Rect)
				for k := 0; k < len(tile.Pix); k++ {
					a := uint8(0)
					if tile.Pix[k] != 0 {
						a = 0xFF
					}
					mask.Pix[k] = a
				}

				tileCopy := image.NewPaletted(image.Rect(0, 0, 8, 8), fr.Image.Palette)
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

		draw.DrawMask(fr.Image, image.Rect(
			oamEntry.X+fr.Image.Rect.Dx()/2,
			oamEntry.Y+fr.Image.Rect.Dy()/2,
			oamEntry.X+fr.Image.Rect.Dx()/2+oamImg.Rect.Dx(),
			oamEntry.Y+fr.Image.Rect.Dy()/2+oamImg.Rect.Dy(),
		), oamImg, image.Point{}, mask, image.Point{}, draw.Over)
	}

	return fr, nil
}

type Animation struct {
	Frames []Frame
}

func ReadAnimation(r io.ReadSeeker, offset int64) (Animation, error) {
	var anim Animation

	var animPtr uint32
	if err := binary.Read(r, binary.LittleEndian, &animPtr); err != nil {
		return anim, err
	}

	retOffset, err := r.Seek(0, os.SEEK_CUR)
	if err != nil {
		return anim, err
	}
	defer func() {
		r.Seek(retOffset, os.SEEK_SET)
	}()

	if _, err := r.Seek(offset+4+int64(animPtr), os.SEEK_SET); err != nil {
		return anim, err
	}

	for {
		frame, err := ReadFrame(r, offset)
		if err != nil {
			return anim, err
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
		return nil, err
	}

	var n uint8
	if err := binary.Read(r, binary.LittleEndian, &n); err != nil {
		return nil, err
	}

	anims := make([]Animation, n)
	for i := 0; i < len(anims); i++ {
		anim, err := ReadAnimation(r, offset)
		if err != nil {
			return nil, err
		}
		anims[i] = anim
	}

	return anims, nil
}

func Read(r io.ReadSeeker, n int) ([][]Animation, error) {
	sprites := make([][]Animation, n)

	for i := 0; i < n; i++ {
		var animPtr uint32
		if err := binary.Read(r, binary.LittleEndian, &animPtr); err != nil {
			return sprites, err
		}

		retOffset, err := r.Seek(0, os.SEEK_CUR)
		if err != nil {
			return sprites, err
		}

		animR := r

		isLZ77 := animPtr&0x80000000 == 0x80000000
		animPtr &= ^uint32(0x88000000)

		if isLZ77 {
			if _, err := r.Seek(int64(animPtr), os.SEEK_SET); err != nil {
				return sprites, err
			}

			buf, err := lz77.Decompress(r)
			if err != nil {
				return sprites, err
			}

			animR = bytes.NewReader(buf)
			animPtr = 4
		}

		if _, err := animR.Seek(int64(animPtr), os.SEEK_SET); err != nil {
			return sprites, err
		}

		anims, err := ReadAnimations(animR, int64(animPtr))
		if err != nil {
			return sprites, err
		}

		sprites[i] = anims

		if _, err := r.Seek(retOffset, os.SEEK_SET); err != nil {
			return sprites, err
		}
	}

	return sprites, nil
}
