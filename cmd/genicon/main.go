// go run ./cmd/genicon — converts assets/icon.png → assets/FocusTrack.ico
// Writes a classic ICO file with BMP (DIB) image data, which is required
// by the Windows Shell_NotifyIcon API used by getlantern/systray.
package main

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/draw"
	"image/png"
	"log"
	"os"

	xdraw "golang.org/x/image/draw"
)

func main() {
	f, err := os.Open("assets/icon.png")
	if err != nil {
		log.Fatalf("open png: %v", err)
	}
	defer f.Close()

	src, err := png.Decode(f)
	if err != nil {
		log.Fatalf("decode png: %v", err)
	}

	sizes := []int{256, 64, 48, 32, 16}

	type entry struct {
		sz   int
		data []byte // raw DIB bytes
	}
	var entries []entry

	for _, sz := range sizes {
		// Scale to target size
		dst := image.NewRGBA(image.Rect(0, 0, sz, sz))
		xdraw.BiLinear.Scale(dst, dst.Bounds(), src, src.Bounds(), xdraw.Over, nil)

		entries = append(entries, entry{sz: sz, data: encodeDIB(dst)})
	}

	out, err := os.Create("assets/FocusTrack.ico")
	if err != nil {
		log.Fatalf("create ico: %v", err)
	}
	defer out.Close()

	n := len(entries)
	w := new(bytes.Buffer)

	// ICONDIR (6 bytes)
	le16(w, 0) // reserved
	le16(w, 1) // type = icon
	le16(w, uint16(n))

	// Data starts after ICONDIR (6) + n * ICONDIRENTRY (16)
	dataOffset := uint32(6 + n*16)

	// ICONDIRENTRY for each image (16 bytes each)
	for _, e := range entries {
		bw := byte(0) // 0 means 256
		if e.sz < 256 {
			bw = byte(e.sz)
		}
		w.WriteByte(bw)                       // width
		w.WriteByte(bw)                       // height
		w.WriteByte(0)                        // color count (0 = true color)
		w.WriteByte(0)                        // reserved
		le16(w, 1)                            // color planes
		le16(w, 32)                           // bits per pixel
		le32(w, uint32(len(e.data)))          // size of image data
		le32(w, dataOffset)                   // offset of image data
		dataOffset += uint32(len(e.data))
	}

	// Image data
	for _, e := range entries {
		w.Write(e.data)
	}

	out.Write(w.Bytes())
	log.Printf("wrote assets/FocusTrack.ico: %d entries, %d bytes total", n, w.Len())
}

// encodeDIB encodes an RGBA image as a BMP DIB (BITMAPINFOHEADER + BGRA pixels + AND mask)
// suitable for embedding in an ICO file.
func encodeDIB(img *image.RGBA) []byte {
	sz := img.Bounds().Dx() // square assumed
	// BMP stores rows bottom-to-top
	const bitsPerPixel = 32
	pixelBytes := sz * sz * 4

	// AND mask (1 bit per pixel, rows bottom-to-top, padded to 4-byte rows)
	maskRowBytes := (sz + 31) / 32 * 4
	maskBytes := maskRowBytes * sz

	dibSize := 40 + pixelBytes + maskBytes

	buf := new(bytes.Buffer)

	// BITMAPINFOHEADER (40 bytes)
	le32(buf, 40)                          // biSize
	le32(buf, uint32(sz))                  // biWidth
	le32(buf, uint32(sz*2))                // biHeight (doubled for ICO = XOR + AND mask)
	le16(buf, 1)                           // biPlanes
	le16(buf, bitsPerPixel)               // biBitCount
	le32(buf, 0)                           // biCompression = BI_RGB
	le32(buf, uint32(pixelBytes))          // biSizeImage
	le32(buf, 0)                           // biXPelsPerMeter
	le32(buf, 0)                           // biYPelsPerMeter
	le32(buf, 0)                           // biClrUsed
	le32(buf, 0)                           // biClrImportant

	// XOR (color) data — BGRA, rows bottom-to-top
	for row := sz - 1; row >= 0; row-- {
		for col := 0; col < sz; col++ {
			px := img.RGBAAt(col, row)
			buf.WriteByte(px.B)
			buf.WriteByte(px.G)
			buf.WriteByte(px.R)
			buf.WriteByte(px.A)
		}
	}

	// AND mask — 0 = opaque, 1 = transparent; rows bottom-to-top
	// We use alpha: if alpha < 128 → transparent (bit=1), else opaque (bit=0)
	for row := sz - 1; row >= 0; row-- {
		var bit uint8
		col := 0
		written := 0
		for col < sz {
			bit = 0
			// Pack 8 pixels into one byte
			for b := 7; b >= 0 && col < sz; b-- {
				px := img.RGBAAt(col, row)
				if px.A < 128 {
					bit |= 1 << uint(b)
				}
				col++
			}
			buf.WriteByte(bit)
			written++
		}
		// Pad row to 4-byte boundary
		for written%4 != 0 {
			buf.WriteByte(0)
			written++
		}
	}

	_ = dibSize
	return buf.Bytes()
}

// Helpers
func le16(w *bytes.Buffer, v uint16) { binary.Write(w, binary.LittleEndian, v) }
func le32(w *bytes.Buffer, v uint32) { binary.Write(w, binary.LittleEndian, v) }

// resize helper using stdlib only (no xdraw needed for main resize, but we use it above)
func resizeNRGBA(src image.Image, sz int) *image.RGBA {
	dst := image.NewRGBA(image.Rect(0, 0, sz, sz))
	draw.Draw(dst, dst.Bounds(), src, image.Point{}, draw.Src)
	return dst
}
