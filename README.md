## COG

COG is a golang project that aims to providing reading/writing of cloud optimized geotiffs. 
This is still an early stage project and only supports reading/writing of single band imagery.

- Reading: The input COG can be a multi-spectral image, the `Reader` decodes each band into a `image.Image`
- Writing: Supports `image.Gray` only

### Usage

```go
package main

import (
	"bytes"
	"image"
	"io/ioutil"
	"log"
	"os"

	"github.com/jh-sz/cog/cog"
)

func main() {
	byts, err := ioutil.ReadFile("./testdata/example.tif")
	if err != nil {
		log.Fatalf("unable to read test file %+v", err)
	}

	d, err := cog.NewReader(bytes.NewReader(byts))
	if err != nil {
		log.Fatalf("unable to initialise Reader %+v", err)
	}

	// reading the 49th tile
	imgs, err := d.Read(48)
	if err != nil {
		log.Fatalf("could not read tile")
	}

	// getting the second band from tile
	img := imgs[1].(*image.Gray)

	var buf bytes.Buffer
	w, err := cog.NewWriter(&buf, cog.Compression(5))
	if err != nil {
		log.Fatalf("unable to initialise writer: %+v\n", err)
	}

	if err := w.Write(img); err != nil {
		log.Fatalf("cannot write image: %+v\n", err)
	}

	if err := ioutil.WriteFile("output.tif", buf.Bytes(), os.ModePerm); err != nil {
		log.Fatalf("cannot write output.tif to disk %+v\n", err)
	}
}
```

