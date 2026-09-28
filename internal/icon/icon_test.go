package icon

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"
)

const size = 64

func render(t *testing.T, active int, busy bool) image.Image {
	t.Helper()
	data, err := Render(size, active, busy)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("the result is not a PNG: %v", err)
	}
	return img
}

// at returns the color of a pixel; fully transparent pixels all read as
// transparent, whatever the color the drawing left in them.
func at(img image.Image, x, y int) color.NRGBA {
	c := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
	if c.A == 0 {
		return transparent
	}
	return c
}

// Points of the 64 px icon, from the proportions of arrow and badge
var (
	shaft       = image.Pt(19, 32) // middle of the arrow shaft
	badgeRight  = image.Pt(56, 56) // inside any badge, off the digits
	badgeLeft   = image.Pt(22, 48) // inside a two-digit badge only
	transparent = color.NRGBA{}
)

func TestRenderSize(t *testing.T) {
	for _, s := range []int{16, 22, 64} {
		data, err := Render(s, 3, true)
		if err != nil {
			t.Fatalf("Render(%d): %v", s, err)
		}
		cfg, err := png.DecodeConfig(bytes.NewReader(data))
		if err != nil {
			t.Fatalf("DecodeConfig: %v", err)
		}
		check(t, "width", cfg.Width, s)
		check(t, "height", cfg.Height, s)
	}
}

func TestRenderArrow(t *testing.T) {
	tests := []struct {
		name string
		busy bool
		want color.NRGBA
	}{
		{"idle", false, idleColor},
		{"busy", true, busyColor},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			img := render(t, 0, tt.busy)
			check(t, "shaft", at(img, shaft.X, shaft.Y), tt.want)
			// Only the arrow is drawn: the background stays see-through
			check(t, "top left corner", at(img, 0, 0), transparent)
			check(t, "top right corner", at(img, size-1, 0), transparent)
		})
	}
}

func TestRenderBadge(t *testing.T) {
	tests := []struct {
		name           string
		active         int
		wantRight      color.NRGBA
		wantLeftOfPill color.NRGBA
	}{
		{"no active forward, no badge", 0, transparent, transparent},
		{"one digit, a circle", 1, badgeColor, transparent},
		{"two digits, a wider pill", 12, badgeColor, badgeColor},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			img := render(t, tt.active, true)
			check(t, "badge", at(img, badgeRight.X, badgeRight.Y), tt.wantRight)
			check(t, "left of the pill", at(img, badgeLeft.X, badgeLeft.Y), tt.wantLeftOfPill)
		})
	}
}

func TestRenderDigits(t *testing.T) {
	// The digits are the only difference between these icons
	one, _ := Render(size, 1, true)
	two, _ := Render(size, 2, true)
	if bytes.Equal(one, two) {
		t.Error("1 and 2 render the same icon")
	}

	// No room beyond two digits: the badge caps at 99
	capped, _ := Render(size, 150, true)
	ninetyNine, _ := Render(size, 99, true)
	if !bytes.Equal(capped, ninetyNine) {
		t.Error("150 should render as 99")
	}
}

func check[T comparable](t *testing.T, name string, got, want T) {
	t.Helper()
	if got != want {
		t.Errorf("%s: got %v, want %v", name, got, want)
	}
}
