package cog

import (
	"bytes"
	"encoding/binary"
	"errors"
	"image"
	"io"
	"sort"

	"github.com/jh-sz/cog/lzw"
)

// only supports little-endian writes
var littleEndian = binary.LittleEndian

type Writer struct {
	w        io.Writer
	metadata *Metadata
}

func Compression(compr uint16) func(m *Metadata) {
	return func(m *Metadata) {
		m.Compression = compr
	}
}

func PlanarConfig(pc uint16) func(m *Metadata) {
	return func(m *Metadata) {
		m.PlanarConfig = pc
	}
}

func Predictor(pr uint16) func(m *Metadata) {
	return func(m *Metadata) {
		m.Predictor = pr
	}
}

func ImageWidth(w uint32) func(m *Metadata) {
	return func(m *Metadata) {
		if w == 0 {
			panic("image width cannot be 0")
		}
		m.ImageWidth = w
	}
}

func ImageLen(l uint32) func(m *Metadata) {
	return func(m *Metadata) {
		if l == 0 {
			panic("image length cannot be 0")
		}
		m.ImageLength = l
	}
}

func NewWriter(w io.Writer, opts ...func(*Metadata)) (*Writer, error) {
	metadata := &Metadata{
		Compression:  cLZW,
		PlanarConfig: pcChunky,
		Predictor:    prNone,

		// these can be obtain from image.Image
		PhotometricInterpretation: pBlackIsZero,
		BitsPerSample:             8,
	}

	for _, opt := range opts {
		opt(metadata)
	}

	if metadata.Predictor == prHorizontal && metadata.Compression != cLZW {
		return nil, errors.New("horizontal differencing only applicable to lzw, see p64 of the spec")
	}

	return &Writer{
		w:        w,
		metadata: metadata,
	}, nil
}

func (w *Writer) Write(imgs ...image.Image) error {
	if len(imgs) == 0 {
		return errors.New("no image given")
	}

	var imgLen, imgWidth uint32
	if w.metadata.ImageLength != 0 && w.metadata.ImageWidth != 0 {
		imgLen = w.metadata.ImageLength
		imgWidth = w.metadata.ImageLength
	} else {
		if len(imgs) > 1 {
			return errors.New("image width or length must be specified when writing more than 1 tile")
		}
		imgWidth = uint32(imgs[0].Bounds().Size().X)
		imgLen = uint32(imgs[0].Bounds().Size().Y)
	}

	dxs := make(map[int]struct{})
	dys := make(map[int]struct{})
	imgTyps := make(map[string]struct{})
	for _, img := range imgs {
		dxs[img.Bounds().Dx()] = struct{}{}
		dys[img.Bounds().Dy()] = struct{}{}

		switch img.(type) {
		case *image.Gray:
			imgTyps["Gray"] = struct{}{}
		default:
			return UnsupportedError("unsupported image type")
		}
	}
	if len(dxs) > 1 || len(dys) > 1 {
		// spec page 66
		return errors.New("all tiles in geotiff needs to be the same size")
	}
	if len(imgTyps) > 1 {
		return errors.New("images are not the same type")
	}

	// TODO: []bytes.Buffer per tile ?
	imgBufs := make([]bytes.Buffer, len(imgs))
	dsts := make([]io.Writer, len(imgs))

	switch w.metadata.Compression {
	case cNone:
		for i := range dsts {
			dsts[i] = &imgBufs[i]
		}
	case cLZW:
		for i := range dsts {
			dsts[i] = lzw.NewWriter(&imgBufs[i], lzw.MSB, 8)
		}
	}

	for i, img := range imgs {
		var err error
		switch m := img.(type) {
		case *image.Gray:
			w.metadata.PhotometricInterpretation = pBlackIsZero
			w.metadata.SamplesPerPixel = 1
			w.metadata.BitsPerSample = 8
			err = encodeGray(dsts[i], m.Pix, m.Bounds().Dx(), m.Bounds().Dy(), m.Stride, w.metadata.Predictor == prHorizontal)
		}
		if err != nil {
			return err
		}
	}

	// compression writers have a Close method
	if w.metadata.Compression != cNone {
		for _, dst := range dsts {
			if err := dst.(io.Closer).Close(); err != nil {
				return err
			}
		}
	}

	if _, err := io.WriteString(w.w, leHeader); err != nil {
		return err
	}

	// write ifd offset. Starts after header bytes
	if err := binary.Write(w.w, littleEndian, uint32(headerLen)); err != nil {
		return err
	}

	entries := []ifdEntry{
		{tImageWidth, dtShort, []uint32{imgWidth}},
		{tImageLength, dtShort, []uint32{imgLen}},
		{tBitsPerSample, dtShort, []uint32{uint32(w.metadata.BitsPerSample)}},
		{tCompression, dtShort, []uint32{uint32(w.metadata.Compression)}},
		{tPhotometricInterpretation, dtShort, []uint32{uint32(w.metadata.PhotometricInterpretation)}},
		{tSamplesPerPixel, dtShort, []uint32{uint32(w.metadata.SamplesPerPixel)}},
		{tPlanarConfiguration, dtShort, []uint32{uint32(pcChunky)}}, // TODO: only support chunky format
		{tPredictor, dtShort, []uint32{uint32(w.metadata.Predictor)}},
		{tTileWidth, dtLong, []uint32{uint32(imgs[0].Bounds().Size().X)}},
		{tTileLength, dtLong, []uint32{uint32(imgs[0].Bounds().Size().Y)}},

		//TODO: support more tags
	}

	imgOffset := headerLen + numOfEntriesByteLen + ifdNextValueLen
	imgOffset += calcEntriesLen(entries)

	imgCounts := len(imgs)
	// tTileOffsets & tTileByteCounts ifd entries length
	// they weren't include in the 'entries' yet
	imgOffset += ifdEntryLen * 2
	if imgCounts*dataLengthMap[dtLong] > entryValueLen {
		imgOffset += imgCounts * dataLengthMap[dtLong] * 2
	}

	tileOffsets := make([]uint32, imgCounts)
	tileByteCounts := make([]uint32, imgCounts)
	for i, imgBuf := range imgBufs {
		bufLen := imgBuf.Len()
		tileOffsets[i] = uint32(imgOffset)
		tileByteCounts[i] = uint32(bufLen)
		imgOffset += bufLen
	}
	entries = append(entries, ifdEntry{tTileOffsets, dtLong, tileOffsets})
	entries = append(entries, ifdEntry{tTileByteCounts, dtLong, tileByteCounts})

	if err := writeIFD(w.w, headerLen, entries); err != nil {
		return err
	}

	for _, tile := range imgBufs {
		if _, err := tile.WriteTo(w.w); err != nil {
			return err
		}
	}

	return nil
}

