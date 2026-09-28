// Package icon draws the tray icon: an arrow, with the count of active
// forwards in a badge. Drawn rather than taken from the icon theme, so that
// the count shows on every desktop.
package icon

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"strconv"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
	"golang.org/x/image/vector"
)

// Mid-tone colors of the GNOME palette: readable on a dark panel as well as
// on a light one, since the tray does not tell which one it is
var (
	idleColor  = color.NRGBA{0x9a, 0x99, 0x96, 0xff}
	busyColor  = color.NRGBA{0x2e, 0xc2, 0x7e, 0xff}
	badgeColor = color.NRGBA{0x35, 0x84, 0xe4, 0xff}
	digitColor = color.NRGBA{0xff, 0xff, 0xff, 0xff}
)

// Render returns the icon as a PNG of sizexsize pixels.
func Render(size, active int, busy bool) ([]byte, error) {
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	arrowColor := idleColor
	if busy {
		arrowColor = busyColor
	}
	fill(img, arrow(size), arrowColor)

	if active > 0 {
		if err := badge(img, strconv.Itoa(min(active, 99))); err != nil {
			return nil, err
		}
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// arrow returns a right-pointing arrow, the "forward", in a sizexsize box.
func arrow(size int) *vector.Rasterizer {
	s := float32(size)
	r := vector.NewRasterizer(size, size)
	r.MoveTo(0.08*s, 0.40*s)
	r.LineTo(0.50*s, 0.40*s)
	r.LineTo(0.50*s, 0.20*s)
	r.LineTo(0.92*s, 0.50*s)
	r.LineTo(0.50*s, 0.80*s)
	r.LineTo(0.50*s, 0.60*s)
	r.LineTo(0.08*s, 0.60*s)
	r.ClosePath()
	return r
}

func fill(img draw.Image, r *vector.Rasterizer, c color.Color) {
	r.Draw(img, img.Bounds(), image.NewUniform(c), image.Point{})
}

// badge draws text in a pill at the bottom right corner.
func badge(img *image.NRGBA, text string) error {
	size := img.Bounds().Dx()
	h := float32(size) * 0.5
	w := h
	if len(text) > 1 {
		w = h * 1.45
	}
	x0, y0 := float32(size)-w, float32(size)-h
	fill(img, pill(size, x0, y0, w, h), badgeColor)

	face, err := digitFace(float64(h) * 0.8)
	if err != nil {
		return err
	}
	defer func() { _ = face.Close() }()

	d := font.Drawer{Dst: img, Src: image.NewUniform(digitColor), Face: face}
	advance := d.MeasureString(text)
	metrics := face.Metrics()
	// Center the digits: they have no descender, so the cap height is the ascent
	x := fixed.Int26_6((x0+w/2)*64) - advance/2
	y := fixed.Int26_6((y0+h/2)*64) + metrics.CapHeight/2
	d.Dot = fixed.Point26_6{X: x, Y: y}
	d.DrawString(text)
	return nil
}

// pill returns a rectangle with fully rounded ends: a circle when w == h.
func pill(size int, x, y, w, h float32) *vector.Rasterizer {
	r := vector.NewRasterizer(size, size)
	rad := h / 2
	// Cubic Bézier approximation of a quarter circle
	k := rad * 0.5523
	r.MoveTo(x+rad, y)
	r.LineTo(x+w-rad, y)
	r.CubeTo(x+w-rad+k, y, x+w, y+rad-k, x+w, y+rad)
	r.CubeTo(x+w, y+rad+k, x+w-rad+k, y+h, x+w-rad, y+h)
	r.LineTo(x+rad, y+h)
	r.CubeTo(x+rad-k, y+h, x, y+rad+k, x, y+rad)
	r.CubeTo(x, y+rad-k, x+rad-k, y, x+rad, y)
	r.ClosePath()
	return r
}

func digitFace(px float64) (font.Face, error) {
	f, err := opentype.Parse(gobold.TTF)
	if err != nil {
		return nil, err
	}
	return opentype.NewFace(f, &opentype.FaceOptions{Size: px, DPI: 72, Hinting: font.HintingFull})
}
