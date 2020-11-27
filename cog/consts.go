// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cog

// A tiff image file contains one or more images. The metadata
// of each image is contained in an Image File Directory (ifd),
// which contains entries of 12 bytes each and is described
// on page 14-16 of the specification. An ifd entry consists of
//
//  - a tag, which describes the signification of the entry,
//  - the data type and length of the entry,
//  - the data itself or a pointer to it if it is more than 4 bytes.
//
// The presence of a length means that each ifd is effectively an array.

const (
	leHeader = "II\x2A\x00" // Header for little-endian files.
	beHeader = "MM\x00\x2A" // Header for big-endian files.
)

const (
	headerLen           = 8  // tiff header length
	ifdEntryLen         = 12 // Length of an ifd entry in bytes.
	entryValueLen       = 4  // 4 bytes in an entry value.
	numOfEntriesByteLen = 2  // 2 bytes holding the value of number of entries
	ifdNextValueLen     = 4  // value offset of the next ifd
)

// Data types (p. 14-16 of the spec).
const (
	dtByte     = 1
	dtASCII    = 2
	dtShort    = 3
	dtLong     = 4
	dtRational = 5
	dtDouble   = 12
)

// The length of one instance of each data type in bytes.
var lengths = [...]uint32{0, 1, 1, 2, 4, 8, 8}

var dataLengthMap = map[uint16]int{
	dtByte:     1,
	dtASCII:    1,
	dtShort:    2,
	dtLong:     4,
	dtRational: 8,
	dtDouble:   8,
}

// Tags (see p. 28-41 of the spec).
const (
	tImageWidth                = 256
	tImageLength               = 257
	tBitsPerSample             = 258
	tCompression               = 259
	tPhotometricInterpretation = 262

	tStripOffsets    = 273
	tSamplesPerPixel = 277
	tRowsPerStrip    = 278
	tStripByteCounts = 279

	tPlanarConfiguration = 284

	tXResolution    = 282
	tYResolution    = 283
	tResolutionUnit = 296

	tPredictor = 317
	tColorMap  = 320

	tTileWidth      = 322
	tTileLength     = 323
	tTileOffsets    = 324
	tTileByteCounts = 325

	tExtraSamples = 338
	tSampleFormat = 339
)

// Compression types (defined in various places in the spec and supplements).
const (
	cNone       = 1
	cCCITT      = 2
	cG3         = 3 // Group 3 Fax.
	cG4         = 4 // Group 4 Fax.
	cLZW        = 5
	cJPEGOld    = 6 // Superseded by cJPEG.
	cJPEG       = 7
	cDeflate    = 8 // zlib compression.
	cPackBits   = 32773
	cDeflateOld = 32946 // Superseded by cDeflate.
)

// Photometric interpretation values (see p. 37 of the spec).
const (
	pWhiteIsZero = 0
	pBlackIsZero = 1
	pRGB         = 2
	pPaletted    = 3
	pTransMask   = 4 // transparency mask
	pCMYK        = 5
	pYCbCr       = 6
	pCIELab      = 8
)

// Values for the tPredictor tag (page 64-65 of the spec).
const (
	prNone       = 1
	prHorizontal = 2
)

// Values for the tResolutionUnit tag (page 18).
const (
	resNone    = 1
	resPerInch = 2 // Dots per inch.
	resPerCM   = 3 // Dots per centimeter.
)

// Values for the tPlanarConfiguration tag (page 38)
const (
	pcChunky = 1
	pcPlanar = 2
)

// ExtraSample values (page 31)
const (
	esUnspecified       = 0
	esAsociatedAlpha    = 1
	esUnassociatedAlpha = 2
)

// imageMode represents the mode of the image.
type imageMode int

const (
	mBilevel imageMode = iota
	mPaletted
	mGray
	mGrayInvert
	mRGB
	mRGBA
	mNRGBA
)

// CompressionType describes the type of compression used in Options.
type CompressionType int

const (
	Uncompressed CompressionType = iota
	Deflate
)

// specValue returns the compression type constant from the tiff spec that
// is equivalent to c.
func (c CompressionType) specValue() uint32 {
	switch c {
	case Deflate:
		return cDeflate
	}
	return cNone
}
