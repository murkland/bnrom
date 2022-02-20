package paletted

import (
	"image"
)

func DrawOver(dst *image.Paletted, r image.Rectangle, src *image.Paletted, sp image.Point) {
	for y := 0; y < r.Dy(); y++ {
		for x := 0; x < r.Dx(); x++ {
			sx := src.Rect.Min.X + sp.X + x
			sy := src.Rect.Min.Y + sp.Y + y

			dx := dst.Rect.Min.X + r.Min.X + x
			dy := dst.Rect.Min.Y + r.Min.Y + y

			srcPixel := src.Pix[sy*src.Rect.Max.X+sx]
			if srcPixel == 0 {
				continue
			}
			dst.Pix[dy*dst.Rect.Max.X+dx] = srcPixel
		}
	}
}

func FlipHorizontal(img *image.Paletted) {
	w := img.Rect.Dx()
	h := img.Rect.Dy()

	for j := 0; j < h; j++ {
		y := img.Rect.Min.Y + j
		for i := 0; i < w/2; i++ {
			x0 := img.Rect.Min.X + i
			x1 := img.Rect.Min.X + w - i - 1
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
		copy(upper, img.Pix[y0*img.Rect.Max.X:y0*img.Rect.Max.X+w])
		copy(img.Pix[y0*img.Rect.Max.X:y0*img.Rect.Max.X+w], img.Pix[y1*img.Rect.Max.X:y1*img.Rect.Max.X+w])
		copy(img.Pix[y1*img.Rect.Max.X:y1*img.Rect.Max.X+w], upper)
	}
}

func FindTrim(img *image.Paletted) image.Rectangle {
	left := img.Rect.Min.X
	top := img.Rect.Min.Y
	right := img.Rect.Max.X
	bottom := img.Rect.Max.Y

	for left = img.Rect.Min.X; left < img.Rect.Max.X; left++ {
		for y := img.Rect.Min.Y; y < img.Rect.Max.Y; y++ {
			if img.Pix[y*img.Rect.Max.X+left] != 0 {
				goto leftDone
			}
		}
		continue
	leftDone:
		break
	}

	for top = img.Rect.Min.Y; top < img.Rect.Max.Y; top++ {
		for x := img.Rect.Min.X; x < img.Rect.Max.X; x++ {
			if img.Pix[top*img.Rect.Max.X+x] != 0 {
				goto topDone
			}
		}
		continue
	topDone:
		break
	}

	for right = img.Rect.Max.X - 1; right >= img.Rect.Min.X; right-- {
		for y := img.Rect.Min.Y; y < img.Rect.Max.Y; y++ {
			if img.Pix[y*img.Rect.Max.X+right] != 0 {
				goto rightDone
			}
		}
		continue
	rightDone:
		break
	}
	right++

	for bottom = img.Rect.Max.Y - 1; bottom >= img.Rect.Min.Y; bottom-- {
		for x := img.Rect.Min.X; x < img.Rect.Max.X; x++ {
			if img.Pix[bottom*img.Rect.Max.X+x] != 0 {
				goto bottomDone
			}
		}
		continue
	bottomDone:
		break
	}
	bottom++

	if right < left || bottom < top {
		return image.Rect(0, 0, 0, 0)
	}

	return image.Rectangle{image.Point{left, top}, image.Point{right, bottom}}
}
