package paletted

import (
	"image"
)

func DrawSimpleMaskOver(dst *image.Paletted, r image.Rectangle, src *image.Paletted, sp image.Point, mask *image.Alpha, mp image.Point) {
	var mw int
	if mask != nil {
		mw = mask.Rect.Dx()
	}

	for y := 0; y < r.Dy(); y++ {
		for x := 0; x < r.Dx(); x++ {
			sx := src.Rect.Min.X + sp.X + x
			sy := src.Rect.Min.Y + sp.Y + y

			dx := dst.Rect.Min.X + r.Min.X + x
			dy := dst.Rect.Min.Y + r.Min.Y + y

			if mask != nil {
				mx := mask.Rect.Min.X + mp.X + x
				my := mask.Rect.Min.Y + mp.X + y
				if mask.Pix[my*mw+mx] == 0 {
					continue
				}
			}

			srcPixel := src.Pix[sy*src.Rect.Max.X+sx]
			dst.Pix[dy*dst.Rect.Max.X+dx] = srcPixel
		}
	}
}

func DrawOver(dst *image.Paletted, r image.Rectangle, src *image.Paletted, sp image.Point) {
	DrawSimpleMaskOver(dst, r, src, sp, nil, image.Point{})
}

func FlipHorizontal(img *image.Paletted) {
	w := img.Rect.Dx()
	h := img.Rect.Dy()

	for j := 0; j < h; j++ {
		for i := 0; i < w/2; i++ {
			x0 := img.Rect.Min.X + i
			x1 := img.Rect.Min.X + w - i - 1
			y := img.Rect.Min.Y + j

			img.Pix[y*img.Rect.Max.X+x0], img.Pix[y*img.Rect.Max.X+x1] = img.Pix[y*img.Rect.Max.X+x1], img.Pix[y*img.Rect.Max.X+x0]
		}
	}
}

func FlipVertical(img *image.Paletted) {
	w := img.Rect.Dx()
	h := img.Rect.Dy()

	for j := 0; j < h/2; j++ {
		y0 := img.Rect.Min.Y + j
		y1 := img.Rect.Min.Y + h - j - 1

		upper := make([]uint8, w)
		copy(upper, img.Pix[j*w:(y0+1)*w])
		copy(img.Pix[y0*w:(y0+1)*w], img.Pix[y1*w:(y1+1)*w])
		copy(img.Pix[y1*w:(y1+1)*w], upper)
	}
}