type ifdEntry struct {
	tag      int
	dataType int
	data     []uint32
}

func (e ifdEntry) putData(p []byte) {
	for _, d := range e.data {
		switch e.dataType {
		case dtByte, dtASCII:
			p[0] = byte(d)
			p = p[1:]
		case dtShort:
			littleEndian.PutUint16(p, uint16(d))
			p = p[2:]
		case dtLong, dtRational:
			littleEndian.PutUint32(p, d)
			p = p[4:]
		}
	}
}

func calcEntriesLen(entries []ifdEntry) int {
	l := len(entries) * ifdEntryLen

	for _, e := range entries {
		valLen := dataLengthMap[uint16(e.dataType)] * len(e.data)
		if valLen > entryValueLen {
			l += valLen
		}
	}

	return l
}

func writeIFD(w io.Writer, ifdOffset int, d []ifdEntry) error {
	var buf [ifdEntryLen]byte
	// Make space for "pointer area" containing IFD entry data
	// longer than 4 bytes.
	parea := make([]byte, 1024)
	pstart := ifdOffset + ifdEntryLen*len(d) + numOfEntriesByteLen + ifdNextValueLen
	var o int // Current offset in parea.

	// The IFD has to be written with the tags in ascending order.
	sort.Slice(d, func(i, j int) bool {
		return d[i].tag < d[j].tag
	})

	// Write the number of entries in this IFD.
	if err := binary.Write(w, littleEndian, uint16(len(d))); err != nil {
		return err
	}

	for _, ent := range d {
		littleEndian.PutUint16(buf[0:2], uint16(ent.tag))
		littleEndian.PutUint16(buf[2:4], uint16(ent.dataType))
		count := uint32(len(ent.data))
		if ent.dataType == dtRational {
			count /= 2
		}
		littleEndian.PutUint32(buf[4:8], count)
		datalen := int(count * lengths[ent.dataType])
		if datalen <= 4 {
			ent.putData(buf[8:12])
		} else {
			if (o + datalen) > len(parea) {
				newlen := len(parea) + 1024
				for (o + datalen) > newlen {
					newlen += 1024
				}
				newarea := make([]byte, newlen)
				copy(newarea, parea)
				parea = newarea
			}
			ent.putData(parea[o : o+datalen])
			littleEndian.PutUint32(buf[8:12], uint32(pstart+o))
			o += datalen
		}
		if _, err := w.Write(buf[:]); err != nil {
			return err
		}
	}
	// The IFD ends with the offset of the next IFD in the file,
	// or zero if it is the last one (page 14).
	if err := binary.Write(w, littleEndian, uint32(0)); err != nil {
		return err
	}
	_, err := w.Write(parea[:o])
	return err
}

func encodeGray(w io.Writer, pix []uint8, dx, dy, stride int, predictor bool) error {
	if !predictor {
		return writePix(w, pix, dy, dx, stride)
	}
	buf := make([]byte, dx)
	for y := 0; y < dy; y++ {
		min := y*stride + 0
		max := y*stride + dx
		off := 0
		var v0 uint8
		for i := min; i < max; i++ {
			v1 := pix[i]
			buf[off] = v1 - v0
			v0 = v1
			off++
		}
		if _, err := w.Write(buf); err != nil {
			return err
		}
	}
	return nil
}

// writePix writes the internal byte array of an image to w. It is less general
// but much faster then encode. writePix is used when pix directly
// corresponds to one of the TIFF image types.
func writePix(w io.Writer, pix []byte, nrows, length, stride int) error {
	if length == stride {
		_, err := w.Write(pix[:nrows*length])
		return err
	}
	for ; nrows > 0; nrows-- {
		if _, err := w.Write(pix[:length]); err != nil {
			return err
		}
		pix = pix[stride:]
	}
	return nil
}
